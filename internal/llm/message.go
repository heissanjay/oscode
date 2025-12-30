package llm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Role represents the role of a message sender
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message represents a chat message
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a block of content in a message
type ContentBlock struct {
	Type      ContentType `json:"type"`
	Text      string      `json:"text,omitempty"`
	Image     *ImageBlock `json:"image,omitempty"`
	ToolUse   *ToolUse    `json:"tool_use,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`
	Thinking  string      `json:"thinking,omitempty"`
}

// ContentType defines the type of content block
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeImage      ContentType = "image"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
	ContentTypeThinking   ContentType = "thinking"
)

// ImageBlock represents an image in a message
type ImageBlock struct {
	Type      string `json:"type"` // "base64" or "url"
	MediaType string `json:"media_type"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// NewTextMessage creates a new text message
func NewTextMessage(role Role, text string) Message {
	return Message{
		Role: role,
		Content: []ContentBlock{
			{
				Type: ContentTypeText,
				Text: text,
			},
		},
	}
}

// NewUserMessage creates a new user message
func NewUserMessage(text string) Message {
	return NewTextMessage(RoleUser, text)
}

// NewAssistantMessage creates a new assistant message
func NewAssistantMessage(text string) Message {
	return NewTextMessage(RoleAssistant, text)
}

// NewToolUseMessage creates a message containing tool use
func NewToolUseMessage(toolUse *ToolUse) Message {
	return Message{
		Role: RoleAssistant,
		Content: []ContentBlock{
			{
				Type:    ContentTypeToolUse,
				ToolUse: toolUse,
			},
		},
	}
}

// NewToolResultMessage creates a message containing tool result
func NewToolResultMessage(result *ToolResult) Message {
	return Message{
		Role: RoleUser,
		Content: []ContentBlock{
			{
				Type:       ContentTypeToolResult,
				ToolResult: result,
			},
		},
	}
}

// NewImageMessage creates a message with an image
func NewImageMessage(imagePath string) (Message, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return Message{}, fmt.Errorf("failed to read image: %w", err)
	}

	mediaType := getMediaType(imagePath)
	if mediaType == "" {
		return Message{}, fmt.Errorf("unsupported image format: %s", filepath.Ext(imagePath))
	}

	return Message{
		Role: RoleUser,
		Content: []ContentBlock{
			{
				Type: ContentTypeImage,
				Image: &ImageBlock{
					Type:      "base64",
					MediaType: mediaType,
					Data:      base64.StdEncoding.EncodeToString(data),
				},
			},
		},
	}, nil
}

// AddText adds text content to a message
func (m *Message) AddText(text string) {
	m.Content = append(m.Content, ContentBlock{
		Type: ContentTypeText,
		Text: text,
	})
}

// AddImage adds an image to a message
func (m *Message) AddImage(imagePath string) error {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	mediaType := getMediaType(imagePath)
	if mediaType == "" {
		return fmt.Errorf("unsupported image format: %s", filepath.Ext(imagePath))
	}

	m.Content = append(m.Content, ContentBlock{
		Type: ContentTypeImage,
		Image: &ImageBlock{
			Type:      "base64",
			MediaType: mediaType,
			Data:      base64.StdEncoding.EncodeToString(data),
		},
	})

	return nil
}

// AddToolUse adds a tool use to a message
func (m *Message) AddToolUse(toolUse *ToolUse) {
	m.Content = append(m.Content, ContentBlock{
		Type:    ContentTypeToolUse,
		ToolUse: toolUse,
	})
}

// AddToolResult adds a tool result to a message
func (m *Message) AddToolResult(result *ToolResult) {
	m.Content = append(m.Content, ContentBlock{
		Type:       ContentTypeToolResult,
		ToolResult: result,
	})
}

// GetText returns all text content from the message
func (m *Message) GetText() string {
	var texts []string
	for _, block := range m.Content {
		if block.Type == ContentTypeText {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// GetToolUses returns all tool uses from the message
func (m *Message) GetToolUses() []*ToolUse {
	var toolUses []*ToolUse
	for _, block := range m.Content {
		if block.Type == ContentTypeToolUse && block.ToolUse != nil {
			toolUses = append(toolUses, block.ToolUse)
		}
	}
	return toolUses
}

// HasToolUse returns true if the message contains tool uses
func (m *Message) HasToolUse() bool {
	for _, block := range m.Content {
		if block.Type == ContentTypeToolUse {
			return true
		}
	}
	return false
}

// ToJSON converts the message to JSON
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// getMediaType returns the MIME type for an image file
func getMediaType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

// Conversation represents a list of messages
type Conversation struct {
	Messages []Message
	System   string
}

// NewConversation creates a new conversation
func NewConversation() *Conversation {
	return &Conversation{
		Messages: make([]Message, 0),
	}
}

// AddMessage adds a message to the conversation
func (c *Conversation) AddMessage(msg Message) {
	c.Messages = append(c.Messages, msg)
}

// AddUserMessage adds a user message
func (c *Conversation) AddUserMessage(text string) {
	c.AddMessage(NewUserMessage(text))
}

// AddAssistantMessage adds an assistant message
func (c *Conversation) AddAssistantMessage(text string) {
	c.AddMessage(NewAssistantMessage(text))
}

// GetLastMessage returns the last message in the conversation
func (c *Conversation) GetLastMessage() *Message {
	if len(c.Messages) == 0 {
		return nil
	}
	return &c.Messages[len(c.Messages)-1]
}

// Clear removes all messages from the conversation
func (c *Conversation) Clear() {
	c.Messages = make([]Message, 0)
}

// Len returns the number of messages in the conversation
func (c *Conversation) Len() int {
	return len(c.Messages)
}
