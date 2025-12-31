package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Transport defines the interface for MCP communication
type Transport interface {
	// Request sends a request and waits for response
	Request(request map[string]interface{}) (map[string]interface{}, error)
	// Notify sends a notification (no response expected)
	Notify(notification map[string]interface{}) error
	// Close closes the transport
	Close() error
}

// StdioTransport communicates with an MCP server via stdio
type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	scanner   *bufio.Scanner
	mu        sync.Mutex
	nextID    int64
	responses map[int64]chan map[string]interface{}
	respMu    sync.Mutex
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args []string, env map[string]string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}

	t := &StdioTransport{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		scanner:   bufio.NewScanner(stdout),
		responses: make(map[int64]chan map[string]interface{}),
	}

	// Start response reader
	go t.readResponses()

	return t, nil
}

func (t *StdioTransport) readResponses() {
	for t.scanner.Scan() {
		line := t.scanner.Text()
		if line == "" {
			continue
		}

		var response map[string]interface{}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			continue
		}

		// Get request ID
		idFloat, ok := response["id"].(float64)
		if !ok {
			continue
		}
		id := int64(idFloat)

		// Send to waiting request
		t.respMu.Lock()
		if ch, ok := t.responses[id]; ok {
			ch <- response
			delete(t.responses, id)
		}
		t.respMu.Unlock()
	}
}

func (t *StdioTransport) Request(request map[string]interface{}) (map[string]interface{}, error) {
	// Assign ID if not present
	id := atomic.AddInt64(&t.nextID, 1)
	request["id"] = id

	// Create response channel
	respCh := make(chan map[string]interface{}, 1)
	t.respMu.Lock()
	t.responses[id] = respCh
	t.respMu.Unlock()

	// Send request
	if err := t.send(request); err != nil {
		t.respMu.Lock()
		delete(t.responses, id)
		t.respMu.Unlock()
		return nil, err
	}

	// Wait for response
	response := <-respCh
	return response, nil
}

func (t *StdioTransport) Notify(notification map[string]interface{}) error {
	return t.send(notification)
}

func (t *StdioTransport) send(msg map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = t.stdin.Write(append(data, '\n'))
	return err
}

func (t *StdioTransport) Close() error {
	t.stdin.Close()
	t.stdout.Close()
	return t.cmd.Process.Kill()
}

// HTTPTransport communicates with an MCP server via HTTP
type HTTPTransport struct {
	url     string
	headers map[string]string
	client  *http.Client
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(url string, headers map[string]string) (*HTTPTransport, error) {
	return &HTTPTransport{
		url:     url,
		headers: headers,
		client:  &http.Client{},
	}, nil
}

func (t *HTTPTransport) Request(request map[string]interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", t.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response, nil
}

func (t *HTTPTransport) Notify(notification map[string]interface{}) error {
	_, err := t.Request(notification)
	return err
}

func (t *HTTPTransport) Close() error {
	return nil
}

// SSETransport communicates with an MCP server via Server-Sent Events
type SSETransport struct {
	url     string
	headers map[string]string
	client  *http.Client
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(url string, headers map[string]string) (*SSETransport, error) {
	return &SSETransport{
		url:     url,
		headers: headers,
		client:  &http.Client{},
	}, nil
}

func (t *SSETransport) Request(request map[string]interface{}) (map[string]interface{}, error) {
	// SSE is typically for receiving, use HTTP for sending
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", t.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response, nil
}

func (t *SSETransport) Notify(notification map[string]interface{}) error {
	_, err := t.Request(notification)
	return err
}

func (t *SSETransport) Close() error {
	return nil
}
