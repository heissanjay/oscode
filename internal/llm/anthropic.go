package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude
type AnthropicProvider struct {
	client *anthropic.Client
	apiKey string
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey string, baseURL string) *AnthropicProvider {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := anthropic.NewClient(opts...)

	return &AnthropicProvider{
		client: client,
		apiKey: apiKey,
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Models() []string {
	return []string{
		"claude-opus-4-20250514",
		"claude-sonnet-4-20250514",
		"claude-haiku-3-5-20241022",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
	}
}

func (p *AnthropicProvider) SupportsTools() bool {
	return true
}

func (p *AnthropicProvider) SupportsVision() bool {
	return true
}

func (p *AnthropicProvider) SupportsStreaming() bool {
	return true
}

func (p *AnthropicProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Convert messages
	messages := p.convertMessages(req.Messages)

	// Build request params
	params := anthropic.MessageNewParams{
		Model:     anthropic.F(anthropic.Model(req.Model)),
		Messages:  anthropic.F(messages),
		MaxTokens: anthropic.F(int64(req.MaxTokens)),
	}

	if req.SystemPrompt != "" {
		params.System = anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(req.SystemPrompt),
		})
	}

	if req.Temperature > 0 {
		params.Temperature = anthropic.F(req.Temperature)
	}

	if req.TopP > 0 {
		params.TopP = anthropic.F(req.TopP)
	}

	if len(req.Tools) > 0 {
		tools := p.convertTools(req.Tools)
		params.Tools = anthropic.F(tools)
	}

	// Make request
	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	return p.convertResponse(resp), nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	events := make(chan StreamEvent, 100)

	// Convert messages
	messages := p.convertMessages(req.Messages)

	// Build request params
	params := anthropic.MessageNewParams{
		Model:     anthropic.F(anthropic.Model(req.Model)),
		Messages:  anthropic.F(messages),
		MaxTokens: anthropic.F(int64(req.MaxTokens)),
	}

	if req.SystemPrompt != "" {
		params.System = anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(req.SystemPrompt),
		})
	}

	if req.Temperature > 0 {
		params.Temperature = anthropic.F(req.Temperature)
	}

	if len(req.Tools) > 0 {
		tools := p.convertTools(req.Tools)
		params.Tools = anthropic.F(tools)
	}

	go func() {
		defer close(events)

		// Use actual streaming
		stream := p.client.Messages.NewStreaming(ctx, params)

		var currentToolUse *ToolUse
		var toolInputJSON string
		var response *ChatResponse

		for stream.Next() {
			event := stream.Current()

			switch evt := event.AsUnion().(type) {
			case anthropic.ContentBlockStartEvent:
				if evt.ContentBlock.Type == "tool_use" {
					currentToolUse = &ToolUse{
						ID:   evt.ContentBlock.ID,
						Name: evt.ContentBlock.Name,
					}
					toolInputJSON = ""
				}

			case anthropic.ContentBlockDeltaEvent:
				switch delta := evt.Delta.AsUnion().(type) {
				case anthropic.TextDelta:
					events <- StreamEvent{
						Type:  EventTypeText,
						Delta: delta.Text,
					}
				case anthropic.InputJSONDelta:
					toolInputJSON += delta.PartialJSON
				}

			case anthropic.ContentBlockStopEvent:
				if currentToolUse != nil {
					// Parse the accumulated JSON
					var input map[string]interface{}
					if err := json.Unmarshal([]byte(toolInputJSON), &input); err == nil {
						currentToolUse.Input = input
					}
					events <- StreamEvent{
						Type:    EventTypeToolUse,
						ToolUse: currentToolUse,
					}
					currentToolUse = nil
					toolInputJSON = ""
				}

			case anthropic.MessageStopEvent:
				// Build final response
				msg := stream.Current().Message
				response = &ChatResponse{
					ID:         msg.ID,
					Model:      string(msg.Model),
					StopReason: string(msg.StopReason),
					Usage: Usage{
						InputTokens:  int(msg.Usage.InputTokens),
						OutputTokens: int(msg.Usage.OutputTokens),
						TotalTokens:  int(msg.Usage.InputTokens + msg.Usage.OutputTokens),
					},
				}
			}
		}

		if err := stream.Err(); err != nil {
			events <- StreamEvent{
				Type:  EventTypeError,
				Error: err,
			}
			return
		}

		// Send done event
		events <- StreamEvent{
			Type:     EventTypeDone,
			Response: response,
		}
	}()

	return events, nil
}

