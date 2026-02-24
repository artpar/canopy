package main

import (
	"flag"
	"fmt"
	"jview/engine"
	"jview/platform/darwin"
	"jview/protocol"
	"jview/renderer"
	"jview/transport"
	"log"
	"os"
	"runtime"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/deepseek"
	"github.com/mozilla-ai/any-llm-go/providers/gemini"
	"github.com/mozilla-ai/any-llm-go/providers/groq"
	"github.com/mozilla-ai/any-llm-go/providers/mistral"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
	"github.com/mozilla-ai/any-llm-go/providers/openai"
)

func main() {
	// macOS requires the main thread for AppKit
	runtime.LockOSThread()

	// Handle "jview test <file>" — uses real AppKit for e2e testing
	if len(os.Args) >= 3 && os.Args[1] == "test" {
		runTests(os.Args[2])
		return
	}

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
	var generateDone chan struct{} // closed when generate-only can exit

	args := flag.Args()
	if len(args) > 0 && *prompt == "" {
		// File mode: positional arg with no --prompt
		tr = transport.NewFileTransport(args[0])
	} else if *prompt != "" {
		// Check cache for prompt-file mode
		if *promptFile != "" && !*regenerate && transport.CacheValid(*promptFile) {
			jsonlPath, _, _ := transport.CachePaths(*promptFile)
			if *generateOnly {
				log.Printf("cache hit: %s (up to date)", jsonlPath)
				os.Exit(0)
			}
			log.Printf("cache hit: using %s", jsonlPath)
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
						log.Printf("cache: rename failed: %v", err)
					} else {
						log.Printf("cache: wrote %s", jsonlPath)
					}
					if err := transport.WriteHashFile(*promptFile); err != nil {
						log.Printf("cache: write hash failed: %v", err)
					}
					if generateDone != nil {
						close(generateDone)
					}
				}
			}

			tr = lt
		}
	} else {
		fmt.Fprintf(os.Stderr, "usage: jview <file.jsonl>\n")
		fmt.Fprintf(os.Stderr, "       jview --prompt \"Build a todo app\"\n")
		fmt.Fprintf(os.Stderr, "       jview --prompt-file prompt.txt\n")
		fmt.Fprintf(os.Stderr, "       jview --llm openai --model gpt-4o --prompt-file prompt.txt\n")
		fmt.Fprintf(os.Stderr, "       jview --prompt-file prompt.txt --generate-only\n")
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

	// Interactive mode: initialize platform and engine
	darwin.AppInit()
	disp := darwin.NewDispatcher()
	rend := darwin.NewRenderer()
	sess := engine.NewSession(rend, disp)

	// Wire events for LLM transport
	if lt, ok := tr.(*transport.LLMTransport); ok {
		sess.OnAction = func(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
			lt.SendAction(surfaceID, event, data)
		}
	}

	// Process messages in a goroutine
	go func() {
		tr.Start()

		for {
			select {
			case msg, ok := <-tr.Messages():
				if !ok {
					log.Println("main: transport closed")
					return
				}
				sess.HandleMessage(msg)

			case err, ok := <-tr.Errors():
				if !ok {
					return
				}
				log.Printf("main: transport error: %v", err)
			}
		}
	}()

	// Run the macOS event loop (blocks forever)
	darwin.AppRun()
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
