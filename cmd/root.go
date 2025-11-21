package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "nyte-bubo",
    Short: "NyteBubo - GitHub Issue to PR Agent",
    Long: `NyteBubo is an AI-powered agent that listens for GitHub issue assignments,
analyzes them, and automatically creates pull requests to resolve the issues.

The agent runs as a server waiting for GitHub webhook events.`,
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("NyteBubo - GitHub Issue to PR Agent")
        fmt.Println("\nAvailable commands:")
        fmt.Println("  init   - Create a config.yaml file")
        fmt.Println("  agent  - Start the polling agent server")
        fmt.Println("  stats  - View token usage statistics")
        fmt.Println("\nUse 'nyte-bubo [command] --help' for more information about a command.")
    },
}

func Execute() error {
    return rootCmd.Execute()
}