func (p *AnthropicProvider) convertMessages(messages []Message) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, 0, len(messages))

	for _, msg := range messages {
		var blocks []anthropic.ContentBlockParamUnion

		for _, content := range msg.Content {
			switch content.Type {
			case ContentTypeText:
				blocks = append(blocks, anthropic.NewTextBlock(content.Text))

			case ContentTypeImage:
				if content.Image != nil && content.Image.Type == "base64" {
					blocks = append(blocks, anthropic.NewImageBlockBase64(
						content.Image.MediaType,
						content.Image.Data,
					))
				}

			case ContentTypeToolResult:
				if content.ToolResult != nil {
					blocks = append(blocks, anthropic.NewToolResultBlock(
						content.ToolResult.ToolUseID,
						content.ToolResult.Content,
						content.ToolResult.IsError,
					))
				}
			}
		}

		// Handle tool use blocks in assistant messages
		for _, content := range msg.Content {
			if content.Type == ContentTypeToolUse && content.ToolUse != nil {
				inputJSON, _ := json.Marshal(content.ToolUse.Input)
				var inputAny interface{}
				json.Unmarshal(inputJSON, &inputAny)

				blocks = append(blocks, anthropic.NewToolUseBlockParam(
					content.ToolUse.ID,
					content.ToolUse.Name,
					inputAny,
				))
			}
		}

		if len(blocks) > 0 {
			var role anthropic.MessageParamRole
			switch msg.Role {
			case RoleUser:
				role = anthropic.MessageParamRoleUser
			case RoleAssistant:
				role = anthropic.MessageParamRoleAssistant
			default:
				role = anthropic.MessageParamRoleUser
			}

			result = append(result, anthropic.MessageParam{
				Role:    anthropic.F(role),
				Content: anthropic.F(blocks),
			})
		}
	}

	return result
}

func (p *AnthropicProvider) convertTools(tools []Tool) []anthropic.ToolUnionUnionParam {
	result := make([]anthropic.ToolUnionUnionParam, len(tools))

	for i, tool := range tools {
		// Build JSON Schema as a map
		schema := map[string]interface{}{
			"type":       "object",
			"properties": tool.InputSchema["properties"],
		}
		if required, ok := tool.InputSchema["required"]; ok {
			schema["required"] = required
		}

		result[i] = anthropic.ToolParam{
			Name:        anthropic.F(tool.Name),
			Description: anthropic.F(tool.Description),
			InputSchema: anthropic.F[interface{}](schema),
		}
	}

	return result
}

func (p *AnthropicProvider) convertResponse(resp *anthropic.Message) *ChatResponse {
	response := &ChatResponse{
		ID:         resp.ID,
		Model:      string(resp.Model),
		StopReason: string(resp.StopReason),
		Usage: Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
			TotalTokens:  int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}

	for _, block := range resp.Content {
		switch block.Type {
		case anthropic.ContentBlockTypeText:
			response.Content = append(response.Content, ContentBlock{
				Type: ContentTypeText,
				Text: block.Text,
			})
		case anthropic.ContentBlockTypeToolUse:
			toolUse := &ToolUse{
				ID:   block.ID,
				Name: block.Name,
			}
			// Parse the tool input from RawMessage
			var inputMap map[string]interface{}
			if err := json.Unmarshal(block.Input, &inputMap); err == nil {
				toolUse.Input = inputMap
			}
			response.ToolUse = append(response.ToolUse, *toolUse)
			response.Content = append(response.Content, ContentBlock{
				Type:    ContentTypeToolUse,
				ToolUse: toolUse,
			})
		}
	}

	return response
}
