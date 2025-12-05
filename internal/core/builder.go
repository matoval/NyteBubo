package core

import (
	"fmt"
	"strings"
)

// LanguageBuilder defines language-specific build and test commands
type LanguageBuilder struct {
	Language     string
	BuildCommand []string
	TestCommand  []string
	RunCommand   []string
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
			Language:     "go",
			BuildCommand: []string{"go", "build", "./..."},
			TestCommand:  []string{"go", "test", "./..."},
			RunCommand:   []string{"go", "run", "."},
		},
		"python": {
			Language:     "python",
			BuildCommand: []string{"python", "-m", "py_compile"},
			TestCommand:  []string{"pytest", "."},
			RunCommand:   []string{"python", "main.py"},
		},
		"javascript": {
			Language:     "javascript",
			BuildCommand: []string{"npm", "install"},
			TestCommand:  []string{"npm", "test"},
			RunCommand:   []string{"npm", "start"},
		},
		"typescript": {
			Language:     "typescript",
			BuildCommand: []string{"npm", "run", "build"},
			TestCommand:  []string{"npm", "test"},
			RunCommand:   []string{"npm", "start"},
		},
		"rust": {
			Language:     "rust",
			BuildCommand: []string{"cargo", "build"},
			TestCommand:  []string{"cargo", "test"},
			RunCommand:   []string{"cargo", "run"},
		},
		"java": {
			Language:     "java",
			BuildCommand: []string{"mvn", "compile"},
			TestCommand:  []string{"mvn", "test"},
			RunCommand:   []string{"mvn", "exec:java"},
		},
	}

	if builder, ok := builders[language]; ok {
		return builder
	}

	// Default/unknown language - no build/test commands
	return &LanguageBuilder{
		Language:     language,
		BuildCommand: nil,
		TestCommand:  nil,
		RunCommand:   nil,
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
		return output, fmt.Errorf("tests failed: %w", err)
	}

	fmt.Printf("✅ Tests passed\n")
	return output, nil
}

// Verify runs both build and test
func (s *Sandbox) Verify() (buildOutput, testOutput string, err error) {
	// Try to build
	buildOutput, buildErr := s.Build()
	if buildErr != nil {
		return buildOutput, "", buildErr
	}

	// Try to test
	testOutput, testErr := s.Test()
	if testErr != nil {
		return buildOutput, testOutput, testErr
	}

	return buildOutput, testOutput, nil
}
