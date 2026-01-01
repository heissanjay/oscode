package main

import (
	"fmt"
	"os"

	"github.com/heissanjay/oscode/internal/app"
	"github.com/heissanjay/oscode/internal/config"
	"github.com/heissanjay/oscode/internal/setup"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "oscode [prompt]",
		Short: "OSCode - AI-powered coding assistant",
		Long: `OSCode is a production-grade CLI coding agent tool that provides
an interactive AI assistant for software development tasks.

Supports multiple LLM providers including Anthropic (Claude) and OpenAI.`,
		Version: fmt.Sprintf("%s (built %s)", Version, BuildTime),
		RunE:    runApp,
		Args:    cobra.MaximumNArgs(1),
	}

	// Global flags
	rootCmd.PersistentFlags().StringP("provider", "P", "", "LLM provider (anthropic, openai)")
	rootCmd.PersistentFlags().StringP("model", "m", "", "Model to use")
	rootCmd.PersistentFlags().BoolP("print", "p", false, "Print mode (non-interactive)")
	rootCmd.PersistentFlags().BoolP("continue", "c", false, "Continue last conversation")
	rootCmd.PersistentFlags().StringP("resume", "r", "", "Resume session by ID or name")
	rootCmd.PersistentFlags().Bool("verbose", false, "Show verbose output")
	rootCmd.PersistentFlags().String("system-prompt", "", "Custom system prompt")
	rootCmd.PersistentFlags().String("output-format", "text", "Output format (text, json, stream-json)")
	rootCmd.PersistentFlags().Int("max-turns", 0, "Maximum agentic turns (0 = unlimited)")
	rootCmd.PersistentFlags().String("permission-mode", "", "Permission mode (auto, ask, plan)")
	rootCmd.PersistentFlags().StringSlice("tools", nil, "Enabled tools")
	rootCmd.PersistentFlags().StringSlice("allowed-tools", nil, "Auto-approved tools")
	rootCmd.PersistentFlags().StringSlice("disallowed-tools", nil, "Disabled tools")
	rootCmd.PersistentFlags().Bool("dangerously-skip-permissions", false, "Skip all permission prompts")

	// Add subcommands
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(mcpCmd())
	rootCmd.AddCommand(updateCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runApp(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if first run / needs setup
	if setup.NeedsSetup(cfg) {
		result, err := setup.Run(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Setup error: %v\n", err)
		}
		if result.Skipped {
			fmt.Println("Setup skipped. Run 'oscode config setup' to configure later.")
			fmt.Println("You can also set environment variables: ANTHROPIC_API_KEY or OPENAI_API_KEY")
		} else {
			// Reload config after setup
			cfg, _ = config.Load()
		}
	}

	// Apply command line overrides
	if provider, _ := cmd.Flags().GetString("provider"); provider != "" {
		cfg.DefaultProvider = provider
	}
	if model, _ := cmd.Flags().GetString("model"); model != "" {
		cfg.DefaultModel = model
	}
	if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
		cfg.Verbose = true
	}
	if systemPrompt, _ := cmd.Flags().GetString("system-prompt"); systemPrompt != "" {
		cfg.SystemPrompt = systemPrompt
	}
	if mode, _ := cmd.Flags().GetString("permission-mode"); mode != "" {
		cfg.PermissionMode = mode
	}

	// Check for print mode
	printMode, _ := cmd.Flags().GetBool("print")
	outputFormat, _ := cmd.Flags().GetString("output-format")
	maxTurns, _ := cmd.Flags().GetInt("max-turns")
	continueSession, _ := cmd.Flags().GetBool("continue")
	resumeSession, _ := cmd.Flags().GetString("resume")
	skipPermissions, _ := cmd.Flags().GetBool("dangerously-skip-permissions")

	// Get initial prompt if provided
	var initialPrompt string
	if len(args) > 0 {
		initialPrompt = args[0]
	}

	// Create and run application
	application, err := app.New(cfg, app.Options{
		InitialPrompt:   initialPrompt,
		PrintMode:       printMode,
		OutputFormat:    outputFormat,
		MaxTurns:        maxTurns,
		ContinueSession: continueSession,
		ResumeSession:   resumeSession,
		SkipPermissions: skipPermissions,
	})
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	return application.Run()
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			fmt.Printf("Configuration file: %s\n", config.GetConfigPath())
			fmt.Printf("Provider: %s\n", cfg.DefaultProvider)
			fmt.Printf("Model: %s\n", cfg.DefaultModel)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show configuration file path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.GetConfigPath())
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Run interactive setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			result, err := setup.Run(cfg)
			if err != nil {
				return err
			}
			if result.Skipped {
				fmt.Println("Setup cancelled.")
			}
			return nil
		},
	})

	return cmd
}

func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP servers",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if len(cfg.MCP.Servers) == 0 {
				fmt.Println("No MCP servers configured")
				return nil
			}
			for name, server := range cfg.MCP.Servers {
				fmt.Printf("  %s (%s)\n", name, server.Transport)
			}
			return nil
		},
	})

	return cmd
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update OSCode to the latest version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Update functionality not yet implemented")
			fmt.Println("Please update via: go install github.com/heissanjay/oscode/cmd/oscode@latest")
		},
	}
}
