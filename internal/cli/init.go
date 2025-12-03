package cli

import (
	"fmt"
	"os"
	"text/template"

	"NyteBubo/internal/types"
)

func RunInitPrompt() {
	outputFile := "config.yaml"

	// Check if config already exists
	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Config file '%s' already exists. Overwrite? (y/N): ", outputFile)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return
		}
	}

	// Create default config
	config := types.Config{
		WorkingDir:   "./workspace",
		StateDBPath:  "./agent_state.db",
		PollInterval: 30,
		Repositories: []string{"owner/repo"},
		WebhookMode:  false,
		ServerPort:   8080,
	}

	// Load template
	tmpl, err := template.ParseFiles("internal/templates/init.tmpl")
	if err != nil {
		fmt.Printf("Error parsing template: %v\n", err)
		os.Exit(1)
	}

	// Create config file
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	err = tmpl.Execute(file, config)
	if err != nil {
		fmt.Printf("Error executing template: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(`
✓ Config file created successfully: %s

Next steps:
  1. Set your environment variables:
     export OPENROUTER_API_KEY="your-api-key"
     export GITHUB_TOKEN="your-github-token"
     export WEBHOOK_SECRET="your-webhook-secret"

  2. Start the agent:
     nyte-bubo agent

  3. Configure your GitHub repository webhook to point to:
     http://your-server:8080/webhook

`, outputFile)
}

