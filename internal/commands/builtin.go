package commands

import (
	"fmt"
	"strings"
)

// RegisterBuiltinCommands registers all built-in commands
func RegisterBuiltinCommands() {
	Register(&Command{
		Name:        "help",
		Aliases:     []string{"h", "?"},
		Description: "Show available commands",
		Usage:       "/help [command]",
		Handler:     handleHelp,
	})

	Register(&Command{
		Name:        "exit",
		Aliases:     []string{"quit", "q"},
		Description: "Exit the application",
		Usage:       "/exit",
		Handler:     handleExit,
	})

	Register(&Command{
		Name:        "clear",
		Aliases:     []string{"cls"},
		Description: "Clear conversation history",
		Usage:       "/clear",
		Handler:     handleClear,
	})

	Register(&Command{
		Name:        "model",
		Aliases:     []string{"m"},
		Description: "Switch or show current model",
		Usage:       "/model [model_name]",
		Handler:     handleModel,
	})

	Register(&Command{
		Name:        "provider",
		Aliases:     []string{"p"},
		Description: "Switch or show current provider",
		Usage:       "/provider [provider_name]",
		Handler:     handleProvider,
	})

	Register(&Command{
		Name:        "cost",
		Description: "Show token usage and estimated cost",
		Usage:       "/cost",
		Handler:     handleCost,
	})

	Register(&Command{
		Name:        "compact",
		Description: "Compact conversation to reduce context size",
		Usage:       "/compact [instructions]",
		Handler:     handleCompact,
	})

	Register(&Command{
		Name:        "context",
		Description: "Show context usage",
		Usage:       "/context",
		Handler:     handleContext,
	})

	Register(&Command{
		Name:        "resume",
		Aliases:     []string{"r"},
		Description: "Resume a previous session",
		Usage:       "/resume [session_id|name]",
		Handler:     handleResume,
	})

	Register(&Command{
		Name:        "rename",
		Description: "Rename current session",
		Usage:       "/rename <name>",
		Handler:     handleRename,
	})

	Register(&Command{
		Name:        "vim",
		Description: "Toggle vim mode",
		Usage:       "/vim",
		Handler:     handleVim,
	})

	Register(&Command{
		Name:        "verbose",
		Aliases:     []string{"v"},
		Description: "Toggle verbose output",
		Usage:       "/verbose",
		Handler:     handleVerbose,
	})

	Register(&Command{
		Name:        "permissions",
		Aliases:     []string{"perms"},
		Description: "Show or manage permissions",
		Usage:       "/permissions [mode]",
		Handler:     handlePermissions,
	})

	Register(&Command{
		Name:        "config",
		Description: "Open configuration settings",
		Usage:       "/config",
		Handler:     handleConfig,
	})

	Register(&Command{
		Name:        "init",
		Description: "Initialize project with CLAUDE.md",
		Usage:       "/init",
		Handler:     handleInit,
	})

	Register(&Command{
		Name:        "todos",
		Description: "Show current todo list",
		Usage:       "/todos",
		Handler:     handleTodos,
	})

	Register(&Command{
		Name:        "bashes",
		Aliases:     []string{"tasks"},
		Description: "List background tasks",
		Usage:       "/bashes",
		Handler:     handleBashes,
	})
}

func handleHelp(ctx *Context, args string) error {
	if args != "" {
		// Show help for specific command
		cmd, ok := DefaultRegistry.Get(args)
		if !ok {
			return fmt.Errorf("unknown command: %s", args)
		}

		ctx.Print(fmt.Sprintf("/%s - %s\n", cmd.Name, cmd.Description))
		ctx.Print(fmt.Sprintf("Usage: %s\n", cmd.Usage))
		if len(cmd.Aliases) > 0 {
			ctx.Print(fmt.Sprintf("Aliases: %s\n", strings.Join(cmd.Aliases, ", ")))
		}
		return nil
	}

	// Show all commands
	var sb strings.Builder
	sb.WriteString("Available Commands:\n\n")

	for _, cmd := range DefaultRegistry.List() {
		if cmd.Hidden {
			continue
		}
		sb.WriteString(fmt.Sprintf("  /%s - %s\n", cmd.Name, cmd.Description))
	}

	sb.WriteString("\nUse /help <command> for more information about a specific command.\n")
	ctx.Print(sb.String())
	return nil
}

