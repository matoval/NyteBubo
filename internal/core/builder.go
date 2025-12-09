package core

import (
	"fmt"
	"strings"
)

// LanguageBuilder defines language-specific build and test commands
type LanguageBuilder struct {
	Language      string
	BuildCommand  []string
	TestCommand   []string
	RunCommand    []string
	LintCommands  []LintCommand // Multiple linting commands (formatters, static analyzers, etc.)
	FormatCommand []string      // Optional: auto-format command to run before linting
}

// LintCommand represents a single linting command with metadata
type LintCommand struct {
	Name        string   // Human-readable name (e.g., "gofmt", "go vet")
	Command     []string // The actual command to run
	Optional    bool     // If true, failure won't block (just warning)
	Description string   // What this linter checks for
}

// DetectLanguage attempts to detect the repository's primary language
func (s *Sandbox) DetectLanguage() (string, error) {
	files, err := s.ListFiles()
	if err != nil {
		return "", err
	}

	// Count file extensions
	extensionCounts := make(map[string]int)
	for _, file := range files {
		if strings.Contains(file, ".") {
			parts := strings.Split(file, ".")
			ext := parts[len(parts)-1]
			extensionCounts[ext]++
		}
	}

	// Language detection heuristics
	languageMap := map[string]string{
		"go":   "go",
		"py":   "python",
		"js":   "javascript",
		"ts":   "typescript",
		"java": "java",
		"rs":   "rust",
		"rb":   "ruby",
		"php":  "php",
		"c":    "c",
		"cpp":  "cpp",
		"cs":   "csharp",
	}

	// Find most common extension
	maxCount := 0
	detectedLang := "unknown"
	for ext, count := range extensionCounts {
		if count > maxCount {
			if lang, ok := languageMap[ext]; ok {
				detectedLang = lang
				maxCount = count
			}
		}
	}

	// Additional checks for specific files
	for _, file := range files {
		switch file {
		case "go.mod":
			return "go", nil
		case "package.json":
			return "javascript", nil
		case "Cargo.toml":
			return "rust", nil
		case "requirements.txt", "setup.py", "pyproject.toml":
			return "python", nil
		case "pom.xml", "build.gradle":
			return "java", nil
		}
	}

	return detectedLang, nil
}

// GetBuilder returns the appropriate builder for the detected language
func GetBuilder(language string) *LanguageBuilder {
	builders := map[string]*LanguageBuilder{
		"go": {
			Language:      "go",
			BuildCommand:  []string{"go", "build", "./..."},
			TestCommand:   []string{"go", "test", "./..."},
			RunCommand:    []string{"go", "run", "."},
			FormatCommand: []string{"goimports", "-w", "."},
			LintCommands: []LintCommand{
				{
					Name:        "gofmt",
					Command:     []string{"gofmt", "-l", "."},
					Optional:    false,
					Description: "Check Go formatting",
				},
				{
					Name:        "go vet",
					Command:     []string{"go", "vet", "./..."},
					Optional:    false,
					Description: "Check for suspicious constructs",
				},
				{
					Name:        "staticcheck",
					Command:     []string{"staticcheck", "./..."},
					Optional:    true, // Optional as it might not be installed
					Description: "Advanced static analysis",
				},
			},
		},
		"python": {
			Language:      "python",
			BuildCommand:  []string{"python", "-m", "py_compile"},
			TestCommand:   []string{"pytest", "."},
			RunCommand:    []string{"python", "main.py"},
			FormatCommand: []string{"black", "."},
			LintCommands: []LintCommand{
				{
					Name:        "flake8",
					Command:     []string{"flake8", ".", "--max-line-length=100"},
					Optional:    true,
					Description: "Check PEP 8 style guide",
				},
				{
					Name:        "pylint",
					Command:     []string{"pylint", "**/*.py"},
					Optional:    true,
					Description: "Advanced Python linting",
				},
			},
		},
		"javascript": {
			Language:      "javascript",
			BuildCommand:  []string{"npm", "install"},
			TestCommand:   []string{"npm", "test"},
			RunCommand:    []string{"npm", "start"},
			FormatCommand: []string{"npm", "run", "format"},
			LintCommands: []LintCommand{
				{
					Name:        "eslint",
					Command:     []string{"npm", "run", "lint"},
					Optional:    true,
					Description: "Check JavaScript style and errors",
				},
			},
		},
		"typescript": {
			Language:      "typescript",
			BuildCommand:  []string{"npm", "run", "build"},
			TestCommand:   []string{"npm", "test"},
			RunCommand:    []string{"npm", "start"},
			FormatCommand: []string{"npm", "run", "format"},
			LintCommands: []LintCommand{
				{
					Name:        "eslint",
					Command:     []string{"npm", "run", "lint"},
					Optional:    true,
					Description: "Check TypeScript style and errors",
				},
				{
					Name:        "tsc",
					Command:     []string{"npx", "tsc", "--noEmit"},
					Optional:    false,
					Description: "TypeScript type checking",
				},
			},
		},
		"rust": {
			Language:      "rust",
			BuildCommand:  []string{"cargo", "build"},
			TestCommand:   []string{"cargo", "test"},
			RunCommand:    []string{"cargo", "run"},
			FormatCommand: []string{"cargo", "fmt"},
			LintCommands: []LintCommand{
				{
					Name:        "cargo fmt check",
					Command:     []string{"cargo", "fmt", "--", "--check"},
					Optional:    false,
					Description: "Check Rust formatting",
				},
				{
					Name:        "cargo clippy",
					Command:     []string{"cargo", "clippy", "--", "-D", "warnings"},
					Optional:    false,
					Description: "Rust linting",
				},
			},
		},
		"java": {
			Language:     "java",
			BuildCommand: []string{"mvn", "compile"},
			TestCommand:  []string{"mvn", "test"},
			RunCommand:   []string{"mvn", "exec:java"},
			LintCommands: []LintCommand{
				{
					Name:        "checkstyle",
					Command:     []string{"mvn", "checkstyle:check"},
					Optional:    true,
					Description: "Java style checking",
				},
			},
		},
	}

	if builder, ok := builders[language]; ok {
		return builder
	}

	// Default/unknown language - no build/test commands
	return &LanguageBuilder{
		Language:      language,
		BuildCommand:  nil,
		TestCommand:   nil,
		RunCommand:    nil,
		LintCommands:  nil,
		FormatCommand: nil,
	}
}

