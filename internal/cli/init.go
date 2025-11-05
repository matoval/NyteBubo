package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/yarlson/tap"
)

func RunInitPrompt() {
	ctx := context.Background()

	tap.Intro("Welcome to  NyteBubo init")

	projectName := tap.Text(ctx, tap.TextOptions{
		Message: "Enter project name:",
		Validate: func(input string) error {
			if input == "" {
				return errors.New("Project name cannot be empty")
			}
			return nil
		},
	})

	lang := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "Choose programming language:",
		Options: []tap.SelectOption[string]{
			{Value: "go", Label: "Go", Hint: "Golang"},
			{Value: "python", Label: "Python"},
			{Value: "javascript", Label: "JavaScript"},
		},
	})

	frameworks := tap.Text(ctx, tap.TextOptions{
		Message: "List the frameworks to use",
	})

	confirmed := tap.Confirm(ctx, tap.ConfirmOptions{
		Message: fmt.Sprintf("Create project %s in %s using these frameworks: %s? Continue?", projectName, lang, frameworks),
	})

	if !confirmed {
		tap.Outro("Aborted")
		os.Exit(1)
	}
}

