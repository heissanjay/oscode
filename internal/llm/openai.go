package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	client *openai.Client
	apiKey string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey string, baseURL string) *OpenAIProvider {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	client := openai.NewClientWithConfig(config)

	return &OpenAIProvider{
		client: client,
		apiKey: apiKey,
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Models() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo-preview",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"o1-preview",
		"o1-mini",
	}
}

func (p *OpenAIProvider) SupportsTools() bool {
	return true
}

func (p *OpenAIProvider) SupportsVision() bool {
	return true
}

func (p *OpenAIProvider) SupportsStreaming() bool {
	return true
}

func (p *OpenAIProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	messages := p.convertMessages(req.Messages, req.SystemPrompt)

	chatReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		chatReq.MaxTokens = req.MaxTokens
	}

	if req.Temperature > 0 {
		chatReq.Temperature = float32(req.Temperature)
	}

	if req.TopP > 0 {
		chatReq.TopP = float32(req.TopP)
	}

	if len(req.Tools) > 0 {
		chatReq.Tools = p.convertTools(req.Tools)
	}

	resp, err := p.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("openai API error: %w", err)
	}

	return p.convertResponse(&resp), nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	events := make(chan StreamEvent, 100)

	messages := p.convertMessages(req.Messages, req.SystemPrompt)

	chatReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
	}

	if req.MaxTokens > 0 {
		chatReq.MaxTokens = req.MaxTokens
	}

	if req.Temperature > 0 {
		chatReq.Temperature = float32(req.Temperature)
	}

	if len(req.Tools) > 0 {
		chatReq.Tools = p.convertTools(req.Tools)
	}

	go func() {
		defer close(events)

		stream, err := p.client.CreateChatCompletionStream(ctx, chatReq)
		if err != nil {
			events <- StreamEvent{
				Type:  EventTypeError,
				Error: err,
			}
			return
		}
		defer stream.Close()

		var toolCalls = make(map[int]*ToolUse)
		var totalOutputTokens int

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				events <- StreamEvent{
					Type:  EventTypeError,
					Error: err,
				}
				return
			}

			if len(resp.Choices) == 0 {
				continue
			}

			choice := resp.Choices[0]
			delta := choice.Delta

			// Handle text content
			if delta.Content != "" {
				events <- StreamEvent{
					Type:  EventTypeText,
					Delta: delta.Content,
				}
			}

			// Handle tool calls
			for _, toolCall := range delta.ToolCalls {
				idx := toolCall.Index
				if idx == nil {
					continue
				}

				if toolCalls[*idx] == nil {
					toolCalls[*idx] = &ToolUse{
						ID:    toolCall.ID,
						Name:  toolCall.Function.Name,
						Input: make(map[string]interface{}),
					}
					toolCalls[*idx].Input["_raw"] = ""
				}

				// Accumulate function arguments
				if toolCall.Function.Arguments != "" {
					// Append to raw JSON accumulator
					if raw, ok := toolCalls[*idx].Input["_raw"].(string); ok {
						toolCalls[*idx].Input["_raw"] = raw + toolCall.Function.Arguments
					} else {
						toolCalls[*idx].Input["_raw"] = toolCall.Function.Arguments
					}
				}
			}

			// Handle finish reason
			if choice.FinishReason == openai.FinishReasonToolCalls {
				for _, toolUse := range toolCalls {
					// Parse accumulated JSON arguments
					if rawJSON, ok := toolUse.Input["_raw"].(string); ok {
						var parsed map[string]interface{}
						if err := json.Unmarshal([]byte(rawJSON), &parsed); err == nil {
							toolUse.Input = parsed
						}
					}

					events <- StreamEvent{
						Type:    EventTypeToolUse,
						ToolUse: toolUse,
					}
				}
			}

			if choice.FinishReason != "" {
				events <- StreamEvent{
					Type: EventTypeDone,
					Response: &ChatResponse{
						ID:         resp.ID,
						Model:      resp.Model,
						StopReason: string(choice.FinishReason),
						Usage: Usage{
							OutputTokens: totalOutputTokens,
						},
					},
				}
			}
		}
	}()

	return events, nil
}

func (p *OpenAIProvider) convertMessages(messages []Message, systemPrompt string) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, 0, len(messages)+1)

	// Add system prompt as first message
	if systemPrompt != "" {
		result = append(result, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}

	for _, msg := range messages {
		var role string
		switch msg.Role {
		case RoleUser:
			role = openai.ChatMessageRoleUser
		case RoleAssistant:
			role = openai.ChatMessageRoleAssistant
		case RoleSystem:
			role = openai.ChatMessageRoleSystem
		default:
			role = openai.ChatMessageRoleUser
		}

		// Handle multi-part content
		var multiContent []openai.ChatMessagePart

		for _, content := range msg.Content {
			switch content.Type {
			case ContentTypeText:
				multiContent = append(multiContent, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: content.Text,
				})

			case ContentTypeImage:
				if content.Image != nil {
					var imageURL string
					if content.Image.Type == "base64" {
						imageURL = fmt.Sprintf("data:%s;base64,%s", content.Image.MediaType, content.Image.Data)
					} else {
						imageURL = content.Image.URL
					}
					multiContent = append(multiContent, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: imageURL,
						},
					})
				}

			case ContentTypeToolResult:
				if content.ToolResult != nil {
					result = append(result, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						Content:    content.ToolResult.Content,
						ToolCallID: content.ToolResult.ToolUseID,
					})
					continue
				}
			}
		}

		// Handle tool calls in assistant messages
		var toolCalls []openai.ToolCall
		for _, content := range msg.Content {
			if content.Type == ContentTypeToolUse && content.ToolUse != nil {
				inputJSON, _ := json.Marshal(content.ToolUse.Input)
				toolCalls = append(toolCalls, openai.ToolCall{
					ID:   content.ToolUse.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      content.ToolUse.Name,
						Arguments: string(inputJSON),
					},
				})
			}
		}

		if len(multiContent) > 0 {
			result = append(result, openai.ChatCompletionMessage{
				Role:         role,
				MultiContent: multiContent,
				ToolCalls:    toolCalls,
			})
		} else if len(toolCalls) > 0 {
			result = append(result, openai.ChatCompletionMessage{
				Role:      role,
				ToolCalls: toolCalls,
			})
		}
	}

	return result
}

func (p *OpenAIProvider) convertTools(tools []Tool) []openai.Tool {
	result := make([]openai.Tool, len(tools))

	for i, tool := range tools {
		// Convert the input schema to OpenAI format
		params := tool.InputSchema
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		result[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		}
	}

	return result
}

func (p *OpenAIProvider) convertResponse(resp *openai.ChatCompletionResponse) *ChatResponse {
	response := &ChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		response.StopReason = string(choice.FinishReason)

		if choice.Message.Content != "" {
			response.Content = append(response.Content, ContentBlock{
				Type: ContentTypeText,
				Text: choice.Message.Content,
			})
		}

		// Handle tool calls
		for _, toolCall := range choice.Message.ToolCalls {
			var input map[string]interface{}
			json.Unmarshal([]byte(toolCall.Function.Arguments), &input)

			toolUse := &ToolUse{
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: input,
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
