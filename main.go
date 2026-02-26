package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"jview/engine"
	"jview/jlog"
	"jview/mcp"
	"jview/platform/darwin"
	"jview/protocol"
	"jview/renderer"
	"jview/transport"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

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

func main() {
	// macOS requires the main thread for AppKit
	runtime.LockOSThread()

	// Load .env file (does not override existing env vars)
	loadEnvFile()

	// Initialize central logger
	jlog.Init(jlog.Config{
		MaxEntries: 10000,
		MinLevel:   jlog.LevelInfo,
		LogDir:     "~/.jview/logs",
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

	ffiConfigPath := flag.String("ffi-config", "", "Path to FFI convention file (JSON) for native function calls")
	llmProvider := flag.String("llm", "anthropic", "LLM provider: anthropic, openai, gemini, ollama, deepseek, groq, mistral")
	model := flag.String("model", "claude-opus-4-6", "Model name (default: claude-opus-4-6)")
	prompt := flag.String("prompt", "", "Prompt describing the UI to build")
	mode := flag.String("mode", "tools", "LLM mode: tools (default) or raw")
	apiKey := flag.String("api-key", "", "API key (overrides environment variable)")
	promptFile := flag.String("prompt-file", "", "Read prompt from file (overrides --prompt)")
	regenerate := flag.Bool("regenerate", false, "Force fresh LLM call, ignore cache")
	generateOnly := flag.Bool("generate-only", false, "Generate JSONL and exit without opening a window")
	flag.Parse()

	if *promptFile != "" {
		data, err := os.ReadFile(*promptFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading prompt file: %v\n", err)
			os.Exit(1)
		}
		*prompt = string(data)
	}

	if *generateOnly && *promptFile == "" {
		fmt.Fprintf(os.Stderr, "error: --generate-only requires --prompt-file\n")
		os.Exit(1)
	}

	var tr transport.Transport
	var llmTr *transport.LLMTransport // non-nil when using LLM transport
	var generateDone chan struct{}    // closed when generate-only can exit

	args := flag.Args()
	if len(args) > 0 && *prompt == "" {
		// File or directory mode: positional arg with no --prompt
		tr = createFileTransport(args[0])
	} else if *prompt != "" {
		// Check cache for prompt-file mode
		if *promptFile != "" && !*regenerate && transport.CacheValid(*promptFile) {
			jsonlPath, _, _ := transport.CachePaths(*promptFile)
			if *generateOnly {
				jlog.Infof("main", "", "cache hit: %s (up to date)", jsonlPath)
				os.Exit(0)
			}
			jlog.Infof("main", "", "cache hit: using %s", jsonlPath)
			tr = transport.NewFileTransport(jsonlPath)
		} else {
			// LLM mode (cache miss or no prompt-file)
			provider, err := createProvider(*llmProvider, *apiKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			cfg := transport.LLMConfig{
				Provider: provider,
				Model:    *model,
				Prompt:   *prompt,
				Mode:     *mode,
			}

			// Set up recording if using prompt-file
			if *promptFile != "" {
				_, _, tmpPath := transport.CachePaths(*promptFile)
				cfg.RecordTo = tmpPath
			}

			lt := transport.NewLLMTransport(cfg)

			// Finalize cache on first turn completion
			if *promptFile != "" {
				jsonlPath, _, tmpPath := transport.CachePaths(*promptFile)
				if *generateOnly {
					generateDone = make(chan struct{})
				}
				lt.OnInitialTurnDone = func() {
					lt.CloseRecorder()
					if err := os.Rename(tmpPath, jsonlPath); err != nil {
						jlog.Errorf("main", "", "cache: rename failed: %v", err)
					} else {
						jlog.Infof("main", "", "cache: wrote %s", jsonlPath)
					}
					if err := transport.WriteHashFile(*promptFile); err != nil {
						jlog.Errorf("main", "", "cache: write hash failed: %v", err)
					}
					if generateDone != nil {
						close(generateDone)
					}
					// Re-enable user callbacks now that generation is complete
					darwin.SetSuppressCallbacks(false)
				}
			}

			// For non-prompt-file LLM mode, still re-enable callbacks after generation
			if lt.OnInitialTurnDone == nil {
				lt.OnInitialTurnDone = func() {
					darwin.SetSuppressCallbacks(false)
				}
			}

			tr = lt
			llmTr = lt
		}
	} else {
		fmt.Fprintf(os.Stderr, "usage: jview <file.jsonl>\n")
		fmt.Fprintf(os.Stderr, "       jview --prompt \"Build a todo app\"\n")
		fmt.Fprintf(os.Stderr, "       jview --prompt-file prompt.txt\n")
		fmt.Fprintf(os.Stderr, "       jview --llm openai --model gpt-4o --prompt-file prompt.txt\n")
		fmt.Fprintf(os.Stderr, "       jview --prompt-file prompt.txt --generate-only\n")
		fmt.Fprintf(os.Stderr, "       jview --ffi-config libs.json testdata/app.jsonl\n")
		os.Exit(1)
	}

	// Generate-only mode: drain messages without AppKit
	if *generateOnly {
		go func() {
			tr.Start()
			for {
				select {
				case _, ok := <-tr.Messages():
					if !ok {
						return
					}
				case _, ok := <-tr.Errors():
					if !ok {
						return
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
	disp := darwin.NewDispatcher()
	rend := darwin.NewRenderer()
	sess := engine.NewSession(rend, disp)
	if ffiRegistry != nil {
		sess.SetFFI(ffiRegistry)
	}

	// Set up process manager with transport factory
	pm := engine.NewProcessManager(sess, func(cfg protocol.ProcessTransportConfig) (engine.ProcessTransport, error) {
		switch cfg.Type {
		case "file":
			ft := transport.NewFileTransport(cfg.Path)
			return ft, nil
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
		default:
			return nil, fmt.Errorf("unknown process transport type: %q", cfg.Type)
		}
	})
	sess.SetProcessManager(pm)

	// Set up channel manager for inter-process communication
	cm := engine.NewChannelManager(sess)
	sess.SetChannelManager(cm)

	// Start embedded MCP server on stdin/stdout
	mcpServer := mcp.NewServer(sess, rend, disp, mcp.WithProcessManager(pm), mcp.WithChannelManager(cm))
	toolNames := mcpServer.ToolNames()
	jlog.Infof("main", "", "mcp: listening on stdin/stdout (%d tools: %s)", len(toolNames), strings.Join(toolNames, ", "))
	go func() {
		mcpTransport := mcp.NewStdioTransport(os.Stdin, os.Stdout)
		ctx := context.Background()
		if err := mcpServer.Run(ctx, mcpTransport); err != nil {
			jlog.Errorf("main", "", "mcp: server error: %v", err)
		}
	}()

	// Wire action events — route to process transport if ProcessID is set, else main transport
	sess.OnAction = func(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
		if event.ProcessID != "" && pm != nil {
			pm.SendTo(event.ProcessID, &protocol.Message{
				Type:      protocol.MsgUpdateDataModel,
				SurfaceID: surfaceID,
			})
			return
		}
		tr.SendAction(surfaceID, event, data)
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

	go func() {
		defer func() {
			if r := recover(); r != nil {
				jlog.Errorf("main", "", "panic in transport goroutine: %v", r)
				fmt.Fprintf(os.Stderr, "[PANIC RECOVERED] transport goroutine: %v\n", r)
			}
		}()

		tr.Start()

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

			case err, ok := <-tr.Errors():
				if !ok {
					return
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

	// Run the macOS event loop (blocks forever)
	darwin.AppRun()
}

// createFileTransport handles both single file and directory mode.
// Directory mode: reads all *.jsonl files sorted lexicographically.
func createFileTransport(path string) transport.Transport {
	info, err := os.Stat(path)
	if err != nil {
		// Let FileTransport handle the error
		return transport.NewFileTransport(path)
	}
	if !info.IsDir() {
		return transport.NewFileTransport(path)
	}

	// Directory mode: create a virtual main.jsonl that includes all files
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading directory: %v\n", err)
		os.Exit(1)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no .jsonl files found in %s\n", path)
		os.Exit(1)
	}

	return transport.NewDirTransport(path, files)
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
	disp := darwin.NewDispatcher()
	rend := darwin.NewRenderer()
	sess := engine.NewSession(rend, disp)

	// No-op action handler — MCP mode has no transport to forward actions to
	sess.OnAction = func(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {}

	// Set up channel manager
	cm := engine.NewChannelManager(sess)
	sess.SetChannelManager(cm)

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
	mcpServer := mcp.NewServer(sess, rend, disp, mcp.WithChannelManager(cm))
	toolNames := mcpServer.ToolNames()
	jlog.Infof("main", "", "mcp: server started on stdin/stdout (%d tools: %s)", len(toolNames), strings.Join(toolNames, ", "))

	// Run MCP server in goroutine; on EOF, quit the app
	go func() {
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

	results, err := engine.RunTestFile(path, rend, disp)
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
