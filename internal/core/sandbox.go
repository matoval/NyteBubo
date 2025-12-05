package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sandbox provides an isolated workspace for making and testing changes
type Sandbox struct {
	workspaceRoot string
	repoPath      string // Full path to cloned repo
	owner         string
	repo          string
	issueNumber   int
	githubToken   string
	defaultBranch string
}

// NewSandbox creates a new isolated workspace for an issue
func NewSandbox(workspaceRoot, owner, repo string, issueNumber int, githubToken string) (*Sandbox, error) {
	// Create workspace directory: workspace/owner-repo-issue-123
	workspaceName := fmt.Sprintf("%s-%s-%d", owner, repo, issueNumber)
	repoPath := filepath.Join(workspaceRoot, workspaceName)

	return &Sandbox{
		workspaceRoot: workspaceRoot,
		repoPath:      repoPath,
		owner:         owner,
		repo:          repo,
		issueNumber:   issueNumber,
		githubToken:   githubToken,
	}, nil
}

// CloneRepo clones the repository into the sandbox workspace
func (s *Sandbox) CloneRepo() error {
	// Check if workspace already exists
	if _, err := os.Stat(s.repoPath); err == nil {
		fmt.Printf("📁 Workspace already exists, using existing clone: %s\n", s.repoPath)
		return nil
	}

	fmt.Printf("📥 Cloning repository %s/%s into sandbox...\n", s.owner, s.repo)

	// Create workspace root if it doesn't exist
	if err := os.MkdirAll(s.workspaceRoot, 0755); err != nil {
		return fmt.Errorf("failed to create workspace root: %w", err)
	}

	// Clone with HTTPS using token authentication
	cloneURL := fmt.Sprintf("https://%s@github.com/%s/%s.git", s.githubToken, s.owner, s.repo)

	cmd := exec.Command("git", "clone", cloneURL, s.repoPath)
	// Don't show token in output
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repo: %w\nOutput: %s", err, output)
	}

	fmt.Printf("✅ Repository cloned successfully\n")
	return nil
}

// GetDefaultBranch detects and returns the default branch name
func (s *Sandbox) GetDefaultBranch() (string, error) {
	if s.defaultBranch != "" {
		return s.defaultBranch, nil
	}

	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD", "--short")
	cmd.Dir = s.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to checking common branch names
		for _, branch := range []string{"main", "master"} {
			cmd := exec.Command("git", "rev-parse", "--verify", branch)
			cmd.Dir = s.repoPath
			if err := cmd.Run(); err == nil {
				s.defaultBranch = branch
				return branch, nil
			}
		}
		return "", fmt.Errorf("failed to detect default branch: %w", err)
	}

	// Output format: "origin/main" -> extract "main"
	branchName := strings.TrimPrefix(strings.TrimSpace(string(output)), "origin/")
	s.defaultBranch = branchName
	return branchName, nil
}

// CreateBranch creates a new branch for the issue
func (s *Sandbox) CreateBranch(branchName string) error {
	fmt.Printf("🌿 Creating branch: %s\n", branchName)

	// Ensure we're on the default branch first
	defaultBranch, err := s.GetDefaultBranch()
	if err != nil {
		return err
	}

	// Checkout default branch
	cmd := exec.Command("git", "checkout", defaultBranch)
	cmd.Dir = s.repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout %s: %w\nOutput: %s", defaultBranch, err, output)
	}

	// Pull latest changes
	cmd = exec.Command("git", "pull", "origin", defaultBranch)
	cmd.Dir = s.repoPath
	if _, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("⚠️  Warning: failed to pull latest changes: %v\n", err)
		// Continue anyway - might be empty repo
	}

	// Create and checkout new branch
	cmd = exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = s.repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch: %w\nOutput: %s", err, output)
	}

	fmt.Printf("✅ Branch created successfully\n")
	return nil
}

// WriteFile writes content to a file in the sandbox
func (s *Sandbox) WriteFile(relativePath, content string) error {
	fullPath := filepath.Join(s.repoPath, relativePath)

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ReadFile reads a file from the sandbox
func (s *Sandbox) ReadFile(relativePath string) (string, error) {
	fullPath := filepath.Join(s.repoPath, relativePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ListFiles lists all files in the repository (excluding .git)
func (s *Sandbox) ListFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(s.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Only include files, not directories
		if !info.IsDir() {
			relPath, err := filepath.Rel(s.repoPath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

// RunCommand executes a command in the sandbox workspace
func (s *Sandbox) RunCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = s.repoPath
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Commit commits all changes in the workspace
func (s *Sandbox) Commit(message string) error {
	fmt.Printf("💾 Committing changes...\n")

	// Add all changes
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = s.repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage changes: %w\nOutput: %s", err, output)
	}

	// Configure git user (required for commits)
	cmd = exec.Command("git", "config", "user.name", "NyteBubo")
	cmd.Dir = s.repoPath
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "noreply@nytebubo")
	cmd.Dir = s.repoPath
	_ = cmd.Run()

	// Commit
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = s.repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if there's nothing to commit
		if strings.Contains(string(output), "nothing to commit") {
			return fmt.Errorf("no changes to commit")
		}
		return fmt.Errorf("failed to commit: %w\nOutput: %s", err, output)
	}

	fmt.Printf("✅ Changes committed\n")
	return nil
}

// Push pushes the branch to remote
func (s *Sandbox) Push(branchName string) error {
	fmt.Printf("📤 Pushing branch to remote...\n")

	// Push with token authentication
	cmd := exec.Command("git", "push", "-u", "origin", branchName)
	cmd.Dir = s.repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push: %w\nOutput: %s", err, output)
	}

	fmt.Printf("✅ Branch pushed successfully\n")
	return nil
}

// Cleanup removes the sandbox workspace
func (s *Sandbox) Cleanup() error {
	fmt.Printf("🧹 Cleaning up workspace: %s\n", s.repoPath)

	if err := os.RemoveAll(s.repoPath); err != nil {
		return fmt.Errorf("failed to cleanup workspace: %w", err)
	}

	fmt.Printf("✅ Workspace cleaned up\n")
	return nil
}

// GetRepoPath returns the full path to the repository
func (s *Sandbox) GetRepoPath() string {
	return s.repoPath
}
