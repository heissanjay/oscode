package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/heissanjay/oscode/internal/config"
	"github.com/heissanjay/oscode/internal/llm"
	"github.com/heissanjay/oscode/internal/tools"
)

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Server represents a connected MCP server
type Server struct {
	Name      string
	Config    config.MCPServerConfig
	Transport Transport
	Tools     []Tool
	Resources []Resource
	mu        sync.RWMutex
}

// Client manages connections to MCP servers
type Client struct {
	servers map[string]*Server
	mu      sync.RWMutex
}

// NewClient creates a new MCP client
func NewClient() *Client {
	return &Client{
		servers: make(map[string]*Server),
	}
}

// Connect connects to an MCP server
func (c *Client) Connect(name string, cfg config.MCPServerConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create transport based on type
	var transport Transport
	var err error

	switch cfg.Transport {
	case "stdio":
		transport, err = NewStdioTransport(cfg.Command, cfg.Args, cfg.Env)
	case "sse":
		transport, err = NewSSETransport(cfg.URL, cfg.Headers)
	case "http":
		transport, err = NewHTTPTransport(cfg.URL, cfg.Headers)
	default:
		return fmt.Errorf("unsupported MCP transport: %s", cfg.Transport)
	}

	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	server := &Server{
		Name:      name,
		Config:    cfg,
		Transport: transport,
	}

	// Initialize connection
	if err := server.initialize(); err != nil {
		transport.Close()
		return fmt.Errorf("failed to initialize server %s: %w", name, err)
	}

	// Discover tools
	if err := server.discoverTools(); err != nil {
		// Not fatal, just log warning
		fmt.Printf("Warning: failed to discover tools from %s: %v\n", name, err)
	}

	c.servers[name] = server
	return nil
}

// Disconnect disconnects from an MCP server
func (c *Client) Disconnect(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	server, ok := c.servers[name]
	if !ok {
		return fmt.Errorf("server not found: %s", name)
	}

	if err := server.Transport.Close(); err != nil {
		return err
	}

	delete(c.servers, name)
	return nil
}

// GetServer returns a server by name
func (c *Client) GetServer(name string) (*Server, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	server, ok := c.servers[name]
	return server, ok
}

// AllTools returns all tools from all connected servers
func (c *Client) AllTools() []tools.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []tools.Tool
	for _, server := range c.servers {
		for _, tool := range server.Tools {
			bridge := NewToolBridge(server, tool)
			result = append(result, bridge)
		}
	}
	return result
}

// ToLLMTools converts all MCP tools to LLM tool format
func (c *Client) ToLLMTools() []llm.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []llm.Tool
	for _, server := range c.servers {
		for _, tool := range server.Tools {
			result = append(result, llm.Tool{
				Name:        fmt.Sprintf("%s:%s", server.Name, tool.Name),
				Description: tool.Description,
				InputSchema: tool.InputSchema,
			})
		}
	}
	return result
}

// Close closes all server connections
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, server := range c.servers {
		server.Transport.Close()
		delete(c.servers, name)
	}
}

// Server methods

func (s *Server) initialize() error {
	// Send initialize request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "oscode",
				"version": "1.0.0",
			},
		},
	}

	response, err := s.Transport.Request(request)
	if err != nil {
		return err
	}

	// Check for error in response
	if errObj, ok := response["error"]; ok {
		return fmt.Errorf("initialization error: %v", errObj)
	}

	// Send initialized notification
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	return s.Transport.Notify(notification)
}

func (s *Server) discoverTools() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Request tools list
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	response, err := s.Transport.Request(request)
	if err != nil {
		return err
	}

	// Parse tools from response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}

	toolsList, ok := result["tools"].([]interface{})
	if !ok {
		return nil // No tools available
	}

	s.Tools = make([]Tool, 0, len(toolsList))
	for _, t := range toolsList {
		toolData, _ := json.Marshal(t)
		var tool Tool
		if err := json.Unmarshal(toolData, &tool); err == nil {
			s.Tools = append(s.Tools, tool)
		}
	}

	return nil
}

// CallTool calls a tool on this server
func (s *Server) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (interface{}, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
	}

	response, err := s.Transport.Request(request)
	if err != nil {
		return nil, err
	}

	// Check for error
	if errObj, ok := response["error"]; ok {
		return nil, fmt.Errorf("tool call error: %v", errObj)
	}

	return response["result"], nil
}
