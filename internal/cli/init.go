package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"NyteBubo/internal/types"

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

	gitRemote := tap.Text(ctx, tap.TextOptions{
		Message: "Add git remote url",
		Validate: func(input string) error {
			gitURLPattern := `^(https:\/\/|git@|git:\/\/|ssh:\/\/|file:\/\/)?(github\.com|gitlab\.com|bitbucket\.org|[a-zA-Z0-9._-]+)(\/[a-zA-Z0-9._-]+)+(\.git)?$`
			re := regexp.MustCompile(gitURLPattern)
			if !re.MatchString(input) {
				return errors.New("Url provided is not a valid git url")
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

	frameworksInput := tap.Text(ctx, tap.TextOptions{
		Message: "List the frameworks to use (comma-separated):",
	})

	frameworksSlice := strings.Split(frameworksInput, ",")
	frameworks := make([]string, 0, len(frameworksSlice))
	for _, fw := range frameworksSlice {
		if trimmed := strings.TrimSpace(fw); trimmed != "" {
			frameworks = append(frameworks, trimmed)
		}
	}

	config := types.Config{
		ProjectName: projectName,
		GitRemote:   gitRemote,
		Lang:        lang,
		Frameworks:  frameworks,
	}

	// Display configuration summary
	fmt.Print(config.Display())

	confirmed := tap.Confirm(ctx, tap.ConfirmOptions{
		Message: "Continue with this configuration?",
	})

	if confirmed {
		tmpl, err := template.ParseFiles("internal/templates/init.tmpl")
		if err != nil {
			tap.Outro(fmt.Sprintf("Error parsing template: %v", err))
		}

		outputFile := "config.yaml"
		file, err := os.Create(outputFile)
		if err != nil {
			tap.Outro(fmt.Sprintf("Error creating file: %v", err))
		}
		defer file.Close()

		err = tmpl.Execute(file, config)
		if err != nil {
			tap.Outro(fmt.Sprintf("Error executing template: %v", err))
		}

		tap.Outro(fmt.Sprintf("Config file created successfully: %s", outputFile))
	} else {
		tap.Outro("Aborted")
		os.Exit(1)
	}
}

