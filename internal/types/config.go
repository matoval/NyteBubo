package types

import (
	"fmt"
	"strings"
)

// Config represents the project configuration
type Config struct {
	ProjectName string
	GitRemote   string
	Lang        string
	Frameworks  []string
	Agent       AgentConfig `yaml:"agent,omitempty"`
}

// AgentConfig contains settings for the GitHub agent
type AgentConfig struct {
	Enabled        bool   `yaml:"enabled"`
	ServerPort     int    `yaml:"server_port"`
	ClaudeAPIKey   string `yaml:"claude_api_key,omitempty"`
	GitHubToken    string `yaml:"github_token,omitempty"`
	WebhookSecret  string `yaml:"webhook_secret,omitempty"`
	WorkingDir     string `yaml:"working_dir"`
	StateDBPath    string `yaml:"state_db_path"`
}

func (c Config) Display() string {
	var b strings.Builder
	b.WriteString("\nConfiguration Summary:\n")
	b.WriteString(fmt.Sprintf("  Project Name:  %s\n", c.ProjectName))
	b.WriteString(fmt.Sprintf("  Git Remote:    %s\n", c.GitRemote))
	b.WriteString(fmt.Sprintf("  Language:      %s\n", c.Lang))
	b.WriteString(fmt.Sprintf("  Frameworks:    %s\n", strings.Join(c.Frameworks, ", ")))
	if c.Agent.Enabled {
		b.WriteString("\n  Agent Configuration:\n")
		b.WriteString(fmt.Sprintf("    Enabled:     %v\n", c.Agent.Enabled))
		b.WriteString(fmt.Sprintf("    Server Port: %d\n", c.Agent.ServerPort))
		b.WriteString(fmt.Sprintf("    Working Dir: %s\n", c.Agent.WorkingDir))
		b.WriteString(fmt.Sprintf("    State DB:    %s\n", c.Agent.StateDBPath))
	}
	b.WriteString("\n")
	return b.String()
}
