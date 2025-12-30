package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// WebFetchInput defines the input for the WebFetch tool
type WebFetchInput struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

// WebFetchTool fetches and processes web content
type WebFetchTool struct {
	BaseTool
	client *http.Client
}

// NewWebFetchTool creates a new WebFetch tool
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		BaseTool: NewBaseTool(
			"WebFetch",
			"Fetches content from a URL and processes it. Returns the page content as markdown.",
			BuildSchema(map[string]interface{}{
				"url":    StringProperty("The URL to fetch content from", true),
				"prompt": StringProperty("What information to extract from the page", true),
			}, []string{"url", "prompt"}),
			true, // Requires permission
			CategoryWeb,
		),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params WebFetchInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.URL == "" {
		return NewErrorResultString("url is required"), nil
	}

	// Ensure HTTPS
	url := params.URL
	if strings.HasPrefix(url, "http://") {
		url = "https://" + strings.TrimPrefix(url, "http://")
	}
	if !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create request: %w", err)), nil
	}

	req.Header.Set("User-Agent", "OSCode/1.0 (CLI Agent)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Make request
	resp, err := t.client.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to fetch URL: %w", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return NewErrorResultString(fmt.Sprintf("HTTP error: %d %s", resp.StatusCode, resp.Status)), nil
	}

	// Read body
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB limit
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to read response: %w", err)), nil
	}

	// Convert HTML to text
	content := extractText(string(body))

	// Truncate if too long
	if len(content) > 50000 {
		content = content[:50000] + "\n\n... (content truncated)"
	}

	result := NewResult(content)
	result.WithMetadata("url", url)
	result.WithMetadata("content_type", resp.Header.Get("Content-Type"))

	return result, nil
}

// extractText extracts text content from HTML
func extractText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	var sb strings.Builder
	var extract func(*html.Node)

	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}

		// Skip script and style tags
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript":
				return
			case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li":
				sb.WriteString("\n")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return strings.TrimSpace(sb.String())
}

// WebSearchInput defines the input for the WebSearch tool
type WebSearchInput struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
}

// WebSearchTool performs web searches
type WebSearchTool struct {
	BaseTool
}

// NewWebSearchTool creates a new WebSearch tool
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		BaseTool: NewBaseTool(
			"WebSearch",
			"Search the web for information. Returns search results with titles, URLs, and snippets.",
			BuildSchema(map[string]interface{}{
				"query":           StringProperty("The search query", true),
				"allowed_domains": ArrayProperty("Only include results from these domains", "string"),
				"blocked_domains": ArrayProperty("Exclude results from these domains", "string"),
			}, []string{"query"}),
			true, // Requires permission
			CategoryWeb,
		),
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params WebSearchInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.Query == "" {
		return NewErrorResultString("query is required"), nil
	}

	// Note: In a production implementation, this would integrate with a search API
	// (Google Custom Search, Bing, etc.)
	// For now, return a placeholder

	return NewResult("Web search functionality requires API configuration. Please configure a search API in settings."), nil
}
