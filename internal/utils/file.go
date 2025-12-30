package utils

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileExists checks if a file exists
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// ReadFileLines reads a file and returns its lines
func ReadFileLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// WriteFileLines writes lines to a file
func WriteFileLines(path string, lines []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		writer.WriteString(line)
		writer.WriteString("\n")
	}
	return writer.Flush()
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// GetFileExtension returns the file extension without the dot
func GetFileExtension(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimPrefix(ext, ".")
}

// IsTextFile checks if a file appears to be a text file
func IsTextFile(path string) bool {
	textExtensions := map[string]bool{
		"txt": true, "md": true, "markdown": true,
		"go": true, "py": true, "js": true, "ts": true,
		"jsx": true, "tsx": true, "html": true, "css": true,
		"json": true, "yaml": true, "yml": true, "toml": true,
		"xml": true, "svg": true, "sh": true, "bash": true,
		"zsh": true, "fish": true, "ps1": true, "bat": true,
		"cmd": true, "rb": true, "php": true, "java": true,
		"kt": true, "scala": true, "rs": true, "c": true,
		"cpp": true, "h": true, "hpp": true, "cs": true,
		"swift": true, "m": true, "mm": true, "sql": true,
		"graphql": true, "proto": true, "make": true,
		"dockerfile": true, "gitignore": true, "env": true,
		"ini": true, "cfg": true, "conf": true, "log": true,
	}

	ext := strings.ToLower(GetFileExtension(path))
	return textExtensions[ext]
}

// IsImageFile checks if a file is an image
func IsImageFile(path string) bool {
	imageExtensions := map[string]bool{
		"png": true, "jpg": true, "jpeg": true,
		"gif": true, "webp": true, "svg": true,
		"bmp": true, "ico": true, "tiff": true,
	}

	ext := strings.ToLower(GetFileExtension(path))
	return imageExtensions[ext]
}

// GetRelativePath returns a path relative to the base directory
func GetRelativePath(path, base string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}

// NormalizePath normalizes a path for the current OS
func NormalizePath(path string) string {
	return filepath.Clean(path)
}

// ExpandHome expands ~ to the user's home directory
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// CountLines counts the number of lines in a file
func CountLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

// GetFileSize returns the size of a file in bytes
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
