package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SelectionState represents the state for interactive selection
type SelectionState int

const (
	SelectionNone SelectionState = iota
	SelectionModelMenu
	SelectionProviderMenu
	SelectionHelpMenu
	SelectionPermissionsMenu
)

// SelectionItem represents an item in a selection list
type SelectionItem struct {
	ID          string
	Label       string
	Description string
	Selected    bool
}

// SelectionModel handles interactive selection UI
type SelectionModel struct {
	State       SelectionState
	Title       string
	Items       []SelectionItem
	Cursor      int
	Filter      string
	FilterMode  bool
	OnSelect    func(item SelectionItem)
	OnCancel    func()
}

// NewSelectionModel creates a new selection model
func NewSelectionModel() *SelectionModel {
	return &SelectionModel{
		State:  SelectionNone,
		Items:  make([]SelectionItem, 0),
		Cursor: 0,
	}
}

// Show displays the selection with given items
func (s *SelectionModel) Show(state SelectionState, title string, items []SelectionItem) {
	s.State = state
	s.Title = title
	s.Items = items
	s.Cursor = 0
	s.Filter = ""
	s.FilterMode = false

	// Find currently selected item and set cursor there
	for i, item := range items {
		if item.Selected {
			s.Cursor = i
			break
		}
	}
}

// Hide closes the selection
func (s *SelectionModel) Hide() {
	s.State = SelectionNone
	s.Items = nil
	s.Cursor = 0
	s.Filter = ""
}

// IsActive returns whether selection is active
func (s *SelectionModel) IsActive() bool {
	return s.State != SelectionNone
}

// MoveUp moves cursor up
func (s *SelectionModel) MoveUp() {
	if s.Cursor > 0 {
		s.Cursor--
	} else {
		s.Cursor = len(s.filteredItems()) - 1
	}
}

// MoveDown moves cursor down
func (s *SelectionModel) MoveDown() {
	filtered := s.filteredItems()
	if s.Cursor < len(filtered)-1 {
		s.Cursor++
	} else {
		s.Cursor = 0
	}
}

// Select selects the current item
func (s *SelectionModel) Select() *SelectionItem {
	filtered := s.filteredItems()
	if s.Cursor >= 0 && s.Cursor < len(filtered) {
		return &filtered[s.Cursor]
	}
	return nil
}

// AddFilterChar adds a character to the filter
func (s *SelectionModel) AddFilterChar(c rune) {
	s.Filter += string(c)
	s.Cursor = 0
}

// RemoveFilterChar removes the last filter character
func (s *SelectionModel) RemoveFilterChar() {
	if len(s.Filter) > 0 {
		s.Filter = s.Filter[:len(s.Filter)-1]
	}
}

// ClearFilter clears the filter
func (s *SelectionModel) ClearFilter() {
	s.Filter = ""
	s.Cursor = 0
}

func (s *SelectionModel) filteredItems() []SelectionItem {
	if s.Filter == "" {
		return s.Items
	}

	filter := strings.ToLower(s.Filter)
	var filtered []SelectionItem
	for _, item := range s.Items {
		if strings.Contains(strings.ToLower(item.Label), filter) ||
			strings.Contains(strings.ToLower(item.Description), filter) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// View renders the selection UI
func (s *SelectionModel) View(width int) string {
	if !s.IsActive() {
		return ""
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB")).
		Bold(true).
		MarginBottom(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111827")).
		Background(lipgloss.Color("#D97706")).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB")).
		Padding(0, 1)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)

	checkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)

	filterStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D97706"))

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(s.Title))
	b.WriteString("\n")

	// Filter indicator
	if s.Filter != "" {
		b.WriteString(filterStyle.Render("Filter: " + s.Filter))
		b.WriteString("\n\n")
	}

	// Items
	filtered := s.filteredItems()
	for i, item := range filtered {
		var line string

		// Selection indicator
		if i == s.Cursor {
			line = selectedStyle.Render("› " + item.Label)
		} else {
			prefix := "  "
			if item.Selected {
				prefix = checkStyle.Render("✓ ")
			}
			line = normalStyle.Render(prefix + item.Label)
		}

		// Description
		if item.Description != "" {
			line += "  " + descStyle.Render(item.Description)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(filtered) == 0 {
		b.WriteString(descStyle.Render("  No matches found"))
		b.WriteString("\n")
	}

	// Hints
	b.WriteString("\n")
	hints := []string{"↑↓ navigate", "enter select", "esc cancel"}
	if len(s.Items) > 5 {
		hints = append([]string{"type to filter"}, hints...)
	}
	b.WriteString(hintStyle.Render(strings.Join(hints, " • ")))

	// Wrap in a box
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#D97706")).
		Padding(1, 2).
		Width(width - 4).
		Render(b.String())
}