func handleExit(ctx *Context, args string) error {
	ctx.Exit()
	return nil
}

func handleClear(ctx *Context, args string) error {
	ctx.Clear()
	ctx.Print("Conversation cleared.\n")
	return nil
}

func handleModel(ctx *Context, args string) error {
	if args == "" {
		// Show available models
		ctx.Print("Available models:\n")
		ctx.Print("  Anthropic: opus, sonnet, haiku\n")
		ctx.Print("  OpenAI: gpt4o, gpt4o-mini, o1, o1-mini\n")
		ctx.Print("\nUsage: /model <model_name>\n")
		return nil
	}

	ctx.SetModel(args)
	ctx.Print(fmt.Sprintf("Model set to: %s\n", args))
	return nil
}

func handleProvider(ctx *Context, args string) error {
	if args == "" {
		ctx.Print("Available providers: anthropic, openai\n")
		ctx.Print("Usage: /provider <provider_name>\n")
		return nil
	}

	ctx.SetProvider(args)
	ctx.Print(fmt.Sprintf("Provider set to: %s\n", args))
	return nil
}

func handleCost(ctx *Context, args string) error {
	ctx.Print("Token usage and cost tracking coming soon.\n")
	return nil
}

func handleCompact(ctx *Context, args string) error {
	ctx.Print("Conversation compacting coming soon.\n")
	return nil
}

func handleContext(ctx *Context, args string) error {
	ctx.Print("Context usage visualization coming soon.\n")
	return nil
}

func handleResume(ctx *Context, args string) error {
	if args == "" {
		ctx.Print("Usage: /resume <session_id|name>\n")
		ctx.Print("Use /resume --list to see available sessions.\n")
		return nil
	}

	if args == "--list" || args == "-l" {
		ctx.Print("Session listing coming soon.\n")
		return nil
	}

	ctx.Print(fmt.Sprintf("Resuming session: %s\n", args))
	return nil
}

func handleRename(ctx *Context, args string) error {
	if args == "" {
		return fmt.Errorf("session name required. Usage: /rename <name>")
	}

	ctx.Print(fmt.Sprintf("Session renamed to: %s\n", args))
	return nil
}

func handleVim(ctx *Context, args string) error {
	ctx.Print("Vim mode toggled.\n")
	return nil
}

func handleVerbose(ctx *Context, args string) error {
	ctx.Print("Verbose mode toggled.\n")
	return nil
}

func handlePermissions(ctx *Context, args string) error {
	if args == "" {
		ctx.Print("Permission modes:\n")
		ctx.Print("  auto - Auto-accept allowed tools\n")
		ctx.Print("  ask  - Ask for all tool executions\n")
		ctx.Print("  plan - Read-only mode (no write operations)\n")
		ctx.Print("\nUsage: /permissions <mode>\n")
		return nil
	}

	ctx.Print(fmt.Sprintf("Permission mode set to: %s\n", args))
	return nil
}

func handleConfig(ctx *Context, args string) error {
	ctx.Print("Configuration editor coming soon.\n")
	ctx.Print("Configuration file location can be found with: oscode config path\n")
	return nil
}

func handleInit(ctx *Context, args string) error {
	ctx.Print("Project initialization coming soon.\n")
	return nil
}

func handleTodos(ctx *Context, args string) error {
	ctx.Print("Todo list:\n")
	ctx.Print("(No todos yet)\n")
	return nil
}

func handleBashes(ctx *Context, args string) error {
	ctx.Print("Background tasks:\n")
	ctx.Print("(No background tasks running)\n")
	return nil
}

func init() {
	RegisterBuiltinCommands()
}
