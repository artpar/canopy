package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"canopy/jlog"
	"canopy/protocol"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// ClaudeCodeConfig configures the Claude Code transport.
type ClaudeCodeConfig struct {
	Prompt       string // UI building prompt
	Model        string // optional model override
	LibraryBlock string // optional library component listing for system prompt
}

// MCPServerHandle is returned by StartHTTPMCP — an abstraction so transport/
// doesn't need to import mcp/ or engine/.
type MCPServerHandle struct {
	Port        int
	Cleanup     func()
	PushAction  func(surfaceID, event string, data map[string]interface{})
}

// StartHTTPMCPFunc creates and starts an HTTP MCP server.
// main.go provides this closure with access to session/renderer/dispatcher.
type StartHTTPMCPFunc func() (*MCPServerHandle, error)

// ClaudeCodeTransport spawns `claude -p` as a subprocess, with Claude Code
// connecting back to jview via an HTTP MCP server to build UI iteratively.
type ClaudeCodeTransport struct {
	config   ClaudeCodeConfig
	messages chan *protocol.Message
	errors   chan error
	done     chan struct{}
	stopOnce sync.Once

	// StartMCP creates and starts an HTTP MCP server.
	// Must be set before Start(). main.go provides this closure with
	// access to session/renderer/dispatcher.
	StartMCP StartHTTPMCPFunc

	// OnDone is called after the Claude process exits. Use this to
	// finalize cache files.
	OnDone func()

	// OnStatus is called with progress updates at key lifecycle points.
	OnStatus func(status string)

	// Runtime state
	mcpHandle  *MCPServerHandle
	cmd        *exec.Cmd
	tmpFiles   []string
	sessionID  string      // UUID for claude --session-id / --resume
	followUpCh chan string // buffered(1), follow-up prompts
	SpawnCount int         // 0 = initial, 1+ = follow-up (exported for OnDone checks)
}

// NewClaudeCodeTransport creates a new Claude Code transport.
func NewClaudeCodeTransport(cfg ClaudeCodeConfig) *ClaudeCodeTransport {
	return &ClaudeCodeTransport{
		config:     cfg,
		messages:   make(chan *protocol.Message, 64),
		errors:     make(chan error, 8),
		done:       make(chan struct{}),
		sessionID:  uuid.New().String(),
		followUpCh: make(chan string, 1),
	}
}

// SessionID returns the UUID used for claude --session-id / --resume.
func (t *ClaudeCodeTransport) SessionID() string {
	return t.sessionID
}

// SendFollowUp queues a follow-up prompt to be sent to Claude via --resume.
func (t *ClaudeCodeTransport) SendFollowUp(prompt string) {
	select {
	case t.followUpCh <- prompt:
	default:
		jlog.Infof("transport", "", "claude-code: follow-up channel full, dropping prompt")
	}
}

func (t *ClaudeCodeTransport) Messages() <-chan *protocol.Message { return t.messages }
func (t *ClaudeCodeTransport) Errors() <-chan error                { return t.errors }

func (t *ClaudeCodeTransport) Start() {
	go t.run()
}

func (t *ClaudeCodeTransport) Stop() {
	t.stopOnce.Do(func() {
		close(t.done)
		if t.cmd != nil && t.cmd.Process != nil {
			t.cmd.Process.Kill()
		}
		if t.mcpHandle != nil && t.mcpHandle.Cleanup != nil {
			t.mcpHandle.Cleanup()
		}
		for _, f := range t.tmpFiles {
			os.Remove(f)
		}
	})
}

func (t *ClaudeCodeTransport) SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
	if t.mcpHandle == nil || t.mcpHandle.PushAction == nil {
		return
	}
	t.mcpHandle.PushAction(surfaceID, event.Name, data)
}

func (t *ClaudeCodeTransport) run() {
	defer close(t.messages)
	defer close(t.errors)

	if t.StartMCP == nil {
		t.errors <- fmt.Errorf("claude-code transport: StartMCP not set (must be set before Start)")
		return
	}

	// Start HTTP MCP server
	t.emitStatus("Starting MCP server...")
	handle, err := t.StartMCP()
	if err != nil {
		t.errors <- fmt.Errorf("claude-code transport: start HTTP MCP: %w", err)
		return
	}
	t.mcpHandle = handle

	// Write MCP config to temp file
	mcpConfigPath, err := t.writeMCPConfig()
	if err != nil {
		t.errors <- fmt.Errorf("claude-code transport: write MCP config: %w", err)
		return
	}

	// Write component reference to temp file so Claude Code can Read it
	refPath, err := t.writeRefFile()
	if err != nil {
		t.errors <- fmt.Errorf("claude-code transport: write ref file: %w", err)
		return
	}

	// Spawn initial claude process
	t.emitStatus("Launching Claude Code...")
	jlog.Infof("transport", "", "claude-code: spawning claude -p on port %d (session=%s)", handle.Port, t.sessionID)
	jlog.Infof("transport", "", "claude-code: mcp-config=%s ref=%s", mcpConfigPath, refPath)
	t.spawnClaude(mcpConfigPath, refPath, t.config.Prompt)

	// Follow-up loop: wait for follow-up prompts or shutdown
	for {
		select {
		case prompt := <-t.followUpCh:
			t.emitStatus("Resuming with follow-up...")
			jlog.Infof("transport", "", "claude-code: follow-up prompt: %s", truncate(prompt, 100))
			t.spawnClaude(mcpConfigPath, refPath, prompt)
		case <-t.done:
			return
		}
	}
}

