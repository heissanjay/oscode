# OSCode - AI-Powered CLI Coding Agent

OSCode is a production-grade CLI coding agent tool inspired by Claude Code. It provides an interactive AI assistant for software development tasks with support for multiple LLM providers.

## Features

- **Multi-Provider LLM Support**: Switch between Anthropic (Claude) and OpenAI seamlessly
- **Rich Terminal UI**: Interactive interface with streaming responses, syntax highlighting, and vim mode
- **Comprehensive Tool System**:
  - **File Operations**: Read, Write, Edit with automatic permission management
  - **Bash Execution**: Run shell commands with background task support
  - **Search Tools**: Glob patterns and regex search (ripgrep-powered)
  - **Web Tools**: Fetch and search web content
- **Permission System**: Configurable allow/ask/deny rules for tool execution
- **Session Management**: Save, resume, and name your sessions
- **CLAUDE.md Memory**: Project-level and user-level memory files
- **Slash Commands**: Built-in commands for quick actions
- **Hooks System**: Customize behavior with pre/post tool hooks

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/oscode-cli/oscode.git
cd oscode

# Build
make build

# Install globally
make install
```

### Using Go Install

```bash
go install github.com/oscode-cli/oscode/cmd/oscode@latest
```

## Quick Start

1. **Set up your API key**:
   ```bash
   # For Anthropic (Claude)
   export ANTHROPIC_API_KEY=your-api-key

   # For OpenAI
   export OPENAI_API_KEY=your-api-key
   ```

2. **Run OSCode**:
   ```bash
   oscode
   ```

3. **Start with a prompt**:
   ```bash
   oscode "explain this codebase"
   ```

## Usage

### Interactive Mode

```bash
oscode                    # Start interactive session
oscode -c                 # Continue last session
oscode -r my-session      # Resume named session
```

### Print Mode (Non-Interactive)

```bash
oscode -p "fix the bug"                    # Single query
oscode -p "query" --output-format json     # JSON output
cat file.py | oscode -p "review this"      # Pipe input
```

### Command Line Options

```
--provider, -P     LLM provider (anthropic, openai)
--model, -m        Model to use (opus, sonnet, haiku, gpt4o, etc.)
--print, -p        Print mode (non-interactive)
--continue, -c     Continue last conversation
--resume, -r       Resume session by ID or name
--verbose          Show detailed output
--system-prompt    Custom system prompt
--output-format    Output format (text, json, stream-json)
--permission-mode  Permission mode (auto, ask, plan)
```

## Slash Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/clear` | Clear conversation history |
| `/model` | Switch or show current model |
| `/provider` | Switch or show current provider |
| `/exit` | Exit the application |
| `/compact` | Compact conversation |
| `/cost` | Show token usage |
| `/resume` | Resume a session |
| `/rename` | Rename current session |
| `/vim` | Toggle vim mode |
| `/permissions` | Manage permissions |

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Submit message |
| `Ctrl+J` | Insert newline |
| `Ctrl+C` | Cancel/Quit |
| `Ctrl+D` | Exit (if empty) |
| `Ctrl+L` | Clear screen |
| `Ctrl+O` | Toggle verbose |
| `Up/Down` | History navigation |
| `PgUp/PgDn` | Scroll messages |
| `Esc` | Enter vim normal mode |

## Configuration

Configuration files are loaded in order of precedence:

1. `~/.oscode/settings.json` - User settings
2. `.oscode/settings.json` - Project settings (shared)
3. `.oscode/settings.local.json` - Project local settings

### Example Configuration

```json
{
  "defaultProvider": "anthropic",
  "defaultModel": "claude-sonnet-4-20250514",
  "providers": {
    "anthropic": {
      "apiKey": "${ANTHROPIC_API_KEY}"
    },
    "openai": {
      "apiKey": "${OPENAI_API_KEY}"
    }
  },
  "permissions": {
    "allow": ["Read", "Glob", "Grep"],
    "ask": ["Bash", "Write", "Edit"],
    "deny": ["Read(.env*)"]
  },
  "ui": {
    "theme": "dark",
    "showTokenCount": true
  }
}
```

## Project Memory (CLAUDE.md)

Create a `CLAUDE.md` file in your project root to provide context:

```markdown
# Project: My App

## Overview
This is a web application built with...

## Code Style
- Use TypeScript
- Follow ESLint rules
- Write tests for new features

## Important Files
- src/index.ts - Entry point
- src/config.ts - Configuration
```

## Tools

### File Tools

- **Read**: Read files with line numbers, supports images and PDFs
- **Write**: Create or overwrite files
- **Edit**: Make targeted string replacements

### Execution Tools

- **Bash**: Execute shell commands with timeout support
- **BashOutput**: Get output from background tasks

### Search Tools

- **Glob**: Find files by pattern (e.g., `**/*.ts`)
- **Grep**: Search file contents with regex

### Web Tools

- **WebFetch**: Fetch and process web pages
- **WebSearch**: Search the web (requires API config)

## Development

```bash
# Run tests
make test

# Run with live reload
make dev

# Build for all platforms
make build-all

# Lint code
make lint
```

## Architecture

```
oscode/
├── cmd/oscode/         # CLI entry point
├── internal/
│   ├── app/            # Application core
│   ├── config/         # Configuration management
│   ├── llm/            # LLM provider abstraction
│   ├── tools/          # Tool implementations
│   ├── permissions/    # Permission system
│   ├── session/        # Session management
│   ├── commands/       # Slash commands
│   ├── ui/             # Terminal UI (Bubble Tea)
│   └── utils/          # Utility functions
└── pkg/types/          # Shared types
```

## License

MIT License - See LICENSE file for details.

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.
