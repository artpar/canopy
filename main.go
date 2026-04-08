package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"canopy/cmd"
	"canopy/engine"
	"canopy/jlog"
	"canopy/mcp"
	"canopy/pkg/registry"
	"canopy/platform/darwin"
	"canopy/protocol"
	"canopy/renderer"
	"canopy/transport"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/deepseek"
	"github.com/mozilla-ai/any-llm-go/providers/gemini"
	"github.com/mozilla-ai/any-llm-go/providers/groq"
	"github.com/mozilla-ai/any-llm-go/providers/mistral"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
	"github.com/mozilla-ai/any-llm-go/providers/openai"
)

func loadEnvFile() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

// hash returns a simple FNV-1a hash of a string, for use as subscription IDs.
func hash(s string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func main() {
	// macOS requires the main thread for AppKit
	runtime.LockOSThread()

	// Load .env file (does not override existing env vars)
	loadEnvFile()

	// Initialize central logger
	jlog.Init(jlog.Config{
		MaxEntries: 10000,
		MinLevel:   jlog.LevelInfo,
		LogDir:     "~/.canopy/logs",
	})
	defer jlog.Close()

	// Handle "jview test <file>" — uses real AppKit for e2e testing
	if len(os.Args) >= 3 && os.Args[1] == "test" {
		runTests(os.Args[2])
		return
	}

	// Handle "jview mcp [file.jsonl]" — embedded MCP server
	if len(os.Args) >= 2 && os.Args[1] == "mcp" {
		runMCP(os.Args[2:])
		return
	}

	// Handle "canopy pkg <subcommand>" — package management (pure Go, no AppKit)
	if len(os.Args) >= 2 && os.Args[1] == "pkg" {
		cmd.RunPkg(os.Args[2:])
		return
	}

	// Handle "canopy bundle <app-path>" — create macOS .app bundle (pure Go, no AppKit)
	if len(os.Args) >= 2 && os.Args[1] == "bundle" {
		cmd.RunBundle(os.Args[2:])
		return
	}

	ffiConfigPath := flag.String("ffi-config", "", "Path to FFI convention file (JSON) for native function calls")
	llmProvider := flag.String("llm", "anthropic", "LLM provider: anthropic, openai, gemini, ollama, deepseek, groq, mistral")
	model := flag.String("model", "claude-opus-4-6", "Model name (default: claude-opus-4-6)")
	prompt := flag.String("prompt", "", "Prompt describing the UI to build")
	mode := flag.String("mode", "tools", "LLM mode: tools (default) or raw")
	apiKey := flag.String("api-key", "", "API key (overrides environment variable)")
	promptFile := flag.String("prompt-file", "", "Read prompt from file (overrides --prompt)")
	regenerate := flag.Bool("regenerate", false, "Force fresh LLM call, ignore cache")
	generateOnly := flag.Bool("generate-only", false, "Generate JSONL and exit without opening a window")
	claudeCode := flag.String("claude-code", "", "Prompt for Claude Code to build UI (spawns claude subprocess)")
	saveComponent := flag.String("save-component", "", "Save generated UI as reusable library component")
	watch := flag.Bool("watch", false, "Watch JSONL files for changes and reload automatically")
	mcpHTTPAddr := flag.String("mcp-http", "", "Also listen for MCP on HTTP (e.g. localhost:8080)")
	flag.Parse()

	if *promptFile != "" {
		data, err := os.ReadFile(*promptFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading prompt file: %v\n", err)
			os.Exit(1)
		}
		*prompt = string(data)
	}

	if *generateOnly && *promptFile == "" && *claudeCode == "" && *prompt == "" {
		fmt.Fprintf(os.Stderr, "error: --generate-only requires --prompt-file, --prompt, or --claude-code\n")
		os.Exit(1)
	}

	// Load component library (persists across sessions)
	lib := engine.NewLibrary()
	lib.Load()
	componentRef := transport.ComponentReference(lib.ComponentListForPrompt())

	var tr transport.Transport
	var llmTr *transport.LLMTransport        // non-nil when using LLM transport
	var ccTr *transport.ClaudeCodeTransport   // non-nil when using Claude Code transport
	var generateDone chan struct{}            // closed when generate-only can exit
	var cacheFinalized chan struct{}          // closed when cache finalization completes

	// Cache state — set when a generative transport needs caching
	var cachePrompt string
	var cacheJsonl, cacheHash, cacheTmp string
	var recorder *engine.Recorder

	args := flag.Args()

	// Auto-detect bundled app: if running inside a .app bundle with
	// Resources/app/, load it as the default input and run as a normal dock app.
	bundledApp := false
	if len(args) == 0 && *prompt == "" && *claudeCode == "" {
		if exePath, err := os.Executable(); err == nil {
			exePath, _ = filepath.EvalSymlinks(exePath)
			macosDir := filepath.Dir(exePath)
			contentsDir := filepath.Dir(macosDir)
			if filepath.Base(macosDir) == "MacOS" &&
				filepath.Base(contentsDir) == "Contents" &&
				strings.HasSuffix(filepath.Dir(contentsDir), ".app") {
				resourceApp := filepath.Join(contentsDir, "Resources", "app")
				if info, err := os.Stat(resourceApp); err == nil && info.IsDir() {
					args = []string{resourceApp}
					bundledApp = true
				}
			}
		}
	}

	if *claudeCode != "" {
		// Determine cache paths for Claude Code prompt
		cachePrompt = *claudeCode
		cacheJsonl, cacheHash, cacheTmp = engine.CachePathsForPrompt(cachePrompt, componentRef)

		if !*regenerate && engine.CacheValid(cacheJsonl, cacheHash, cachePrompt, componentRef) {
			if *saveComponent != "" {
				dc, err := engine.ExtractComponent(cacheJsonl, *saveComponent)
				if err != nil {
					jlog.Errorf("main", "", "save-component: extract failed: %v", err)
				} else if err := lib.Save(dc); err != nil {
					jlog.Errorf("main", "", "save-component: save failed: %v", err)
				} else {
					fmt.Fprintf(os.Stderr, "saved component %q to library\n", *saveComponent)
				}
			}
			if *generateOnly {
				jlog.Infof("main", "", "cache hit: %s (up to date)", cacheJsonl)
				os.Exit(0)
			}
			jlog.Infof("main", "", "cache hit: using %s", cacheJsonl)
			tr = transport.NewFileTransport(cacheJsonl)
		} else {
			// Claude Code mode — StartMCP set after session/renderer creation
			ccTr = transport.NewClaudeCodeTransport(
				transport.ClaudeCodeConfig{Prompt: *claudeCode, Model: *model, LibraryBlock: lib.ComponentListForPrompt()},
			)
			tr = ccTr
		}
	} else if len(args) > 0 && *prompt == "" {
		// File or directory mode: positional arg with no --prompt
		if *watch {
			tr = transport.NewWatchTransport(args[0])
		} else {
			tr = createFileTransport(args[0])
		}
	} else if *prompt != "" {
		// Determine cache paths
		if *promptFile != "" {
			cachePrompt = *prompt
			cacheJsonl, cacheHash, cacheTmp = engine.CachePathsForFile(*promptFile)
		} else {
			cachePrompt = *prompt
			cacheJsonl, cacheHash, cacheTmp = engine.CachePathsForPrompt(cachePrompt, componentRef)
		}

		// Unified cache check
		if !*regenerate && cachePrompt != "" && engine.CacheValid(cacheJsonl, cacheHash, cachePrompt, componentRef) {
			if *saveComponent != "" {
				dc, err := engine.ExtractComponent(cacheJsonl, *saveComponent)
				if err != nil {
					jlog.Errorf("main", "", "save-component: extract failed: %v", err)
				} else if err := lib.Save(dc); err != nil {
					jlog.Errorf("main", "", "save-component: save failed: %v", err)
				} else {
					fmt.Fprintf(os.Stderr, "saved component %q to library\n", *saveComponent)
				}
			}
			if *generateOnly {
				jlog.Infof("main", "", "cache hit: %s (up to date)", cacheJsonl)
				os.Exit(0)
			}
			jlog.Infof("main", "", "cache hit: using %s", cacheJsonl)
			tr = transport.NewFileTransport(cacheJsonl)
		} else {
			// LLM mode (cache miss)
			provider, err := createProvider(*llmProvider, *apiKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			cfg := transport.LLMConfig{
				Provider:     provider,
				Model:        *model,
				Prompt:       *prompt,
				Mode:         *mode,
				LibraryBlock: lib.ComponentListForPrompt(),
			}

			lt := transport.NewLLMTransport(cfg)
			tr = lt
			llmTr = lt
		}
	} else {
		// No arguments — start as tray-only app (no transport)
		tr = nil
	}

	// Set up recorder for generative transports (LLM or Claude Code) when caching
	if cachePrompt != "" && cacheTmp != "" && (llmTr != nil || ccTr != nil) {
		if err := os.MkdirAll(filepath.Dir(cacheTmp), 0755); err != nil {
			jlog.Errorf("main", "", "cache: mkdir failed: %v", err)
		} else {
			f, err := os.Create(cacheTmp)
			if err != nil {
				jlog.Errorf("main", "", "cache: create tmp failed: %v", err)
			} else {
				recorder = engine.NewRecorder(f)
			}
		}
	}

	// Cache finalization function (shared by LLM and Claude Code)
	// Called exactly once when generation completes. Closes cacheFinalized channel.
	cacheFinalized = make(chan struct{})
	finalizeCache := func() {
		if recorder == nil {
			return
		}
		recorder.Close()
		if err := os.Rename(cacheTmp, cacheJsonl); err != nil {
			jlog.Errorf("main", "", "cache: rename failed: %v", err)
		} else {
			jlog.Infof("main", "", "cache: wrote %s", cacheJsonl)
		}
		if err := engine.WriteCacheHash(cacheHash, cachePrompt, componentRef); err != nil {
			jlog.Errorf("main", "", "cache: write hash failed: %v", err)
		}
		recorder = nil
		// Save as library component if requested
		if *saveComponent != "" {
			dc, err := engine.ExtractComponent(cacheJsonl, *saveComponent)
			if err != nil {
				jlog.Errorf("main", "", "save-component: extract failed: %v", err)
			} else if err := lib.Save(dc); err != nil {
				jlog.Errorf("main", "", "save-component: save failed: %v", err)
			} else {
				fmt.Fprintf(os.Stderr, "saved component %q to library\n", *saveComponent)
			}
		}
		select {
		case <-cacheFinalized:
		default:
			close(cacheFinalized)
		}
	}

	// Wire finalization callbacks
	if llmTr != nil {
		if *generateOnly {
			generateDone = make(chan struct{})
		}
		lt := llmTr
		lt.OnInitialTurnDone = func() {
			finalizeCache()
			if generateDone != nil {
				close(generateDone)
			}
			darwin.SetSuppressCallbacks(false)
		}
	}
	if ccTr != nil {
		if *generateOnly {
			generateDone = make(chan struct{})
		}
		ccTr.OnDone = func() {
			jlog.Infof("main", "", "claude-code: generation complete (spawn #%d)", ccTr.SpawnCount)
			if ccTr.SpawnCount == 1 {
				// Only finalize cache on initial generation
				finalizeCache()
				if generateDone != nil {
					close(generateDone)
				}
			}
		}
	}

	// Generate-only mode: route messages through a headless session for recording
	if *generateOnly {
		mockRend := &renderer.MockRenderer{}
		mockDisp := &renderer.MockDispatcher{}
		goSess := engine.NewSession(mockRend, mockDisp)
		goSess.SetLibrary(lib)
		if recorder != nil {
			goSess.SetRecorder(recorder)
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					jlog.Errorf("main", "", "panic in generate-only transport: %v", r)
				}
			}()
			tr.Start()
			errCh := tr.Errors()
			for {
				select {
				case msg, ok := <-tr.Messages():
					if !ok {
						goSess.FlushPendingComponents()
						return
					}
					if msg == nil {
						if llmTr != nil && llmTr.OnInitialTurnDone != nil {
							llmTr.OnInitialTurnDone()
						}
						continue
					}
					goSess.HandleMessage(msg)
				case _, ok := <-errCh:
					if !ok {
						errCh = nil
						continue
					}
				}
			}
		}()
		if generateDone != nil {
			<-generateDone
		}
		tr.Stop()
		return
	}

	// Load FFI config if specified
	var ffiRegistry *engine.FFIRegistry
	if *ffiConfigPath != "" {
		cfg, err := engine.LoadFFIConfig(*ffiConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		ffiRegistry = engine.NewFFIRegistry()
		if err := ffiRegistry.LoadFromConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		jlog.Infof("main", "", "ffi: loaded %d libraries from %s", len(cfg.Libraries), *ffiConfigPath)
	}

	// Interactive mode: initialize platform and engine
	darwin.AppInit()

	// Bundled apps run as normal dock apps; standalone Canopy runs as menubar tray
	if bundledApp {
		darwin.SetAppMode("normal", "", "", 0)
	} else {
		darwin.SetAppMode("menubar", "app.dashed", "Canopy", 0)

		// Scan sample_apps/ for the Apps submenu
		sampleApps := scanSampleApps()
		if len(sampleApps) > 0 {
			darwin.SetStatusMenuApps(sampleApps)
		}
	}

	// Show splash only when loading a fixture/prompt (not bare startup)
	hasForegroundWork := tr != nil
	if hasForegroundWork {
		splashTitle := "Canopy"
		if bundledApp {
			splashTitle = bundledAppName(args[0])
		}
		darwin.ShowSplashWindow(splashTitle, 400, 200)
	}
	disp := darwin.NewDispatcher()
	rend := darwin.NewRenderer()
	sess := engine.NewSession(rend, disp)
	sess.SetNativeProvider(darwin.NewNativeProvider())
	if ffiRegistry != nil {
		sess.SetFFI(ffiRegistry)
	}
	sess.SetLibrary(lib)
	if recorder != nil {
		sess.SetRecorder(recorder)
	}

	// Pre-declare pm and cm so the process factory closure can capture them
	var pm *engine.ProcessManager
	var cm *engine.ChannelManager

	// Set up process manager with transport factory
	pm = engine.NewProcessManager(sess, func(cfg protocol.ProcessTransportConfig) (engine.ProcessTransport, error) {
		switch cfg.Type {
		case "file":
			return createFileTransportOrError(cfg.Path)
		case "interval":
			if cfg.Interval <= 0 {
				return nil, fmt.Errorf("interval transport requires interval > 0")
			}
			if cfg.Message == nil {
				return nil, fmt.Errorf("interval transport requires message")
			}
			return transport.NewIntervalTransport(cfg.Interval, cfg.Message), nil
		case "llm":
			provider, err := createProvider(cfg.Provider, "")
			if err != nil {
				return nil, fmt.Errorf("create LLM provider: %w", err)
			}
			llmCfg := transport.LLMConfig{
				Provider: provider,
				Model:    cfg.Model,
				Prompt:   cfg.Prompt,
				Mode:     "tools",
			}
			return transport.NewLLMTransport(llmCfg), nil
		case "claude-code":
			cc := transport.NewClaudeCodeTransport(transport.ClaudeCodeConfig{
				Prompt: cfg.Prompt,
				Model:  cfg.Model,
			})
			cc.StartMCP = func() (*transport.MCPServerHandle, error) {
				opts := []mcp.ServerOption{
					mcp.WithProcessManager(pm),
					mcp.WithChannelManager(cm),
					mcp.WithComponentReference(transport.ComponentReference(lib.ComponentListForPrompt())),
				}
				srv := mcp.NewServer(sess, rend, disp, opts...)
				port, cleanup, err := srv.ListenHTTP("localhost:0")
				if err != nil {
					return nil, err
				}
				return &transport.MCPServerHandle{
					Port:    port,
					Cleanup: cleanup,
					PushAction: func(surfaceID, event string, data map[string]interface{}) {
						srv.PushAction(mcp.PendingAction{
							SurfaceID: surfaceID,
							Event:     event,
							Data:      data,
						})
					},
				}, nil
			}
			return cc, nil
		default:
			return nil, fmt.Errorf("unknown process transport type: %q", cfg.Type)
		}
	})
	sess.SetProcessManager(pm)

	// Set up channel manager for inter-process communication
	cm = engine.NewChannelManager(sess)
	sess.SetChannelManager(cm)

	// Wire window events from native delegate → EventManager
	darwin.SetWindowEventHandler(func(surfaceID, event, data string) {
		sess.EventManager().Fire(event, surfaceID, data)
	})

	// Wire system events from native observers → EventManager
	darwin.SetSystemEventHandler(func(event, data string) {
		sess.EventManager().Fire(event, "", data)
	})

	// Wire hardware events (distributed notifications) → EventManager
	darwin.SetHardwareEventHandler(func(subscriptionID uint64, data string) {
		// Hardware events with subscription IDs are handled per-subscription
		// For now, distributed notifications fire through the generic system event path
		sess.EventManager().Fire("system.ipc.distributed", "", data)
	})

	// Set up on-demand event source control (bluetooth, location, USB, distributed notifications)
	sess.EventManager().SetControl(func(event, action string, config map[string]interface{}) {
		disp.RunOnMain(func() {
			switch event {
			case "system.bluetooth":
				if action == "start" {
					darwin.StartBluetoothObserver()
				} else {
					darwin.StopBluetoothObserver()
				}
			case "system.location":
				if action == "start" {
					darwin.StartLocationObserver()
				} else {
					darwin.StopLocationObserver()
				}
			case "system.usb":
				if action == "start" {
					darwin.StartUSBObserver()
				} else {
					darwin.StopUSBObserver()
				}
			case "system.ipc.distributed":
				if config == nil {
					return
				}
				if action == "start" {
					name, _ := config["name"].(string)
					subID, _ := config["subscriptionID"].(string)
					if name != "" && subID != "" {
						// Use a hash of the subscription ID as the native callback ID
						darwin.ObserveDistributedNotification(name, uint64(hash(subID)))
					}
				} else {
					subID, _ := config["subscriptionID"].(string)
					if subID != "" {
						darwin.UnobserveDistributedNotification(uint64(hash(subID)))
					}
				}
			}
		})
	})

	// Start always-on system observers (lightweight NSNotificationCenter subscriptions)
	disp.RunOnMain(func() {
		darwin.StartAppearanceObserver()
		darwin.StartPowerObserver()
		darwin.StartDisplayObserver()
		darwin.StartLocaleObserver()
		darwin.StartClipboardObserver(500) // poll every 500ms
		darwin.StartNetworkObserver()
		darwin.StartAccessibilityObserver()
		darwin.StartThermalObserver()
	})

	// Wire status bar menu handlers
	darwin.OnStatusMenuAppClicked = func(appPath string) {
		jlog.Infof("main", "", "launching app: %s", appPath)
		processID := "app_" + filepath.Base(appPath)
		err := pm.Create(protocol.CreateProcess{
			ProcessID: processID,
			Transport: protocol.ProcessTransportConfig{
				Type: "file",
				Path: appPath,
			},
		})
		if err != nil {
			jlog.Errorf("main", "", "failed to launch app %s: %v", appPath, err)
		}
	}
	darwin.OnStatusMenuSettingsClicked = func() {
		jlog.Infof("main", "", "settings clicked (not yet implemented)")
	}

	// Initialize package registry
	pkgRegistry, err := registry.New()
	if err != nil {
		jlog.Errorf("main", "", "failed to init package registry: %v", err)
	}

	// Start embedded MCP server on stdin/stdout
	mcpOpts := []mcp.ServerOption{mcp.WithProcessManager(pm), mcp.WithChannelManager(cm)}
	if pkgRegistry != nil {
		mcpOpts = append(mcpOpts, mcp.WithRegistry(pkgRegistry))
	}
	mcpServer := mcp.NewServer(sess, rend, disp, mcpOpts...)
	toolNames := mcpServer.ToolNames()
	jlog.Infof("main", "", "mcp: listening on stdin/stdout (%d tools: %s)", len(toolNames), strings.Join(toolNames, ", "))
	go func() {
		defer func() {
			if r := recover(); r != nil {
				jlog.Errorf("main", "", "panic in mcp stdin server: %v", r)
			}
		}()
		mcpTransport := mcp.NewStdioTransport(os.Stdin, os.Stdout)
		ctx := context.Background()
		if err := mcpServer.Run(ctx, mcpTransport); err != nil {
			jlog.Errorf("main", "", "mcp: server error: %v", err)
		}
	}()

	// Optional: also listen for MCP on HTTP alongside stdin/stdout
	if *mcpHTTPAddr != "" {
		port, cleanup, err := mcpServer.ListenHTTP(*mcpHTTPAddr)
		if err != nil {
			jlog.Errorf("main", "", "mcp-http: failed to listen: %v", err)
		} else {
			defer cleanup()
			jlog.Infof("main", "", "mcp-http: listening on localhost:%d", port)
		}
	}

	// Wire Claude Code transport status updates to splash window
	if ccTr != nil {
		ccTr.OnStatus = func(status string) {
			disp.RunOnMain(func() { darwin.UpdateSplashStatus(status) })
		}
	}

	// Wire Claude Code transport's StartMCP now that deps are available
	if ccTr != nil {
		ccTr.StartMCP = func() (*transport.MCPServerHandle, error) {
			opts := []mcp.ServerOption{
				mcp.WithProcessManager(pm),
				mcp.WithChannelManager(cm),
				mcp.WithComponentReference(transport.ComponentReference(lib.ComponentListForPrompt())),
			}
			if pkgRegistry != nil {
				opts = append(opts, mcp.WithRegistry(pkgRegistry))
			}
			srv := mcp.NewServer(sess, rend, disp, opts...)
			srv.OnToolCall = func(toolName string) {
				var status string
				switch toolName {
				case "send_message":
					status = "Building UI..."
				case "take_screenshot":
					status = "Verifying layout..."
				case "get_tree", "get_component":
					status = "Inspecting components..."
				case "get_data_model", "set_data_model":
					status = "Updating data..."
				case "click", "fill", "toggle", "interact":
					status = "Testing interactions..."
				default:
					return
				}
				disp.RunOnMain(func() { darwin.UpdateSplashStatus(status) })
			}
			port, cleanup, err := srv.ListenHTTP("localhost:0")
			if err != nil {
				return nil, err
			}
			return &transport.MCPServerHandle{
				Port:    port,
				Cleanup: cleanup,
				PushAction: func(surfaceID, event string, data map[string]interface{}) {
					srv.PushAction(mcp.PendingAction{
						SurfaceID: surfaceID,
						Event:     event,
						Data:      data,
					})
				},
			}, nil
		}
	}

	// Wire Cmd+L follow-up prompt for LLM transport
	if llmTr != nil {
		lt := llmTr
		darwin.SetFollowUpEnabled(true)
		darwin.OnFollowUpTriggered = func() {
			go func() {
				disp.RunOnMain(func() { darwin.SetFollowUpEnabled(false) })
				result := darwin.ShowFollowUpPanel()
				disp.RunOnMain(func() { darwin.SetFollowUpEnabled(true) })
				if result == "" {
					return
				}
				lt.SendFollowUp(result)
			}()
		}
	}

	// Wire Cmd+L follow-up prompt for Claude Code transport
	if ccTr != nil {
		// Override OnDone to also re-enable the menu item (needs disp)
		origOnDone := ccTr.OnDone
		ccTr.OnDone = func() {
			if origOnDone != nil {
				origOnDone()
			}
			disp.RunOnMain(func() { darwin.SetFollowUpEnabled(true) })
		}

		darwin.OnFollowUpTriggered = func() {
			go func() {
				disp.RunOnMain(func() { darwin.SetFollowUpEnabled(false) })
				result := darwin.ShowFollowUpPanel()
				if result == "" {
					disp.RunOnMain(func() { darwin.SetFollowUpEnabled(true) })
					return
				}
				ccTr.SendFollowUp(result)
			}()
		}
	}

	// Wire action events — route to process transport if ProcessID is set,
	// also push to MCP server for polling, then forward to main transport.
	sess.OnAction = func(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
		if event.ProcessID != "" && pm != nil {
			pm.SendTo(event.ProcessID, &protocol.Message{
				Type:      protocol.MsgUpdateDataModel,
				SurfaceID: surfaceID,
			})
			return
		}
		// Push to MCP action queue for get_pending_actions polling
		mcpServer.PushAction(mcp.PendingAction{
			SurfaceID: surfaceID,
			Event:     event.Name,
			Data:      data,
		})
		if tr != nil {
			tr.SendAction(surfaceID, event, data)
		}
	}

	// Process messages in a goroutine
	// For LLM mode, also handle layout feedback and screenshot requests.
	// Suppress user callbacks during generation to prevent interactions from
	// changing data model state and causing test assertion failures.
	var screenshotCh <-chan transport.ScreenshotRequest
	if llmTr != nil {
		screenshotCh = llmTr.ScreenshotReqCh
		darwin.SetSuppressCallbacks(true)
	}

	if tr == nil {
		// No transport — tray-only mode. Skip to the run loop.
		goto runLoop
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				jlog.Errorf("main", "", "panic in transport goroutine: %v", r)
				fmt.Fprintf(os.Stderr, "[PANIC RECOVERED] transport goroutine: %v\n", r)
			}
		}()

		// Update splash status based on transport type
		if llmTr != nil {
			disp.RunOnMain(func() { darwin.UpdateSplashStatus("Connecting to " + *llmProvider + "...") })
		} else if ccTr != nil {
			disp.RunOnMain(func() { darwin.UpdateSplashStatus("Starting Claude Code...") })
		} else {
			disp.RunOnMain(func() { darwin.UpdateSplashStatus("Loading file...") })
		}

		tr.Start()
		errCh := tr.Errors()
		firstMessage := true

		for {
			select {
			case msg, ok := <-tr.Messages():
				if !ok {
					sess.FlushPendingComponents()
					if llmTr != nil {
						darwin.SetSuppressCallbacks(false)
					}
					jlog.Info("main", "", "transport closed")
					return
				}
				// Nil sentinel = first turn done (all first-turn messages consumed)
				if msg == nil {
					if llmTr != nil && llmTr.OnInitialTurnDone != nil {
						llmTr.OnInitialTurnDone()
					}
					continue
				}
				if firstMessage {
					firstMessage = false
					disp.RunOnMain(func() { darwin.UpdateSplashStatus("Building UI...") })
				}
				// Execute test messages and return real results to the LLM
				if msg.Type == protocol.MsgTest && llmTr != nil {
					sess.FlushPendingComponents()
					tm := msg.Body.(protocol.TestMessage)
					result := engine.ExecuteTestLite(sess, rend, tm)
					llmTr.TestResultCh <- engine.FormatTestResult(result)
					continue
				}
				// For updateComponents in LLM mode: buffer without rendering.
				// Components accumulate across batches and render all at once
				// when a non-updateComponents message triggers a flush.
				if msg.Type == protocol.MsgUpdateComponents && llmTr != nil {
					sess.HandleMessage(msg) // just buffers in pendingComponents
					uc := msg.Body.(protocol.UpdateComponents)
					llmTr.LayoutResultCh <- fmt.Sprintf("ok — %d components buffered", len(uc.Components))
					continue
				}
				sess.HandleMessage(msg)

			case err, ok := <-errCh:
				if !ok {
					// Errors channel closed — nil it out to stop selecting on it.
					// Keep draining messages (file transport closes both channels
					// simultaneously; exiting here would lose buffered messages).
					errCh = nil
					continue
				}
				jlog.Errorf("main", "", "transport error: %v", err)

			case req := <-screenshotCh:
				// Flush any buffered components before capturing
				sess.FlushPendingComponents()
				// Screenshot request from LLM transport — capture on main thread
				pngData := dispatchSyncMain(disp, func() []byte {
					data, err := rend.CaptureWindow(req.SurfaceID)
					if err != nil {
						jlog.Errorf("main", "", "screenshot capture failed: %v", err)
						return nil
					}
					return data
				})
				if pngData == nil {
					req.ResultCh <- transport.ScreenshotResult{Err: fmt.Errorf("capture failed for surface %q", req.SurfaceID)}
				} else {
					req.ResultCh <- transport.ScreenshotResult{Data: pngData}
				}
			}
		}
	}()

