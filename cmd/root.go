package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "nytebubo",
    Short: "NyteBubo AI Assistant",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Welcome to NyteBubo!")
    },
}

func Execute() error {
    return rootCmd.Execute()
}
