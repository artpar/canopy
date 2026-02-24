package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/mozilla-ai/any-llm-go/providers"
)

const (
	anthropicDefaultMaxTokens = 4096
	anthropicEnvAPIKey        = "ANTHROPIC_API_KEY"
)

// AnthropicProvider is a custom Anthropic provider that enables prompt caching
// via cache_control on system blocks and the last tool definition.
type AnthropicProvider struct {
	client *anthropic.Client
}

// NewAnthropicProvider creates an Anthropic provider with prompt caching support.
// If apiKey is empty, falls back to ANTHROPIC_API_KEY env var.
func NewAnthropicProvider(apiKey string) (*AnthropicProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv(anthropicEnvAPIKey)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic: API key required (set --api-key or %s)", anthropicEnvAPIKey)
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{client: &client}, nil
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) Completion(
	ctx context.Context,
	params providers.CompletionParams,
) (*providers.ChatCompletion, error) {
	req := p.convertParams(params)

	resp, err := p.client.Messages.New(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}

	return p.convertResponse(resp), nil
}

func (p *AnthropicProvider) CompletionStream(
	ctx context.Context,
	params providers.CompletionParams,
) (<-chan providers.ChatCompletionChunk, <-chan error) {
	chunks := make(chan providers.ChatCompletionChunk)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		req := p.convertParams(params)
		stream := p.client.Messages.NewStreaming(ctx, req)

		var (
			messageID      string
			model          string
			inputUsage     int64
			toolCalls      []providers.ToolCall
			currentToolIdx = -1
		)

		mkChunk := func(delta providers.ChunkDelta) providers.ChatCompletionChunk {
			return providers.ChatCompletionChunk{
				ID:     messageID,
				Object: "chat.completion.chunk",
				Model:  model,
				Choices: []providers.ChunkChoice{{
					Index: 0,
					Delta: delta,
				}},
			}
		}

		for stream.Next() {
			event := stream.Current()

			switch event.Type {
			case "message_start":
				msg := event.AsMessageStart()
				messageID = msg.Message.ID
				model = string(msg.Message.Model)
				inputUsage = msg.Message.Usage.InputTokens
				chunks <- mkChunk(providers.ChunkDelta{Role: providers.RoleAssistant})

			case "content_block_start":
				block := event.AsContentBlockStart()
				if block.ContentBlock.Type == "tool_use" {
					currentToolIdx++
					toolCalls = append(toolCalls, providers.ToolCall{
						ID:   block.ContentBlock.ID,
						Type: "function",
						Function: providers.FunctionCall{
							Name: block.ContentBlock.Name,
						},
					})
				}

			case "content_block_delta":
				delta := event.AsContentBlockDelta()
				switch delta.Delta.Type {
				case "text_delta":
					chunks <- mkChunk(providers.ChunkDelta{Content: delta.Delta.Text})
				case "input_json_delta":
					if currentToolIdx >= 0 && currentToolIdx < len(toolCalls) {
						toolCalls[currentToolIdx].Function.Arguments += delta.Delta.PartialJSON
						chunks <- mkChunk(providers.ChunkDelta{
							ToolCalls: []providers.ToolCall{toolCalls[currentToolIdx]},
						})
					}
				}

			case "message_delta":
				md := event.AsMessageDelta()
				finishReason := convertAnthropicStopReason(string(md.Delta.StopReason))
				c := mkChunk(providers.ChunkDelta{})
				c.Choices[0].FinishReason = finishReason
				c.Usage = &providers.Usage{
					PromptTokens:     int(inputUsage),
					CompletionTokens: int(md.Usage.OutputTokens),
					TotalTokens:      int(inputUsage + md.Usage.OutputTokens),
				}
				chunks <- c
			}
		}

		if err := stream.Err(); err != nil {
			errs <- fmt.Errorf("anthropic stream: %w", err)
		}
	}()

	return chunks, errs
}

