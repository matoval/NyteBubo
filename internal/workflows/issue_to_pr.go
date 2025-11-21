package workflows

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"NyteBubo/internal/core"
)

// IssueAgent orchestrates the issue-to-PR workflow
type IssueAgent struct {
	github       *core.GitHubClient
	claude       *core.ClaudeAgent
	stateManager *core.StateManager
	workingDir   string
}

// NewIssueAgent creates a new issue agent
func NewIssueAgent(githubToken, claudeAPIKey, stateDBPath, workingDir string) (*IssueAgent, error) {
	github := core.NewGitHubClient(githubToken)
	claude := core.NewClaudeAgent(claudeAPIKey)

	stateManager, err := core.NewStateManager(stateDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	return &IssueAgent{
		github:       github,
		claude:       claude,
		stateManager: stateManager,
		workingDir:   workingDir,
	}, nil
}

// HandleIssueAssignment handles when the agent is assigned to an issue
func (ia *IssueAgent) HandleIssueAssignment(owner, repo string, issueNumber int) error {
	// Get the issue
	issue, err := ia.github.GetIssue(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	// Check if we already have state for this issue
	state, err := ia.stateManager.GetState(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	// If no state, create a new one
	if state == nil {
		state = &core.State{
			Owner:       owner,
			Repo:        repo,
			IssueNumber: issueNumber,
			Status:      "analyzing",
			Conversation: []core.AgentMessage{},
		}
	}

	// Analyze the issue with Claude
	title := issue.GetTitle()
	body := issue.GetBody()

	response, usage, err := ia.claude.AnalyzeIssue(title, body)
	if err != nil {
		return fmt.Errorf("failed to analyze issue: %w", err)
	}

	// Track token usage
	state.TotalInputTokens += usage.InputTokens
	state.TotalOutputTokens += usage.OutputTokens
	state.TotalCost += usage.EstimatedCost

	// Update conversation history
	state.Conversation = append(state.Conversation, core.AgentMessage{
		Role:    "user",
		Content: fmt.Sprintf("Issue Title: %s\n\nIssue Description:\n%s", title, body),
	})
	state.Conversation = append(state.Conversation, core.AgentMessage{
		Role:    "assistant",
		Content: response,
	})

	// Post the analysis as a comment
	commentBody := fmt.Sprintf("👋 Hi! I've been assigned to this issue. Here's my understanding:\n\n%s", response)
	if err := ia.github.CreateIssueComment(owner, repo, issueNumber, commentBody); err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	// Determine next status based on response
	if strings.Contains(strings.ToLower(response), "question") || strings.Contains(strings.ToLower(response), "clarif") {
		state.Status = "waiting_for_clarification"
	} else {
		state.Status = "ready_to_implement"
	}

	// Save state
	if err := ia.stateManager.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// If ready to implement, start implementation
	if state.Status == "ready_to_implement" {
		return ia.StartImplementation(owner, repo, issueNumber)
	}

	return nil
}

// HandleIssueComment handles new comments on an issue the agent is working on
func (ia *IssueAgent) HandleIssueComment(owner, repo string, issueNumber int, commentBody string) error {
	// Get current state
	state, err := ia.stateManager.GetState(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	if state == nil {
		return fmt.Errorf("no state found for this issue")
	}

	// Add the comment to conversation history
	state.Conversation = append(state.Conversation, core.AgentMessage{
		Role:    "user",
		Content: commentBody,
	})

	// Get Claude's response
	response, usage, err := ia.claude.SendMessage(state.Conversation, "You are a helpful coding assistant working on a GitHub issue. Respond to the user's comment.")
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}

	// Track token usage
	state.TotalInputTokens += usage.InputTokens
	state.TotalOutputTokens += usage.OutputTokens
	state.TotalCost += usage.EstimatedCost

	// Update conversation
	state.Conversation = append(state.Conversation, core.AgentMessage{
		Role:    "assistant",
		Content: response,
	})

	// Post response as comment
	if err := ia.github.CreateIssueComment(owner, repo, issueNumber, response); err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	// Check if we're ready to implement now
	if state.Status == "waiting_for_clarification" {
		lowerResponse := strings.ToLower(response)
		if !strings.Contains(lowerResponse, "question") && !strings.Contains(lowerResponse, "clarif") {
			state.Status = "ready_to_implement"
			if err := ia.stateManager.SaveState(state); err != nil {
				return fmt.Errorf("failed to save state: %w", err)
			}
			return ia.StartImplementation(owner, repo, issueNumber)
		}
	}

	// Save state
	if err := ia.stateManager.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// StartImplementation begins implementing the solution
func (ia *IssueAgent) StartImplementation(owner, repo string, issueNumber int) error {
	state, err := ia.stateManager.GetState(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	if state == nil {
		return fmt.Errorf("no state found")
	}

	// Update status
	state.Status = "implementing"
	if err := ia.stateManager.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Notify that we're starting implementation
	comment := "🚀 Great! I have a clear understanding now. I'll start working on this and create a pull request shortly."
	if err := ia.github.CreateIssueComment(owner, repo, issueNumber, comment); err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	// Get repository info
	repository, err := ia.github.GetRepository(owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	language := repository.GetLanguage()
	defaultBranch := repository.GetDefaultBranch()

	// Create a branch name
	branchName := fmt.Sprintf("nyte-bubo/issue-%d", issueNumber)
	state.BranchName = branchName

	// Create branch
	if err := ia.github.CreateBranch(owner, repo, branchName, defaultBranch); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Get code generation from Claude
	task := fmt.Sprintf("Implement the changes for issue #%d", issueNumber)
	repoContext := fmt.Sprintf("Repository: %s/%s, Language: %s", owner, repo, language)

	codeResponse, usage, err := ia.claude.GenerateCode(task, repoContext, language, state.Conversation)
	if err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	// Track token usage
	state.TotalInputTokens += usage.InputTokens
	state.TotalOutputTokens += usage.OutputTokens
	state.TotalCost += usage.EstimatedCost

	// Parse the code response and extract file changes
	fileChanges := parseCodeChanges(codeResponse)

	// Apply the changes to the branch
	for filePath, content := range fileChanges {
		if err := ia.github.CreateOrUpdateFile(owner, repo, filePath, fmt.Sprintf("Update %s for issue #%d", filePath, issueNumber), content, branchName, nil); err != nil {
			return fmt.Errorf("failed to update file %s: %w", filePath, err)
		}
	}

	// Create PR
	issue, err := ia.github.GetIssue(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	prTitle := fmt.Sprintf("Fix: %s", issue.GetTitle())
	prBody := fmt.Sprintf("Fixes #%d\n\n%s\n\n---\n\n🤖 This PR was automatically generated by NyteBubo", issueNumber, codeResponse)

	pr, err := ia.github.CreatePullRequest(owner, repo, prTitle, prBody, branchName, defaultBranch)
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	// Update state
	prNumber := pr.GetNumber()
	state.PRNumber = &prNumber
	state.Status = "pr_created"
	if err := ia.stateManager.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Comment on the issue with PR link
	prComment := fmt.Sprintf("✅ I've created a pull request: #%d", prNumber)
	if err := ia.github.CreateIssueComment(owner, repo, issueNumber, prComment); err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	return nil
}

// HandlePRComment handles comments on the PR
func (ia *IssueAgent) HandlePRComment(owner, repo string, prNumber int, commentBody string) error {
	// Find the issue number from PR (we'll need to store this mapping)
	// For now, we'll extract from the PR body
	pr, err := ia.github.GetPullRequest(owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR: %w", err)
	}

	// Extract issue number from PR body
	issueNumber := extractIssueNumber(pr.GetBody())
	if issueNumber == 0 {
		return fmt.Errorf("could not find issue number in PR body")
	}

	state, err := ia.stateManager.GetState(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	if state == nil {
		return fmt.Errorf("no state found")
	}

	// Update status
	state.Status = "reviewing"

	// Add comment to conversation
	state.Conversation = append(state.Conversation, core.AgentMessage{
		Role:    "user",
		Content: fmt.Sprintf("Review feedback: %s", commentBody),
	})

	// Get updated code from Claude
	response, usage, err := ia.claude.ReviewFeedback(commentBody, "", state.Conversation)
	if err != nil {
		return fmt.Errorf("failed to get review response: %w", err)
	}

	// Track token usage
	state.TotalInputTokens += usage.InputTokens
	state.TotalOutputTokens += usage.OutputTokens
	state.TotalCost += usage.EstimatedCost

	// Update conversation
	state.Conversation = append(state.Conversation, core.AgentMessage{
		Role:    "assistant",
		Content: response,
	})

	// Parse and apply changes
	fileChanges := parseCodeChanges(response)
	for filePath, content := range fileChanges {
		if err := ia.github.CreateOrUpdateFile(owner, repo, filePath, fmt.Sprintf("Address review feedback for issue #%d", issueNumber), content, state.BranchName, nil); err != nil {
			return fmt.Errorf("failed to update file %s: %w", filePath, err)
		}
	}

	// Save state
	if err := ia.stateManager.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// parseCodeChanges extracts file paths and content from Claude's response
func parseCodeChanges(response string) map[string]string {
	changes := make(map[string]string)

	// Simple regex to find code blocks with file paths
	// Format: ```language filename.ext
	re := regexp.MustCompile("(?s)```\\w+\\s+([\\w/.-]+)\\n(.+?)```")
	matches := re.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) == 3 {
			filePath := match[1]
			content := match[2]
			changes[filePath] = content
		}
	}

	return changes
}

// extractIssueNumber extracts the issue number from PR body
func extractIssueNumber(body string) int {
	re := regexp.MustCompile(`Fixes #(\d+)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) == 2 {
		var issueNum int
		fmt.Sscanf(matches[1], "%d", &issueNum)
		return issueNum
	}
	return 0
}

// Close closes the agent and cleans up resources
func (ia *IssueAgent) Close() error {
	return ia.stateManager.Close()
}

// StartPolling begins polling for assigned issues
func (ia *IssueAgent) StartPolling(pollIntervalSeconds int, repositories []string) error {
	poller, err := core.NewPoller(
		ia.github,
		ia.stateManager,
		core.PollerConfig{
			PollInterval: time.Duration(pollIntervalSeconds) * time.Second,
			Repositories: repositories,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create poller: %w", err)
	}

	// Start polling and handle new issues
	return poller.Start(func(owner, repo string, issueNumber int) error {
		return ia.HandleIssueAssignment(owner, repo, issueNumber)
	})
}