// filterEnv returns a copy of env with any variable whose key matches removed.
func filterEnv(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}

func (t *ClaudeCodeTransport) emitStatus(status string) {
	if t.OnStatus != nil {
		t.OnStatus(status)
	}
}

func (t *ClaudeCodeTransport) writeMCPConfig() (string, error) {
	config := map[string]any{
		"mcpServers": map[string]any{
			"jview": map[string]any{
				"type": "http",
				"url":  fmt.Sprintf("http://localhost:%d/mcp", t.mcpHandle.Port),
			},
		},
	}
	data, _ := json.MarshalIndent(config, "", "  ")

	f, err := os.CreateTemp("", "jview-cc-*.json")
	if err != nil {
		return "", err
	}
	t.tmpFiles = append(t.tmpFiles, f.Name())
	f.Write(data)
	f.Close()
	return f.Name(), nil
}

func (t *ClaudeCodeTransport) writeRefFile() (string, error) {
	f, err := os.CreateTemp("", "jview-a2ui-ref-*.txt")
	if err != nil {
		return "", err
	}
	t.tmpFiles = append(t.tmpFiles, f.Name())
	f.WriteString(ComponentReference(t.config.LibraryBlock))
	f.Close()
	return f.Name(), nil
}

func (t *ClaudeCodeTransport) spawnClaude(mcpConfigPath, refPath, prompt string) {
	appendPrompt := fmt.Sprintf(
		"You build native macOS UIs using jview MCP tools. "+
			"Read %s for the A2UI protocol reference. "+
			"Use send_message to create surfaces and components. "+
			"Use take_screenshot to verify your layout. "+
			"Build SELF-CONTAINED apps: wire all interactivity via client-side actions "+
			"(onClick with updateDataModel + functionCalls). "+
			"For apps that need external commands (whois, curl, etc.), use the shell() function "+
			"in functionCall values — e.g. shell(concat(\"whois \", {path:\"/domain\"})). "+
			"Do NOT use get_pending_actions or server-side polling — the app must work after you exit.",
		refPath,
	)

	var args []string
	if t.SpawnCount == 0 {
		// Initial generation: use --session-id so we can --resume later
		args = []string{
			"-p", prompt,
			"--session-id", t.sessionID,
			"--append-system-prompt", appendPrompt,
			"--mcp-config", mcpConfigPath,
			"--output-format", "text",
			"--dangerously-skip-permissions",
		}
	} else {
		// Follow-up: resume the same session
		args = []string{
			"--resume", t.sessionID,
			"-p", prompt,
			"--mcp-config", mcpConfigPath,
			"--output-format", "text",
			"--dangerously-skip-permissions",
		}
	}
	if t.config.Model != "" {
		args = append(args, "--model", t.config.Model)
	}
	t.SpawnCount++

	t.cmd = exec.Command("claude", args...)
	// Clear CLAUDECODE env var to allow nested invocation
	t.cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")
	t.cmd.Stderr = os.Stderr
	jlog.Infof("transport", "", "claude-code: args=%v", args)

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		t.errors <- fmt.Errorf("claude-code transport: stdout pipe: %w", err)
		return
	}

	if err := t.cmd.Start(); err != nil {
		t.errors <- fmt.Errorf("claude-code transport: start claude: %w", err)
		return
	}

	jlog.Infof("transport", "", "claude-code: process started (pid %d, spawn #%d)", t.cmd.Process.Pid, t.SpawnCount)
	t.emitStatus("Claude is thinking...")

	// Read stdout for logging
	go func() {
		defer func() {
			if r := recover(); r != nil {
				jlog.Errorf("transport", "", "panic in claude-code stdout reader: %v", r)
			}
		}()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			jlog.Infof("transport", "", "claude-code: %s", line)
		}
	}()

	// Wait for process to exit
	err = t.cmd.Wait()
	if err != nil {
		jlog.Infof("transport", "", "claude-code: process exited with: %v", err)
	} else {
		jlog.Infof("transport", "", "claude-code: process exited normally")
	}

	if t.OnDone != nil {
		t.OnDone()
	}
}

