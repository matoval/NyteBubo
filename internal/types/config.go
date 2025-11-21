package types

import (
	"fmt"
	"strings"
)

// Config represents the agent configuration
type Config struct {
	WorkingDir       string   `yaml:"working_dir"`
	StateDBPath      string   `yaml:"state_db_path"`
	ClaudeAPIKey     string   `yaml:"claude_api_key,omitempty"`
	GitHubToken      string   `yaml:"github_token,omitempty"`
	PollInterval     int      `yaml:"poll_interval"` // in seconds
	Repositories     []string `yaml:"repositories"`  // List of repositories to monitor (format: "owner/repo")

	// Webhook mode (optional, deprecated)
	ServerPort    int    `yaml:"server_port,omitempty"`
	WebhookSecret string `yaml:"webhook_secret,omitempty"`
	WebhookMode   bool   `yaml:"webhook_mode,omitempty"` // Set to true to use webhook mode instead of polling
}

func (c Config) Display() string {
	var b strings.Builder
	b.WriteString("\nAgent Configuration:\n")

	if c.WebhookMode {
		b.WriteString("  Mode:            Webhook\n")
		b.WriteString(fmt.Sprintf("  Server Port:     %d\n", c.ServerPort))
		b.WriteString(fmt.Sprintf("  Webhook Secret:  %s\n", maskSecret(c.WebhookSecret)))
	} else {
		b.WriteString("  Mode:            Polling\n")
		b.WriteString(fmt.Sprintf("  Poll Interval:   %ds\n", c.PollInterval))
		b.WriteString(fmt.Sprintf("  Repositories:    %s\n", strings.Join(c.Repositories, ", ")))
	}

	b.WriteString(fmt.Sprintf("  Working Dir:     %s\n", c.WorkingDir))
	b.WriteString(fmt.Sprintf("  State DB:        %s\n", c.StateDBPath))
	b.WriteString(fmt.Sprintf("  Claude API Key:  %s\n", maskSecret(c.ClaudeAPIKey)))
	b.WriteString(fmt.Sprintf("  GitHub Token:    %s\n", maskSecret(c.GitHubToken)))
	b.WriteString("\n")
	return b.String()
}

func maskSecret(secret string) string {
	if secret == "" {
		return "(using environment variable)"
	}
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}
