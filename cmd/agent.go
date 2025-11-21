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
	// Load configuration
	configPath := "config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatal("config.yaml not found. Please run 'nyte-bubo init' first.")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config.yaml: %v", err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Fatalf("Failed to parse config.yaml: %v", err)
	}

	// Check if agent is enabled
	if !config.Agent.Enabled {
		log.Fatal("Agent is not enabled in config.yaml. Please enable it and configure the required settings.")
	}

	// Get credentials from environment variables (for security)
	claudeAPIKey := os.Getenv("CLAUDE_API_KEY")
	if claudeAPIKey == "" && config.Agent.ClaudeAPIKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set and not found in config.yaml")
	}
	if claudeAPIKey == "" {
		claudeAPIKey = config.Agent.ClaudeAPIKey
	}

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" && config.Agent.GitHubToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable is not set and not found in config.yaml")
	}
	if githubToken == "" {
		githubToken = config.Agent.GitHubToken
	}

	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" && config.Agent.WebhookSecret == "" {
		log.Println("Warning: WEBHOOK_SECRET is not set. Webhook signature verification will be disabled.")
	}
	if webhookSecret == "" {
		webhookSecret = config.Agent.WebhookSecret
	}

	// Create the issue agent
	agent, err := workflows.NewIssueAgent(githubToken, claudeAPIKey, config.Agent.StateDBPath, config.Agent.WorkingDir)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	// Create and start the webhook server
	webhookServer := server.NewWebhookServer(agent, webhookSecret)

	fmt.Printf(`
╔═══════════════════════════════════════════════╗
║        NyteBubo Agent Server Starting         ║
╚═══════════════════════════════════════════════╝

Configuration:
  Project: %s
  Server Port: %d
  Working Directory: %s
  State Database: %s

The agent is now listening for GitHub webhook events.
Configure your GitHub repository webhook to point to:
  http://your-server:%d/webhook

Health check endpoint:
  http://your-server:%d/health

Press Ctrl+C to stop the server.
`, config.ProjectName, config.Agent.ServerPort, config.Agent.WorkingDir, config.Agent.StateDBPath, config.Agent.ServerPort, config.Agent.ServerPort)

	if err := webhookServer.Start(config.Agent.ServerPort); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