// Build runs the build command in the sandbox
func (s *Sandbox) Build() (string, error) {
	language, err := s.DetectLanguage()
	if err != nil {
		return "", fmt.Errorf("failed to detect language: %w", err)
	}

	builder := GetBuilder(language)
	if builder.BuildCommand == nil {
		fmt.Printf("⚠️  No build command for language: %s\n", language)
		return "No build command available", nil
	}

	fmt.Printf("🔨 Building project (%s)...\n", language)
	output, err := s.RunCommand(builder.BuildCommand[0], builder.BuildCommand[1:]...)
	if err != nil {
		// Check for empty output - suggests command might not exist or be configured
		if strings.TrimSpace(output) == "" {
			return output, fmt.Errorf("build failed with no output (command may not be configured): %w\n\nHint: This repository may not have build scripts set up. Check if build commands like '%s' are available.", err, builder.BuildCommand[0])
		}
		return output, fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("✅ Build successful\n")
	return output, nil
}

// Test runs the test command in the sandbox
func (s *Sandbox) Test() (string, error) {
	language, err := s.DetectLanguage()
	if err != nil {
		return "", fmt.Errorf("failed to detect language: %w", err)
	}

	builder := GetBuilder(language)
	if builder.TestCommand == nil {
		fmt.Printf("⚠️  No test command for language: %s\n", language)
		return "No test command available", nil
	}

	fmt.Printf("🧪 Running tests (%s)...\n", language)
	output, err := s.RunCommand(builder.TestCommand[0], builder.TestCommand[1:]...)
	if err != nil {
		// Check for empty output - suggests command might not exist or be configured
		if strings.TrimSpace(output) == "" {
			return output, fmt.Errorf("tests failed with no output (command may not be configured): %w\n\nHint: This repository may not have test scripts set up. Check if test commands like '%s' are available.", err, builder.TestCommand[0])
		}
		return output, fmt.Errorf("tests failed: %w", err)
	}

	fmt.Printf("✅ Tests passed\n")
	return output, nil
}

// Format auto-formats code before verification
func (s *Sandbox) Format() (string, error) {
	language, err := s.DetectLanguage()
	if err != nil {
		return "", fmt.Errorf("failed to detect language: %w", err)
	}

	builder := GetBuilder(language)
	if builder.FormatCommand == nil {
		fmt.Printf("⚠️  No format command for language: %s\n", language)
		return "No format command available", nil
	}

	fmt.Printf("🎨 Auto-formatting code (%s)...\n", language)
	output, err := s.RunCommand(builder.FormatCommand[0], builder.FormatCommand[1:]...)
	if err != nil {
		// Formatting failure is not critical - just log it
		fmt.Printf("⚠️  Warning: formatting failed: %v\n", err)
		return output, nil // Don't block on format failures
	}

	fmt.Printf("✅ Code formatted\n")
	return output, nil
}

// LintResult represents the result of a single linting check
type LintResult struct {
	Name        string
	Output      string
	Passed      bool
	Optional    bool
	Description string
}

// Lint runs all linting checks for the detected language
func (s *Sandbox) Lint() ([]LintResult, error) {
	language, err := s.DetectLanguage()
	if err != nil {
		return nil, fmt.Errorf("failed to detect language: %w", err)
	}

	builder := GetBuilder(language)
	if len(builder.LintCommands) == 0 {
		fmt.Printf("⚠️  No lint commands for language: %s\n", language)
		return nil, nil
	}

	fmt.Printf("🔍 Running linting checks (%s)...\n", language)

	var results []LintResult
	var criticalFailures []string

	for _, lintCmd := range builder.LintCommands {
		fmt.Printf("  📋 Running %s: %s\n", lintCmd.Name, lintCmd.Description)

		output, err := s.RunCommand(lintCmd.Command[0], lintCmd.Command[1:]...)

		result := LintResult{
			Name:        lintCmd.Name,
			Output:      output,
			Passed:      err == nil,
			Optional:    lintCmd.Optional,
			Description: lintCmd.Description,
		}
		results = append(results, result)

		if err != nil {
			if lintCmd.Optional {
				fmt.Printf("  ⚠️  %s failed (optional): %v\n", lintCmd.Name, err)
			} else {
				fmt.Printf("  ❌ %s failed (critical): %v\n", lintCmd.Name, err)
				criticalFailures = append(criticalFailures, fmt.Sprintf("%s: %s", lintCmd.Name, output))
			}
		} else {
			// Check if gofmt found unformatted files (even without error)
			if lintCmd.Name == "gofmt" && strings.TrimSpace(output) != "" {
				fmt.Printf("  ❌ %s found unformatted files:\n%s\n", lintCmd.Name, output)
				result.Passed = false
				criticalFailures = append(criticalFailures, fmt.Sprintf("gofmt: files need formatting:\n%s", output))
			} else {
				fmt.Printf("  ✅ %s passed\n", lintCmd.Name)
			}
		}
	}

	// If there were critical failures, return an error
	if len(criticalFailures) > 0 {
		return results, fmt.Errorf("critical linting failures:\n%s", strings.Join(criticalFailures, "\n---\n"))
	}

	fmt.Printf("✅ All linting checks passed\n")
	return results, nil
}

// Verify runs format (if available), lint, build, and test
func (s *Sandbox) Verify() (buildOutput, testOutput string, err error) {
	// Step 1: Auto-format code (non-blocking)
	fmt.Printf("\n━━━ Step 1/4: Auto-formatting ━━━\n")
	formatOutput, _ := s.Format()

	// Step 2: Lint code (blocking on critical failures)
	fmt.Printf("\n━━━ Step 2/4: Linting ━━━\n")
	lintResults, lintErr := s.Lint()
	if lintErr != nil {
		// Format lint output for display
		var lintOutput strings.Builder
		lintOutput.WriteString("Linting failed:\n\n")
		for _, result := range lintResults {
			status := "✅"
			if !result.Passed {
				status = "❌"
				if result.Optional {
					status = "⚠️"
				}
			}
			lintOutput.WriteString(fmt.Sprintf("%s %s: %s\n", status, result.Name, result.Description))
			if !result.Passed && result.Output != "" {
				lintOutput.WriteString(fmt.Sprintf("   Output:\n%s\n\n", result.Output))
			}
		}
		return formatOutput, lintOutput.String(), lintErr
	}

	// Step 3: Build
	fmt.Printf("\n━━━ Step 3/4: Building ━━━\n")
	buildOutput, buildErr := s.Build()
	if buildErr != nil {
		return buildOutput, "", buildErr
	}

	// Step 4: Test
	fmt.Printf("\n━━━ Step 4/4: Testing ━━━\n")
	testOutput, testErr := s.Test()
	if testErr != nil {
		return buildOutput, testOutput, testErr
	}

	fmt.Printf("\n✅ All verification steps passed!\n")
	return buildOutput, testOutput, nil
}