runLoop:
	// Handle SIGINT/SIGTERM: clean exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		jlog.Info("main", "", "signal received, cleaning up...")
		// If cache was already finalized (generation complete), just exit
		select {
		case <-cacheFinalized:
			jlog.Info("main", "", "cache already finalized, exiting")
			os.Exit(0)
		default:
		}
		// Generation still in progress — kill transport, wait briefly for
		// OnDone to fire and finalize, then exit
		if ccTr != nil {
			ccTr.Stop()
		}
		select {
		case <-cacheFinalized:
			jlog.Info("main", "", "cache finalized after signal, exiting")
		case <-time.After(2 * time.Second):
			jlog.Info("main", "", "cache finalization timed out, discarding partial cache")
			if recorder != nil {
				recorder.Close()
				recorder = nil
			}
		}
		os.Exit(0)
	}()

	if ccTr != nil {
		defer ccTr.Stop()
	}

	// Run the macOS event loop (blocks forever)
	darwin.AppRun()
}

// createFileTransportOrError resolves a file or directory path to a Transport.
// Returns an error instead of calling os.Exit, so callers can handle gracefully.
func createFileTransportOrError(path string) (transport.Transport, error) {
	info, err := os.Stat(path)
	if err != nil {
		// Let FileTransport handle the error
		return transport.NewFileTransport(path), nil
	}
	if !info.IsDir() {
		return transport.NewFileTransport(path), nil
	}

	// Directory mode: prefer a canonical entry point
	for _, entry := range []string{"app.jsonl", "main.jsonl"} {
		ep := filepath.Join(path, entry)
		if _, err := os.Stat(ep); err == nil {
			return transport.NewFileTransport(ep), nil
		}
	}

	// Fallback: read all .jsonl files sorted lexicographically
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		return nil, fmt.Errorf("no .jsonl files found in %s", path)
	}

	return transport.NewDirTransport(path, files), nil
}

