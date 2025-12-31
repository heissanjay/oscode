package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/heissanjay/oscode/internal/config"
	"github.com/heissanjay/oscode/internal/llm"
)

// Session represents an active or saved session
type Session struct {
	ID                string        `json:"id"`
	Name              string        `json:"name,omitempty"`
	WorkingDir        string        `json:"working_dir"`
	Provider          string        `json:"provider"`
	Model             string        `json:"model"`
	Messages          []llm.Message `json:"messages"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	TotalInputTokens  int           `json:"total_input_tokens"`
	TotalOutputTokens int           `json:"total_output_tokens"`
	Checkpoints       []Checkpoint  `json:"checkpoints,omitempty"`
}

// Checkpoint represents a point in the session that can be restored
type Checkpoint struct {
	ID          string      `json:"id"`
	MessageIdx  int         `json:"message_idx"`
	Description string      `json:"description"`
	CreatedAt   time.Time   `json:"created_at"`
	FileState   []FileState `json:"file_state,omitempty"`
}

// FileState captures the state of a file at a checkpoint
type FileState struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Exists  bool   `json:"exists"`
}

// NewSession creates a new session
func NewSession(workingDir, provider, model string) *Session {
	now := time.Now()
	return &Session{
		ID:         uuid.New().String(),
		WorkingDir: workingDir,
		Provider:   provider,
		Model:      model,
		Messages:   make([]llm.Message, 0),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(msg llm.Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// UpdateTokens updates the token counts
func (s *Session) UpdateTokens(input, output int) {
	s.TotalInputTokens += input
	s.TotalOutputTokens += output
	s.UpdatedAt = time.Now()
}

// CreateCheckpoint creates a new checkpoint
func (s *Session) CreateCheckpoint(description string, files []string) *Checkpoint {
	cp := Checkpoint{
		ID:          uuid.New().String(),
		MessageIdx:  len(s.Messages),
		Description: description,
		CreatedAt:   time.Now(),
	}

	// Capture file states
	for _, path := range files {
		state := FileState{Path: path}
		content, err := os.ReadFile(path)
		if err != nil {
			state.Exists = false
		} else {
			state.Exists = true
			state.Content = string(content)
		}
		cp.FileState = append(cp.FileState, state)
	}

	s.Checkpoints = append(s.Checkpoints, cp)
	return &cp
}

// RewindTo rewinds the session to a checkpoint
func (s *Session) RewindTo(checkpointID string) error {
	var cp *Checkpoint
	for i, c := range s.Checkpoints {
		if c.ID == checkpointID {
			cp = &s.Checkpoints[i]
			break
		}
	}

	if cp == nil {
		return fmt.Errorf("checkpoint not found: %s", checkpointID)
	}

	// Truncate messages
	if cp.MessageIdx < len(s.Messages) {
		s.Messages = s.Messages[:cp.MessageIdx]
	}

	// Restore file states
	for _, state := range cp.FileState {
		if state.Exists {
			// Restore file content
			dir := filepath.Dir(state.Path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				continue
			}
			os.WriteFile(state.Path, []byte(state.Content), 0644)
		} else {
			// Delete file if it didn't exist
			os.Remove(state.Path)
		}
	}

	// Remove checkpoints after this one
	newCheckpoints := make([]Checkpoint, 0)
	for _, c := range s.Checkpoints {
		if c.CreatedAt.Before(cp.CreatedAt) || c.CreatedAt.Equal(cp.CreatedAt) {
			newCheckpoints = append(newCheckpoints, c)
		}
	}
	s.Checkpoints = newCheckpoints

	s.UpdatedAt = time.Now()
	return nil
}

// Manager handles session persistence
type Manager struct {
	sessionsDir string
	current     *Session
}

// NewManager creates a new session manager
func NewManager() *Manager {
	return &Manager{
		sessionsDir: config.GetSessionsDir(),
	}
}

// Create creates a new session
func (m *Manager) Create(workingDir, provider, model string) *Session {
	m.current = NewSession(workingDir, provider, model)
	return m.current
}

// Current returns the current session
func (m *Manager) Current() *Session {
	return m.current
}

// SetCurrent sets the current session
func (m *Manager) SetCurrent(s *Session) {
	m.current = s
}

// Save saves the current session
func (m *Manager) Save() error {
	if m.current == nil {
		return nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(m.sessionsDir, 0755); err != nil {
		return err
	}

	// Save session
	path := filepath.Join(m.sessionsDir, m.current.ID+".json")
	data, err := json.MarshalIndent(m.current, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Load loads a session by ID
func (m *Manager) Load(id string) (*Session, error) {
	path := filepath.Join(m.sessionsDir, id+".json")
	return m.loadFromPath(path)
}

// LoadByName loads a session by name
func (m *Manager) LoadByName(name string) (*Session, error) {
	sessions, err := m.List()
	if err != nil {
		return nil, err
	}

	for _, s := range sessions {
		if s.Name == name {
			return m.Load(s.ID)
		}
	}

	return nil, fmt.Errorf("session not found: %s", name)
}

// LoadLatest loads the most recent session
func (m *Manager) LoadLatest() (*Session, error) {
	sessions, err := m.List()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}

	// Sessions are sorted by UpdatedAt desc
	return m.Load(sessions[0].ID)
}

func (m *Manager) loadFromPath(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// List returns all saved sessions
func (m *Manager) List() ([]*Session, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, err
	}

	sessions := make([]*Session, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(m.sessionsDir, entry.Name())
		session, err := m.loadFromPath(path)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	// Sort by updated time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Delete deletes a session by ID
func (m *Manager) Delete(id string) error {
	path := filepath.Join(m.sessionsDir, id+".json")
	return os.Remove(path)
}

// Cleanup removes old sessions
func (m *Manager) Cleanup(maxAge time.Duration) error {
	sessions, err := m.List()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	for _, s := range sessions {
		if s.UpdatedAt.Before(cutoff) {
			m.Delete(s.ID)
		}
	}

	return nil
}

// Rename renames the current session
func (m *Manager) Rename(name string) error {
	if m.current == nil {
		return fmt.Errorf("no active session")
	}
	m.current.Name = name
	return m.Save()
}
