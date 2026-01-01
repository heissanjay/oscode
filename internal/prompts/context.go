package prompts

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Context holds dynamic context information
type Context struct {
	WorkDir        string
	GitBranch      string
	GitStatus      string
	RecentCommits  string
	IsGitRepo      bool
	OscodeMD       string // Contents of OSCODE.md if present
	HasOscodeMD    bool
}

// GatherContext collects dynamic context from the environment
func GatherContext(workDir string) *Context {
	ctx := &Context{
		WorkDir: workDir,
	}

	// Check if it's a git repo
	gitDir := filepath.Join(workDir, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		ctx.IsGitRepo = true
		ctx.gatherGitInfo(workDir)
	}

	// Load OSCODE.md if present
	ctx.loadOscodeMD(workDir)

	return ctx
}

// loadOscodeMD loads the OSCODE.md project context file if it exists
func (c *Context) loadOscodeMD(workDir string) {
	// Try multiple locations: root, .oscode/, docs/
	locations := []string{
		filepath.Join(workDir, "OSCODE.md"),
		filepath.Join(workDir, ".oscode", "OSCODE.md"),
		filepath.Join(workDir, "docs", "OSCODE.md"),
	}

	for _, path := range locations {
		if content, err := os.ReadFile(path); err == nil {
			c.OscodeMD = string(content)
			c.HasOscodeMD = true
			return
		}
	}
}

func (c *Context) gatherGitInfo(workDir string) {
	// Get current branch
	c.GitBranch = runGitCommand(workDir, "rev-parse", "--abbrev-ref", "HEAD")

	// Get git status (abbreviated)
	status := runGitCommand(workDir, "status", "--short")
	if status != "" {
		lines := strings.Split(status, "\n")
		if len(lines) > 10 {
			lines = lines[:10]
			lines = append(lines, "... (truncated)")
		}
		c.GitStatus = strings.Join(lines, "\n")
	}

	// Get recent commits
	c.RecentCommits = runGitCommand(workDir, "log", "-5", "--oneline")
}

func runGitCommand(workDir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil // Ignore errors

	if err := cmd.Run(); err != nil {
		return ""
	}

	return strings.TrimSpace(stdout.String())
}

// GetMainBranch determines the main branch name (main or master)
func GetMainBranch(workDir string) string {
	// Check for main first
	if runGitCommand(workDir, "rev-parse", "--verify", "main") != "" {
		return "main"
	}
	// Fallback to master
	if runGitCommand(workDir, "rev-parse", "--verify", "master") != "" {
		return "master"
	}
	return "main"
}

// IsWorkingTreeClean checks if the git working tree is clean
func IsWorkingTreeClean(workDir string) bool {
	status := runGitCommand(workDir, "status", "--porcelain")
	return status == ""
}

// GetRemoteURL gets the origin remote URL
func GetRemoteURL(workDir string) string {
	return runGitCommand(workDir, "remote", "get-url", "origin")
}
