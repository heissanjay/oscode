package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsGitRepo checks if the directory is inside a git repository
func IsGitRepo(dir string) bool {
	// Look for .git directory or file
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			// .git exists (could be directory or file for worktrees)
			return info.IsDir() || info.Mode().IsRegular()
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}

// GetGitRoot returns the root directory of the git repository
func GetGitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetGitBranch returns the current git branch name
func GetGitBranch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetGitStatus returns the git status summary
func GetGitStatus(dir string) (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func HasUncommittedChanges(dir string) bool {
	status, err := GetGitStatus(dir)
	if err != nil {
		return false
	}
	return status != ""
}

// GetGitRemoteURL returns the remote URL for the given remote name
func GetGitRemoteURL(dir, remote string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", remote)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetLastCommitHash returns the last commit hash
func GetLastCommitHash(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetLastCommitMessage returns the last commit message
func GetLastCommitMessage(dir string) (string, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GitAdd stages files for commit
func GitAdd(dir string, files ...string) error {
	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

// GitCommit creates a commit with the given message
func GitCommit(dir, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	return cmd.Run()
}

// GitDiff returns the diff of staged or unstaged changes
func GitDiff(dir string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// GetTrackedFiles returns a list of git-tracked files
func GetTrackedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// IsIgnored checks if a file is ignored by git
func IsIgnored(dir, path string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", path)
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}