// convertParams builds Anthropic MessageNewParams with cache_control on system blocks.
func (p *AnthropicProvider) convertParams(params providers.CompletionParams) anthropic.MessageNewParams {
	messages, system := convertAnthropicMessages(params.Messages)

	maxTokens := int64(anthropicDefaultMaxTokens)
	if params.MaxTokens != nil {
		maxTokens = int64(*params.MaxTokens)
	}

	req := anthropic.MessageNewParams{
		Model:     anthropic.Model(params.Model),
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	// Use automatic caching: the API places a cache breakpoint on the last
	// cacheable block and moves it forward as conversations grow.
	req.SetExtraFields(map[string]any{
		"cache_control": map[string]string{"type": "ephemeral"},
	})

	if system != "" {
		req.System = []anthropic.TextBlockParam{
			{Text: system},
		}
	}

	if params.Temperature != nil {
		req.Temperature = anthropic.Float(*params.Temperature)
	}

	if params.TopP != nil {
		req.TopP = anthropic.Float(*params.TopP)
	}

	if len(params.Stop) > 0 {
		req.StopSequences = params.Stop
	}

	if len(params.Tools) > 0 {
		tools := make([]anthropic.ToolUnionParam, 0, len(params.Tools))
		for _, tool := range params.Tools {
			tools = append(tools, convertAnthropicTool(tool))
		}
		req.Tools = tools
	}

	return req
}

// convertAnthropicMessages extracts the system message and converts the rest.
func convertAnthropicMessages(messages []providers.Message) ([]anthropic.MessageParam, string) {
	result := make([]anthropic.MessageParam, 0, len(messages))
	var systemParts []string

	for _, msg := range messages {
		if msg.Role == providers.RoleSystem {
			systemParts = append(systemParts, msg.ContentString())
			continue
		}

		switch msg.Role {
		case providers.RoleUser:
			m := anthropic.NewUserMessage(anthropic.NewTextBlock(msg.ContentString()))
			result = append(result, m)

		case providers.RoleAssistant:
			if len(msg.ToolCalls) == 0 {
				m := anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.ContentString()))
				result = append(result, m)
			} else {
				content := make([]anthropic.ContentBlockParamUnion, 0)
				if msg.ContentString() != "" {
					content = append(content, anthropic.NewTextBlock(msg.ContentString()))
				}
				for _, tc := range msg.ToolCalls {
					var input map[string]any
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
					content = append(content, anthropic.ContentBlockParamUnion{
						OfToolUse: &anthropic.ToolUseBlockParam{
							Type:  "tool_use",
							ID:    tc.ID,
							Name:  tc.Function.Name,
							Input: input,
						},
					})
				}
				m := anthropic.NewAssistantMessage(content...)
				result = append(result, m)
			}

		case providers.RoleTool:
			m := anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(msg.ToolCallID, msg.ContentString(), false),
			)
			result = append(result, m)
		}
	}

	return result, strings.Join(systemParts, "\n")
}

// convertAnthropicTool converts a providers.Tool to Anthropic ToolUnionParam.
func convertAnthropicTool(tool providers.Tool) anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{Type: "object"}

	if tool.Function.Parameters != nil {
		if props, ok := tool.Function.Parameters["properties"]; ok {
			schema.Properties = props
		}
		if req, ok := tool.Function.Parameters["required"]; ok {
			if strs, err := toAnthropicStringSlice(req); err == nil {
				schema.Required = strs
			}
		}
	}

	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        tool.Function.Name,
			Description: anthropic.String(tool.Function.Description),
			InputSchema: schema,
		},
	}
}

// convertResponse converts an Anthropic Message to providers.ChatCompletion.
func (p *AnthropicProvider) convertResponse(resp *anthropic.Message) *providers.ChatCompletion {
	var content string
	var toolCalls []providers.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			inputJSON := ""
			if block.Input != nil {
				if b, err := json.Marshal(block.Input); err == nil {
					inputJSON = string(b)
				}
			}
			toolCalls = append(toolCalls, providers.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: providers.FunctionCall{
					Name:      block.Name,
					Arguments: inputJSON,
				},
			})
		}
	}

	finishReason := convertAnthropicStopReason(string(resp.StopReason))

	usage := &providers.Usage{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
	}

	log.Printf("anthropic usage: input=%d output=%d cache_create=%d cache_read=%d",
		resp.Usage.InputTokens, resp.Usage.OutputTokens,
		resp.Usage.CacheCreationInputTokens, resp.Usage.CacheReadInputTokens)

	return &providers.ChatCompletion{
		ID:     resp.ID,
		Object: "chat.completion",
		Model:  string(resp.Model),
		Choices: []providers.Choice{{
			Index: 0,
			Message: providers.Message{
				Role:      providers.RoleAssistant,
				Content:   content,
				ToolCalls: toolCalls,
			},
			FinishReason: finishReason,
		}},
		Usage: usage,
	}
}

func convertAnthropicStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return providers.FinishReasonStop
	case "max_tokens":
		return providers.FinishReasonLength
	case "tool_use":
		return providers.FinishReasonToolCalls
	case "stop_sequence":
		return providers.FinishReasonStop
	default:
		return providers.FinishReasonStop
	}
}

func toAnthropicStringSlice(v any) ([]string, error) {
	switch typed := v.(type) {
	case []string:
		return typed, nil
	case []any:
		result := make([]string, len(typed))
		for i, elem := range typed {
			s, ok := elem.(string)
			if !ok {
				return nil, fmt.Errorf("element %d: expected string, got %T", i, elem)
			}
			result[i] = s
		}
		return result, nil
	default:
		return nil, fmt.Errorf("expected []string or []any, got %T", v)
	}
}
