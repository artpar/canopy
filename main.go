package main

import (
	"flag"
	"fmt"
	"jview/engine"
	"jview/platform/darwin"
	"jview/protocol"
	"jview/transport"
	"log"
	"os"
	"runtime"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/anthropic"
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

	llmProvider := flag.String("llm", "anthropic", "LLM provider: anthropic, openai, gemini, ollama, deepseek, groq, mistral")
	model := flag.String("model", "claude-haiku-4-5-20251001", "Model name (default: claude-haiku-4-5-20251001)")
	prompt := flag.String("prompt", "", "Prompt describing the UI to build")
	mode := flag.String("mode", "tools", "LLM mode: tools (default) or raw")
	apiKey := flag.String("api-key", "", "API key (overrides environment variable)")
	flag.Parse()

	// Initialize platform
	darwin.AppInit()
	disp := darwin.NewDispatcher()
	rend := darwin.NewRenderer()

	// Create session
	sess := engine.NewSession(rend, disp)

	var tr transport.Transport

	args := flag.Args()
	if len(args) > 0 && *prompt == "" {
		// File mode: positional arg with no --prompt
		tr = transport.NewFileTransport(args[0])
	} else if *prompt != "" {
		// LLM mode
		provider, err := createProvider(*llmProvider, *apiKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		lt := transport.NewLLMTransport(transport.LLMConfig{
			Provider: provider,
			Model:    *model,
			Prompt:   *prompt,
			Mode:     *mode,
		})
		tr = lt

		// Wire actions from engine back to transport
		sess.OnAction = func(surfaceID string, action *protocol.Action, data map[string]interface{}) {
			lt.SendAction(surfaceID, action, data)
		}
	} else {
		fmt.Fprintf(os.Stderr, "usage: jview <file.jsonl>\n")
		fmt.Fprintf(os.Stderr, "       jview --prompt \"Build a todo app\"\n")
		fmt.Fprintf(os.Stderr, "       jview --llm openai --model gpt-4o --prompt \"Build a counter\"\n")
		os.Exit(1)
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
		return anthropic.New(opts...)
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
