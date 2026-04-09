package engine

import (
	"fmt"
	"canopy/protocol"
	"sync"
)

// ProcessTransport is the interface that process transports must satisfy.
// Matches transport.Transport but lives here to avoid circular imports.
type ProcessTransport interface {
	Messages() <-chan *protocol.Message
	Errors() <-chan error
	Start()
	Stop()
	SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{})
}

// TransportFactory creates a transport from a process transport config.
type TransportFactory func(cfg protocol.ProcessTransportConfig) (ProcessTransport, error)

// Process is a named, managed goroutine with its own transport.
type Process struct {
	ID        string
	transport ProcessTransport
	cancel    chan struct{}
	status    string // "running", "stopped", "error"
}

// ProcessManager manages the lifecycle of processes.
type ProcessManager struct {
	mu        sync.Mutex
	processes map[string]*Process
	sess      *Session
	factory   TransportFactory
}

func NewProcessManager(sess *Session, factory TransportFactory) *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*Process),
		sess:      sess,
		factory:   factory,
	}
}

// Create starts a new process with the given configuration.
func (pm *ProcessManager) Create(cp protocol.CreateProcess) error {
	pm.mu.Lock()
	if _, exists := pm.processes[cp.ProcessID]; exists {
		pm.mu.Unlock()
		return fmt.Errorf("process %q already exists", cp.ProcessID)
	}
	pm.mu.Unlock()

	tr, err := pm.factory(cp.Transport)
	if err != nil {
		return fmt.Errorf("create transport for process %q: %w", cp.ProcessID, err)
	}

	proc := &Process{
		ID:        cp.ProcessID,
		transport: tr,
		cancel:    make(chan struct{}),
		status:    "running",
	}

	pm.mu.Lock()
	pm.processes[cp.ProcessID] = proc
	pm.mu.Unlock()

	pm.sess.forEachSurfaceLocked(func(sid string, surf *Surface) {
		surf.HandleUpdateDataModel(protocol.UpdateDataModel{
			SurfaceID: sid,
			Ops:       []protocol.DataModelOp{{Op: "add", Path: fmt.Sprintf("/processes/%s/status", cp.ProcessID), Value: "running"}},
		})
	})

	go proc.run(pm.sess, pm)
	return nil
}

// Stop terminates a running process.
func (pm *ProcessManager) Stop(processID string) error {
	pm.mu.Lock()
	proc, ok := pm.processes[processID]
	if !ok {
		pm.mu.Unlock()
		return fmt.Errorf("process %q not found", processID)
	}
	pm.mu.Unlock()

	select {
	case <-proc.cancel:
		// already stopped
	default:
		close(proc.cancel)
	}
	proc.transport.Stop()

	pm.mu.Lock()
	proc.status = "stopped"
	pm.mu.Unlock()

	pm.sess.forEachSurfaceLocked(func(sid string, surf *Surface) {
		surf.HandleUpdateDataModel(protocol.UpdateDataModel{
			SurfaceID: sid,
			Ops:       []protocol.DataModelOp{{Op: "add", Path: fmt.Sprintf("/processes/%s/status", processID), Value: "stopped"}},
		})
	})

	// Clean up channel subscriptions for this process
	if cm := pm.sess.ChannelManager(); cm != nil {
		cm.CleanupProcess(processID)
	}
	return nil
}

// SendTo routes a message to a process's transport via SendAction.
func (pm *ProcessManager) SendTo(processID string, msg *protocol.Message) error {
	pm.mu.Lock()
	proc, ok := pm.processes[processID]
	pm.mu.Unlock()

	if !ok {
		return fmt.Errorf("process %q not found", processID)
	}

	// For sendToProcess, route the inner message through the session directly
	pm.sess.HandleMessage(msg)
	_ = proc // process exists, message routed
	return nil
}

// IDs returns the IDs of all processes.
func (pm *ProcessManager) IDs() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	ids := make([]string, 0, len(pm.processes))
	for id := range pm.processes {
		ids = append(ids, id)
	}
	return ids
}

// GetStatus returns the status of a process.
func (pm *ProcessManager) GetStatus(processID string) string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if proc, ok := pm.processes[processID]; ok {
		return proc.status
	}
	return ""
}

// StopAll terminates all running processes.
func (pm *ProcessManager) StopAll() {
	pm.mu.Lock()
	ids := make([]string, 0, len(pm.processes))
	for id := range pm.processes {
		ids = append(ids, id)
	}
	pm.mu.Unlock()

	for _, id := range ids {
		pm.Stop(id)
	}
}

// setStatus writes process status to the data model of all surfaces.
// Called from goroutines (not from within HandleMessage) — uses ForEachSurface which acquires the lock.
func (pm *ProcessManager) setStatus(processID, status string) {
	path := fmt.Sprintf("/processes/%s/status", processID)
	pm.sess.ForEachSurface(func(sid string, surf *Surface) {
		surf.HandleUpdateDataModel(protocol.UpdateDataModel{
			SurfaceID: sid,
			Ops:       []protocol.DataModelOp{{Op: "add", Path: path, Value: status}},
		})
	})
}

// run is the process goroutine. Reads from transport and routes to session.
func (p *Process) run(sess *Session, pm *ProcessManager) {
	defer logRecover("process", "", p.ID)

	p.transport.Start()
	errCh := p.transport.Errors()

	for {
		select {
		case <-p.cancel:
			return
		case msg, ok := <-p.transport.Messages():
			if !ok {
				sess.FlushPendingComponents()
				pm.mu.Lock()
				p.status = "stopped"
				pm.mu.Unlock()
				pm.setStatus(p.ID, "stopped") // called from goroutine, uses ForEachSurface (locking)
				if cm := sess.ChannelManager(); cm != nil {
					cm.CleanupProcess(p.ID)
				}
				return
			}
			if msg == nil {
				// nil sentinel = end of a generation turn; flush pending components
				sess.FlushPendingComponents()
				continue
			}
			sess.HandleMessage(msg)
		case err, ok := <-errCh:
			if !ok {
				// Errors channel closed — nil it out to stop selecting on it.
				// Keep draining messages so FlushPendingComponents runs.
				errCh = nil
				continue
			}
			logError("process", "", fmt.Sprintf("process %s error: %v", p.ID, err))
		}
	}
}
