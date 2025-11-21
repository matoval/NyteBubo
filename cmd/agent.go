package cmd

import (
	"fmt"
	"log"
	"os"

	"NyteBubo/internal/types"
	"NyteBubo/internal/workflows"
	"NyteBubo/server"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start the NyteBubo GitHub agent server",
	Long:  `Start the webhook server that listens for GitHub issue assignments and creates PRs.`,
	Run:   runAgent,
}

func init() {
	rootCmd.AddCommand(agentCmd)
}

func runAgent(cmd *cobra.Command, args []string) {
	// Load configuration with defaults
	config := types.Config{
		WorkingDir:   "./workspace",
		StateDBPath:  "./agent_state.db",
		PollInterval: 30,
		Repositories: []string{},
		WebhookMode:  false,
		ServerPort:   8080,
	}

	// Try to load config.yaml if it exists
	configPath := "config.yaml"
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatalf("Failed to read config.yaml: %v", err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			log.Fatalf("Failed to parse config.yaml: %v", err)
		}
	} else {
		log.Println("No config.yaml found, using defaults. Run 'nyte-bubo init' to create one.")
		log.Fatal("Error: repositories list is required. Please create a config.yaml file.")
	}

	// Validate configuration
	if !config.WebhookMode && len(config.Repositories) == 0 {
		log.Fatal("Error: repositories list cannot be empty in polling mode. Please add repositories to config.yaml")
	}

	// Get credentials from environment variables (preferred) or config file
	claudeAPIKey := os.Getenv("CLAUDE_API_KEY")
	if claudeAPIKey == "" && config.ClaudeAPIKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set and not found in config.yaml")
	}
	if claudeAPIKey == "" {
		claudeAPIKey = config.ClaudeAPIKey
	}

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" && config.GitHubToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable is not set and not found in config.yaml")
	}
	if githubToken == "" {
		githubToken = config.GitHubToken
	}

	// Create the issue agent
	agent, err := workflows.NewIssueAgent(githubToken, claudeAPIKey, config.StateDBPath, config.WorkingDir)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	// Start in appropriate mode
	if config.WebhookMode {
		startWebhookMode(agent, config)
	} else {
		startPollingMode(agent, config)
	}
}

func startPollingMode(agent *workflows.IssueAgent, config types.Config) {
	fmt.Printf(`
╔═══════════════════════════════════════════════╗
║        NyteBubo Agent Starting (Polling)      ║
╚═══════════════════════════════════════════════╝

Configuration:
  Mode: Polling
  Poll Interval: %ds
  Repositories: %v
  Working Directory: %s
  State Database: %s

The agent is now polling for assigned issues.
No public endpoint required - runs entirely on your local network!

Press Ctrl+C to stop the agent.
`, config.PollInterval, config.Repositories, config.WorkingDir, config.StateDBPath)

	// Start polling
	if err := agent.StartPolling(config.PollInterval, config.Repositories); err != nil {
		log.Fatalf("Polling error: %v", err)
	}
}

func startWebhookMode(agent *workflows.IssueAgent, config types.Config) {
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" && config.WebhookSecret == "" {
		log.Println("Warning: WEBHOOK_SECRET is not set. Webhook signature verification will be disabled.")
	}
	if webhookSecret == "" {
		webhookSecret = config.WebhookSecret
	}

	// Create and start the webhook server
	webhookServer := server.NewWebhookServer(agent, webhookSecret)

	fmt.Printf(`
╔═══════════════════════════════════════════════╗
║        NyteBubo Agent Starting (Webhook)      ║
╚═══════════════════════════════════════════════╝

Configuration:
  Mode: Webhook
  Server Port: %d
  Working Directory: %s
  State Database: %s

The agent is now listening for GitHub webhook events.
Configure your GitHub repository webhook to point to:
  http://your-server:%d/webhook

Health check endpoint:
  http://your-server:%d/health

Press Ctrl+C to stop the server.
`, config.ServerPort, config.WorkingDir, config.StateDBPath, config.ServerPort, config.ServerPort)

	if err := webhookServer.Start(config.ServerPort); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
