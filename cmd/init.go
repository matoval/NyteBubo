package cmd

import (
    "github.com/spf13/cobra"
    "NyteBubo/internal/cli"
)

var editCmd = &cobra.Command{
    Use:   "init",
    Short: "Set up NyteBubo",
    Run: func(cmd *cobra.Command, args []string) {
        cli.RunInitPrompt()
    },
}

func init() {
    rootCmd.AddCommand(editCmd)
}