// createFileTransport wraps createFileTransportOrError for CLI startup (fatal on error).
func createFileTransport(path string) transport.Transport {
	tr, err := createFileTransportOrError(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	return tr
}

func createProvider(name string, apiKey string) (anyllm.Provider, error) {
	var opts []anyllm.Option
	if apiKey != "" {
		opts = append(opts, anyllm.WithAPIKey(apiKey))
	}

	switch name {
	case "anthropic":
		return transport.NewAnthropicProvider(apiKey)
	case "openai":
		return openai.New(opts...)
	case "gemini":
		return gemini.New(opts...)
	case "ollama":
		return ollama.New(opts...)
	case "deepseek":
		return deepseek.New(opts...)
	case "groq":
		return groq.New(opts...)
	case "mistral":
		return mistral.New(opts...)
	default:
		return nil, fmt.Errorf("unknown provider %q (supported: anthropic, openai, gemini, ollama, deepseek, groq, mistral)", name)
	}
}

// dispatchSyncMain dispatches a function to the main thread and blocks until it completes.
func dispatchSyncMain[T any](disp renderer.Dispatcher, fn func() T) T {
	ch := make(chan T, 1)
	disp.RunOnMain(func() {
		defer func() {
			if r := recover(); r != nil {
				jlog.Errorf("main", "", "dispatch panic: %v", r)
				var zero T
				ch <- zero
			}
		}()
		ch <- fn()
	})
	return <-ch
}

func runMCP(args []string) {
	darwin.AppInit()
	darwin.SetAppMode("menubar", "app.dashed", "Canopy", 0)
	disp := darwin.NewDispatcher()
	rend := darwin.NewRenderer()
	sess := engine.NewSession(rend, disp)
	sess.SetNativeProvider(darwin.NewNativeProvider())

	// Set up process manager for MCP mode
	pm := engine.NewProcessManager(sess, func(cfg protocol.ProcessTransportConfig) (engine.ProcessTransport, error) {
		switch cfg.Type {
		case "file":
			return transport.NewFileTransport(cfg.Path), nil
		default:
			return nil, fmt.Errorf("unknown process transport type: %q", cfg.Type)
		}
	})
	sess.SetProcessManager(pm)

	// Set up channel manager
	cm := engine.NewChannelManager(sess)
	sess.SetChannelManager(cm)

	// Wire window events from native delegate → EventManager
	darwin.SetWindowEventHandler(func(surfaceID, event, data string) {
		sess.EventManager().Fire(event, surfaceID, data)
	})

	// Wire system events from native observers → EventManager
	darwin.SetSystemEventHandler(func(event, data string) {
		sess.EventManager().Fire(event, "", data)
	})

	// Wire hardware events → EventManager
	darwin.SetHardwareEventHandler(func(subscriptionID uint64, data string) {
		sess.EventManager().Fire("system.ipc.distributed", "", data)
	})

	// Set up on-demand event source control
	sess.EventManager().SetControl(func(event, action string, config map[string]interface{}) {
		disp.RunOnMain(func() {
			switch event {
			case "system.bluetooth":
				if action == "start" {
					darwin.StartBluetoothObserver()
				} else {
					darwin.StopBluetoothObserver()
				}
			case "system.location":
				if action == "start" {
					darwin.StartLocationObserver()
				} else {
					darwin.StopLocationObserver()
				}
			case "system.usb":
				if action == "start" {
					darwin.StartUSBObserver()
				} else {
					darwin.StopUSBObserver()
				}
			case "system.ipc.distributed":
				if config == nil {
					return
				}
				if action == "start" {
					name, _ := config["name"].(string)
					subID, _ := config["subscriptionID"].(string)
					if name != "" && subID != "" {
						darwin.ObserveDistributedNotification(name, hash(subID))
					}
				} else {
					subID, _ := config["subscriptionID"].(string)
					if subID != "" {
						darwin.UnobserveDistributedNotification(hash(subID))
					}
				}
			}
		})
	})

	// Start always-on system observers
	disp.RunOnMain(func() {
		darwin.StartAppearanceObserver()
		darwin.StartPowerObserver()
		darwin.StartDisplayObserver()
		darwin.StartLocaleObserver()
		darwin.StartClipboardObserver(500)
		darwin.StartNetworkObserver()
		darwin.StartAccessibilityObserver()
		darwin.StartThermalObserver()
	})

	// If a file arg is provided, load it as initial UI
	if len(args) > 0 {
		tr := createFileTransport(args[0])
		go func() {
			defer func() {
				if r := recover(); r != nil {
					jlog.Errorf("main", "", "panic in mcp file transport: %v", r)
				}
			}()

			tr.Start()
			for {
				select {
				case msg, ok := <-tr.Messages():
					if !ok {
						sess.FlushPendingComponents()
						jlog.Info("main", "", "mcp: file transport closed")
						return
					}
					sess.HandleMessage(msg)
				case err, ok := <-tr.Errors():
					if !ok {
						return
					}
					jlog.Errorf("main", "", "mcp: file transport error: %v", err)
				}
			}
		}()
	}

	mcpTransport := mcp.NewStdioTransport(os.Stdin, os.Stdout)
	mcpPkgRegistry, _ := registry.New()
	mcpOpts2 := []mcp.ServerOption{mcp.WithProcessManager(pm), mcp.WithChannelManager(cm)}
	if mcpPkgRegistry != nil {
		mcpOpts2 = append(mcpOpts2, mcp.WithRegistry(mcpPkgRegistry))
	}
	mcpServer := mcp.NewServer(sess, rend, disp, mcpOpts2...)
	toolNames := mcpServer.ToolNames()

	// Wire actions to MCP action queue for get_pending_actions polling
	sess.OnAction = func(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
		mcpServer.PushAction(mcp.PendingAction{
			SurfaceID: surfaceID,
			Event:     event.Name,
			Data:      data,
		})
	}
	jlog.Infof("main", "", "mcp: server started on stdin/stdout (%d tools: %s)", len(toolNames), strings.Join(toolNames, ", "))

	// Run MCP server in goroutine; on EOF, quit the app
	go func() {
		defer func() {
			if r := recover(); r != nil {
				jlog.Errorf("main", "", "panic in mcp server: %v", r)
			}
		}()
		ctx := context.Background()
		if err := mcpServer.Run(ctx, mcpTransport); err != nil {
			jlog.Errorf("main", "", "mcp: server error: %v", err)
		}
		darwin.AppStop()
	}()

	darwin.AppRun()
}

func runTests(path string) {
	darwin.AppInit()
	// Use synchronous dispatcher — we're already on the main thread.
	// Real darwin.Dispatcher uses dispatch_async which won't execute until the run loop runs.
	disp := &renderer.MockDispatcher{}
	rend := darwin.NewRenderer()

	results, err := engine.RunTestFile(path, rend, disp, engine.WithNativeProvider(darwin.NewNativeProvider()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	passed, failed := 0, 0
	for _, r := range results {
		if r.Passed {
			passed++
			fmt.Printf("PASS  %s (%d assertions)\n", r.Name, r.Assertions)
		} else {
			failed++
			fmt.Printf("FAIL  %s\n", r.Name)
			fmt.Printf("      %s\n", r.Error)
		}
	}

	fmt.Printf("\nResults: %d passed, %d failed, %d total\n", passed, failed, len(results))
	if failed > 0 {
		os.Exit(1)
	}
}

// scanSampleApps scans sample_apps/ and ~/.canopy/apps/ for the status bar Apps submenu.
func scanSampleApps() []darwin.StatusMenuApp {
	// Look for sample_apps relative to the executable, then current dir
	dirs := []string{"sample_apps"}
	exePath, err := os.Executable()
	if err == nil {
		dirs = append([]string{filepath.Join(filepath.Dir(exePath), "sample_apps")}, dirs...)
	}

	var apps []darwin.StatusMenuApp
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			absPath, _ := filepath.Abs(filepath.Join(dir, name))
			apps = append(apps, darwin.StatusMenuApp{
				Label: titleCase(name),
				Path:  absPath,
				Icon:  "app",
			})
		}
		if len(apps) > 0 {
			break // found sample_apps in one location, don't double-scan
		}
	}

	// Also scan installed packages from ~/.canopy/apps/{host}/{owner}/{repo}/
	home, err := os.UserHomeDir()
	if err == nil {
		appsDir := filepath.Join(home, ".canopy", "apps")
		hosts, _ := os.ReadDir(appsDir)
		for _, host := range hosts {
			if !host.IsDir() {
				continue
			}
			owners, _ := os.ReadDir(filepath.Join(appsDir, host.Name()))
			for _, owner := range owners {
				if !owner.IsDir() {
					continue
				}
				pkgs, _ := os.ReadDir(filepath.Join(appsDir, host.Name(), owner.Name()))
				for _, pkg := range pkgs {
					if !pkg.IsDir() {
						continue
					}
					pkgPath := filepath.Join(appsDir, host.Name(), owner.Name(), pkg.Name())
					label := titleCase(pkg.Name())
					icon := "app"

					// Try to read canopy.json for better metadata
					manifestData, err := os.ReadFile(filepath.Join(pkgPath, "canopy.json"))
					if err == nil {
						var m struct {
							Name string `json:"name"`
							Icon string `json:"icon"`
						}
						if json.Unmarshal(manifestData, &m) == nil {
							if m.Name != "" {
								label = m.Name
							}
							if m.Icon != "" {
								icon = m.Icon
							}
						}
					}

					apps = append(apps, darwin.StatusMenuApp{
						Label: label,
						Path:  pkgPath,
						Icon:  icon,
					})
				}
			}
		}
	}

	return apps
}

// bundledAppName reads the app name from canopy.json in the given directory,
// falling back to the directory name.
func bundledAppName(appDir string) string {
	data, err := os.ReadFile(filepath.Join(appDir, "canopy.json"))
	if err == nil {
		var m struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &m) == nil && m.Name != "" {
			return m.Name
		}
	}
	return titleCase(filepath.Base(appDir))
}

func titleCase(name string) string {
	label := strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(label)
	for j, w := range words {
		if len(w) > 0 {
			words[j] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
