package workflows

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"NyteBubo/internal/core"
	"github.com/google/go-github/v63/github"
)

// IssueAgent orchestrates the issue-to-PR workflow
type IssueAgent struct {
	github       *core.GitHubClient
	claude       *core.ClaudeAgent
	stateManager *core.StateManager
	workingDir   string
}

// NewIssueAgent creates a new issue agent
func NewIssueAgent(githubToken, claudeAPIKey, model, stateDBPath, workingDir string) (*IssueAgent, error) {
	github := core.NewGitHubClient(githubToken)
	claude := core.NewClaudeAgent(claudeAPIKey, model)

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
	fmt.Printf("🔍 Starting analysis of issue %s/%s #%d\n", owner, repo, issueNumber)

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

	// If no state, create a new one and load existing conversation from GitHub
	if state == nil {
		state = &core.State{
			Owner:       owner,
			Repo:        repo,
			IssueNumber: issueNumber,
			Status:      "analyzing",
			Conversation: []core.AgentMessage{},
		}

		// Fetch existing comments to build conversation history
		fmt.Printf("📥 Fetching existing comments from GitHub to build context...\n")
		comments, err := ia.github.ListIssueComments(owner, repo, issueNumber)
		if err != nil {
			fmt.Printf("⚠️  Warning: failed to fetch existing comments: %v\n", err)
		} else if len(comments) > 0 {
			fmt.Printf("📚 Found %d existing comment(s) to add to context\n", len(comments))
		}

		// Build conversation from issue description and comments
		title := issue.GetTitle()
		body := issue.GetBody()

		state.Conversation = append(state.Conversation, core.AgentMessage{
			Role:    "user",
			Content: fmt.Sprintf("Issue Title: %s\n\nIssue Description:\n%s", title, body),
		})

		// Add existing comments to conversation
		botUsername, err := ia.github.GetAuthenticatedUser()
		if err == nil && len(comments) > 0 {
			for _, comment := range comments {
				isBot := comment.GetUser().GetLogin() == botUsername.GetLogin()
				role := "user"
				if isBot {
					role = "assistant"
				}
				state.Conversation = append(state.Conversation, core.AgentMessage{
					Role:    role,
					Content: comment.GetBody(),
				})
			}
		}
	}

	// Analyze with full context
	fmt.Printf("🤖 Sending issue to AI for analysis (with %d message(s) of context)...\n", len(state.Conversation))

	title := issue.GetTitle()
	body := issue.GetBody()

	var response string
	var usage core.TokenUsage

	// If we have existing conversation, use it
	if len(state.Conversation) > 1 {
		// Already has conversation history, ask AI to confirm understanding
		systemPrompt := "You are a helpful coding assistant. Review the entire conversation and determine if you have enough information to proceed with implementation. If you do, say so clearly. If not, ask specific clarifying questions."
		response, usage, err = ia.claude.SendMessage(state.Conversation, systemPrompt)
	} else {
		// Fresh issue, analyze it
		response, usage, err = ia.claude.AnalyzeIssue(title, body)
		state.Conversation = append(state.Conversation, core.AgentMessage{
			Role:    "assistant",
			Content: response,
		})
	}

	if err != nil {
		return fmt.Errorf("failed to analyze issue: %w", err)
	}
	fmt.Printf("✅ AI analysis complete\n")

	// Track token usage
	state.TotalInputTokens += usage.InputTokens
	state.TotalOutputTokens += usage.OutputTokens
	state.TotalCost += usage.Cost

	// Add AI response to conversation if not already there
	if len(state.Conversation) > 0 && state.Conversation[len(state.Conversation)-1].Content != response {
		state.Conversation = append(state.Conversation, core.AgentMessage{
			Role:    "assistant",
			Content: response,
		})
	}

	// Post the analysis as a comment (only if it's actually new analysis, not just reviewing existing conversation)
	shouldComment := len(state.Conversation) <= 2 // Only the initial issue and bot response

	// Check if response indicates readiness without asking questions
	lowerResponse := strings.ToLower(response)
	isAskingQuestion := strings.Contains(lowerResponse, "question?") ||
		strings.Contains(lowerResponse, "questions:") ||
		strings.Contains(lowerResponse, "could you clarify") ||
		strings.Contains(lowerResponse, "can you clarify") ||
		strings.Contains(lowerResponse, "please clarify") ||
		strings.Contains(lowerResponse, "need clarification") ||
		strings.HasSuffix(lowerResponse, "?")

	if shouldComment {
		commentBody := fmt.Sprintf("👋 Hi! I've been assigned to this issue. Here's my understanding:\n\n%s", response)
		if err := ia.github.CreateIssueComment(owner, repo, issueNumber, commentBody); err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}
	}

	// Determine next status based on response
	if isAskingQuestion {
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
	fmt.Printf("💬 Processing new comment on issue %s/%s #%d\n", owner, repo, issueNumber)

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
	fmt.Printf("🤖 Sending comment to AI for response...\n")
	response, usage, err := ia.claude.SendMessage(state.Conversation, "You are a helpful coding assistant working on a GitHub issue. Respond to the user's comment.")
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}
	fmt.Printf("✅ AI response generated\n")

	// Track token usage
	state.TotalInputTokens += usage.InputTokens
	state.TotalOutputTokens += usage.OutputTokens
	state.TotalCost += usage.Cost

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
		// Check if the response is asking for clarification (not just mentioning it)
		isAskingQuestion := strings.Contains(lowerResponse, "question?") ||
			strings.Contains(lowerResponse, "questions:") ||
			strings.Contains(lowerResponse, "could you clarify") ||
			strings.Contains(lowerResponse, "can you clarify") ||
			strings.Contains(lowerResponse, "please clarify") ||
			strings.Contains(lowerResponse, "need clarification") ||
			strings.HasSuffix(lowerResponse, "?")

		if !isAskingQuestion {
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
	fmt.Printf("🚀 Starting implementation for issue %s/%s #%d\n", owner, repo, issueNumber)

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
	if defaultBranch == "" {
		defaultBranch = "main" // Default to main if not set
	}

	// Create a branch name
	branchName := fmt.Sprintf("nytebubo/issue-%d", issueNumber)
	state.BranchName = branchName

	// Try to create branch - if repo is empty, we'll commit directly to main
	fmt.Printf("🌿 Creating branch: %s\n", branchName)
	err = ia.github.CreateBranch(owner, repo, branchName, defaultBranch)
	if err != nil {
		// Check if repo is empty (409 error)
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "empty") {
			fmt.Printf("📝 Repository is empty - will create initial commit on %s instead of branch\n", defaultBranch)
			branchName = defaultBranch // Commit directly to main
			state.BranchName = branchName
		} else {
			return fmt.Errorf("failed to create branch: %w", err)
		}
	}

	// Get code generation from Claude with retry logic for rate limits
	task := fmt.Sprintf("Implement the changes for issue #%d", issueNumber)
	repoContext := fmt.Sprintf("Repository: %s/%s, Language: %s", owner, repo, language)

	fmt.Printf("🤖 Generating code with AI...\n")

	// Backoff pattern: 60s, 120s, 240s, then 240s forever
	backoffDurations := []time.Duration{60 * time.Second, 120 * time.Second, 240 * time.Second}
	maxBackoff := 240 * time.Second

	var codeResponse string
	var usage core.TokenUsage

	attempt := 0
	for {
		codeResponse, usage, err = ia.claude.GenerateCode(task, repoContext, language, state.Conversation)
		if err == nil {
			// Success!
			break
		}

		// Check if it's a rate limit error
		isRateLimit := strings.Contains(err.Error(), "429") ||
			strings.Contains(strings.ToLower(err.Error()), "rate limit") ||
			strings.Contains(strings.ToLower(err.Error()), "rate-limit")

		if !isRateLimit {
			// Non-rate-limit error, fail immediately
			return fmt.Errorf("failed to generate code: %w", err)
		}

		// Calculate wait duration (cap at maxBackoff for attempts >= 3)
		var waitDuration time.Duration
		if attempt < len(backoffDurations) {
			waitDuration = backoffDurations[attempt]
		} else {
			waitDuration = maxBackoff
		}

		attempt++
		fmt.Printf("⏳ Rate limit hit, waiting %v before retry (attempt %d)...\n", waitDuration, attempt+1)
		time.Sleep(waitDuration)
		fmt.Printf("🔄 Retrying code generation (attempt %d)...\n", attempt+1)
	}

	fmt.Printf("✅ Code generated successfully\n")

	// Track token usage
	state.TotalInputTokens += usage.InputTokens
	state.TotalOutputTokens += usage.OutputTokens
	state.TotalCost += usage.Cost

	// Parse the code response and extract file changes
	fileChanges := parseCodeChanges(codeResponse)

	// Apply the changes to the branch
	if len(fileChanges) > 0 {
		fmt.Printf("📝 Applying %d file change(s) to branch %s\n", len(fileChanges), branchName)
		for filePath, content := range fileChanges {
			fmt.Printf("  - Updating %s\n", filePath)
			if err := ia.github.CreateOrUpdateFile(owner, repo, filePath, fmt.Sprintf("Update %s for issue #%d", filePath, issueNumber), content, branchName, nil); err != nil {
				return fmt.Errorf("failed to update file %s: %w", filePath, err)
			}
		}
	} else {
		fmt.Printf("⚠️  Warning: No file changes detected from AI response\n")
	}

	// Create PR or comment about direct commit
	issue, err := ia.github.GetIssue(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	// If we committed directly to main (empty repo), just comment on the issue
	if branchName == defaultBranch {
		fmt.Printf("✅ Changes committed directly to %s (empty repository)\n", defaultBranch)
		state.Status = "completed"
		if err := ia.stateManager.SaveState(state); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		comment := fmt.Sprintf("✅ I've committed the changes directly to the `%s` branch since the repository was empty.\n\n%s\n\nClosing this issue as completed.\n\n---\n\n🤖 Changes made by NyteBubo", defaultBranch, codeResponse)
		if err := ia.github.CreateIssueComment(owner, repo, issueNumber, comment); err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}

		// Close the issue
		closed := "closed"
		issueUpdate := &github.IssueRequest{State: &closed}
		if _, _, err := ia.github.GetClient().Issues.Edit(ia.github.GetContext(), owner, repo, issueNumber, issueUpdate); err != nil {
			fmt.Printf("⚠️  Warning: failed to close issue: %v\n", err)
		}

		return nil
	}

	// Normal PR flow
	prTitle := fmt.Sprintf("Fix: %s", issue.GetTitle())
	prBody := fmt.Sprintf("Fixes #%d\n\n%s\n\n---\n\n🤖 This PR was automatically generated by NyteBubo", issueNumber, codeResponse)

	fmt.Printf("📬 Creating pull request...\n")
	pr, err := ia.github.CreatePullRequest(owner, repo, prTitle, prBody, branchName, defaultBranch)
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}
	fmt.Printf("✅ Pull request #%d created successfully!\n", pr.GetNumber())

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
	state.TotalCost += usage.Cost

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

	// Start polling and handle events
	handlers := core.PollerHandlers{
		HandleIssue: func(owner, repo string, issueNumber int) error {
			return ia.HandleIssueAssignment(owner, repo, issueNumber)
		},
		HandleIssueComment: func(owner, repo string, issueNumber int, commentBody string) error {
			return ia.HandleIssueComment(owner, repo, issueNumber, commentBody)
		},
		HandlePRComment: func(owner, repo string, prNumber int, commentBody string) error {
			return ia.HandlePRComment(owner, repo, prNumber, commentBody)
		},
		HandleImplementation: func(owner, repo string, issueNumber int) error {
			return ia.StartImplementation(owner, repo, issueNumber)
		},
	}

	return poller.Start(handlers)
}
