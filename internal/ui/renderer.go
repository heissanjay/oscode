package ui

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

// OSCode Style - matches the exact color palette
var oscodeStyle = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#F5F5F4"),
		},
		Margin: uintPtr(0),
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#F5F5F4"),
			Bold:  boolPtr(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color:     stringPtr("#F5F5F4"),
			Bold:      boolPtr(true),
			Underline: boolPtr(true),
		},
		Margin: uintPtr(1),
	},
	H2: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#F5F5F4"),
			Bold:  boolPtr(true),
		},
		Margin: uintPtr(1),
	},
	H3: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#A8A29E"),
			Bold:  boolPtr(true),
		},
	},
	H4: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#A8A29E"),
			Bold:  boolPtr(true),
		},
	},
	Paragraph: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#F5F5F4"),
		},
	},
	Text: ansi.StylePrimitive{
		Color: stringPtr("#F5F5F4"),
	},
	Link: ansi.StylePrimitive{
		Color:     stringPtr("#60A5FA"),
		Underline: boolPtr(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: stringPtr("#60A5FA"),
		Bold:  boolPtr(true),
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color:           stringPtr("#60A5FA"),
			BackgroundColor: stringPtr("#292524"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			Margin: uintPtr(1),
		},
		Chroma: &ansi.Chroma{
			// Dracula-inspired theme
			Text:                ansi.StylePrimitive{Color: stringPtr("#F5F5F4")},
			Error:               ansi.StylePrimitive{Color: stringPtr("#DC2626")},
			Comment:             ansi.StylePrimitive{Color: stringPtr("#78716C"), Italic: boolPtr(true)},
			CommentPreproc:      ansi.StylePrimitive{Color: stringPtr("#C15F3C")},
			Keyword:             ansi.StylePrimitive{Color: stringPtr("#C15F3C"), Bold: boolPtr(true)},
			KeywordReserved:     ansi.StylePrimitive{Color: stringPtr("#C15F3C"), Bold: boolPtr(true)},
			KeywordNamespace:    ansi.StylePrimitive{Color: stringPtr("#C15F3C")},
			KeywordType:         ansi.StylePrimitive{Color: stringPtr("#60A5FA")},
			Operator:            ansi.StylePrimitive{Color: stringPtr("#F5F5F4")},
			Punctuation:         ansi.StylePrimitive{Color: stringPtr("#A8A29E")},
			Name:                ansi.StylePrimitive{Color: stringPtr("#F5F5F4")},
			NameBuiltin:         ansi.StylePrimitive{Color: stringPtr("#E07A3A")},
			NameTag:             ansi.StylePrimitive{Color: stringPtr("#C15F3C")},
			NameAttribute:       ansi.StylePrimitive{Color: stringPtr("#16A34A")},
			NameClass:           ansi.StylePrimitive{Color: stringPtr("#E07A3A"), Bold: boolPtr(true)},
			NameConstant:        ansi.StylePrimitive{Color: stringPtr("#E07A3A")},
			NameDecorator:       ansi.StylePrimitive{Color: stringPtr("#C15F3C")},
			NameException:       ansi.StylePrimitive{Color: stringPtr("#DC2626")},
			NameFunction:        ansi.StylePrimitive{Color: stringPtr("#16A34A")},
			NameOther:           ansi.StylePrimitive{Color: stringPtr("#F5F5F4")},
			Literal:             ansi.StylePrimitive{Color: stringPtr("#E07A3A")},
			LiteralNumber:       ansi.StylePrimitive{Color: stringPtr("#E07A3A")},
			LiteralString:       ansi.StylePrimitive{Color: stringPtr("#16A34A")},
			LiteralStringEscape: ansi.StylePrimitive{Color: stringPtr("#C15F3C")},
			GenericDeleted:      ansi.StylePrimitive{Color: stringPtr("#DC2626")},
			GenericEmph:         ansi.StylePrimitive{Italic: boolPtr(true)},
			GenericInserted:     ansi.StylePrimitive{Color: stringPtr("#16A34A")},
			GenericStrong:       ansi.StylePrimitive{Bold: boolPtr(true)},
			GenericSubheading:   ansi.StylePrimitive{Color: stringPtr("#60A5FA")},
			Background:          ansi.StylePrimitive{BackgroundColor: stringPtr("#1C1917")},
		},
	},
	List: ansi.StyleList{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#F5F5F4"),
			},
		},
		LevelIndent: 2,
	},
	Item: ansi.StylePrimitive{
		Color: stringPtr("#C15F3C"),
	},
	Emph: ansi.StylePrimitive{
		Italic: boolPtr(true),
	},
	Strong: ansi.StylePrimitive{
		Bold: boolPtr(true),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color:  stringPtr("#78716C"),
			Italic: boolPtr(true),
		},
		Indent: uintPtr(2),
	},
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#F5F5F4"),
			},
		},
		CenterSeparator: stringPtr("┼"),
		ColumnSeparator: stringPtr("│"),
		RowSeparator:    stringPtr("─"),
	},
	DefinitionDescription: ansi.StylePrimitive{
		Color: stringPtr("#A8A29E"),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color: stringPtr("#44403C"),
	},
}

// Cached renderer for performance
var (
	rendererCache     *glamour.TermRenderer
	rendererCacheLock sync.Mutex
	rendererWidth     int
)

// getRenderer returns a cached renderer or creates a new one
func getRenderer(width int) *glamour.TermRenderer {
	rendererCacheLock.Lock()
	defer rendererCacheLock.Unlock()

	// Return cached if width matches
	if rendererCache != nil && rendererWidth == width {
		return rendererCache
	}

	// Create new renderer with OSCode style
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(oscodeStyle),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		// Fallback to dark style
		r, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
	}

	rendererCache = r
	rendererWidth = width
	return r
}

// RenderMarkdown renders markdown text using glamour with OSCode styling
func RenderMarkdown(content string, width int) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	r := getRenderer(width)
	if r == nil {
		return content
	}

	out, err := r.Render(content)
	if err != nil {
		return content
	}

	// Trim trailing whitespace/newlines that glamour adds
	return strings.TrimRight(out, "\n\r\t ")
}

// RenderMarkdownStreaming renders markdown that may be incomplete (streaming)
// It handles partial code blocks gracefully
func RenderMarkdownStreaming(content string, width int) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	// Check for unclosed code blocks and close them temporarily
	processedContent := content
	codeBlockCount := strings.Count(content, "```")
	if codeBlockCount%2 != 0 {
		// Unclosed code block - close it for rendering
		processedContent = content + "\n```"
	}

	r := getRenderer(width)
	if r == nil {
		return content
	}

	out, err := r.Render(processedContent)
	if err != nil {
		return content
	}

	return strings.TrimRight(out, "\n\r\t ")
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func uintPtr(u uint) *uint {
	return &u
}
