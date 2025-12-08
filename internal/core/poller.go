package core

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/go-github/v63/github"
)

// PollerHandlers contains callbacks for different event types
type PollerHandlers struct {
	HandleIssue            func(owner, repo string, issueNumber int) error
	HandleIssueComment     func(owner, repo string, issueNumber int, commentBody string) error
	HandlePRComment        func(owner, repo string, prNumber int, commentBody string) error
	HandleImplementation   func(owner, repo string, issueNumber int) error
}

// Poller polls GitHub for assigned issues and triggers workflows
type Poller struct {
	github       *GitHubClient
	stateManager *StateManager
	pollInterval time.Duration
	repositories []string // List of repositories to monitor (format: "owner/repo")
	username     string   // Bot username
}

// PollerConfig contains configuration for the poller
type PollerConfig struct {
	PollInterval time.Duration
	Repositories []string
}

// NewPoller creates a new GitHub issue poller
func NewPoller(github *GitHubClient, stateManager *StateManager, config PollerConfig) (*Poller, error) {
	// Get the authenticated user
	user, err := github.GetAuthenticatedUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated user: %w", err)
	}

	return &Poller{
		github:       github,
		stateManager: stateManager,
		pollInterval: config.PollInterval,
		repositories: config.Repositories,
		username:     user.GetLogin(),
	}, nil
}

// Start begins polling for assigned issues
func (p *Poller) Start(handlers PollerHandlers) error {
	log.Printf("Starting poller for user: %s", p.username)
	log.Printf("Monitoring repositories: %v", p.repositories)
	log.Printf("Poll interval: %v", p.pollInterval)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Do an initial poll immediately
	if err := p.poll(handlers); err != nil {
		log.Printf("Error during initial poll: %v", err)
	}

	// Then poll at intervals
	for range ticker.C {
		if err := p.poll(handlers); err != nil {
			log.Printf("Error during poll: %v", err)
		}
	}

	return nil
}

// poll checks for new assigned issues and processes them
func (p *Poller) poll(handlers PollerHandlers) error {
	log.Printf("Polling for assigned issues...")

	for _, repoFullName := range p.repositories {
		// Parse owner/repo
		parts := strings.Split(repoFullName, "/")
		if len(parts) != 2 {
			log.Printf("Invalid repository format: %s (expected owner/repo)", repoFullName)
			continue
		}
		owner, repo := parts[0], parts[1]

		// Get assigned issues for this repository
		issues, err := p.github.ListRepositoryIssues(owner, repo, p.username)
		if err != nil {
			log.Printf("Failed to list issues for %s: %v", repoFullName, err)
			continue
		}

		log.Printf("Found %d assigned issue(s) in %s", len(issues), repoFullName)

		// Process each issue
		for _, issue := range issues {
			if err := p.processIssue(owner, repo, issue, handlers); err != nil {
				log.Printf("Error processing issue #%d in %s: %v", issue.GetNumber(), repoFullName, err)
			}
		}
	}

	return nil
}

// processIssue checks if an issue needs to be processed and handles it
func (p *Poller) processIssue(owner, repo string, issue *github.Issue, handlers PollerHandlers) error {
	issueNumber := issue.GetNumber()

	// Check if we've already processed this issue
	state, err := p.stateManager.GetState(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	// If we have no state for this issue, it's new - process it
	if state == nil {
		log.Printf("New issue detected: %s/%s #%d - %s", owner, repo, issueNumber, issue.GetTitle())
		if handlers.HandleIssue != nil {
			return handlers.HandleIssue(owner, repo, issueNumber)
		}
		return nil
	}

	// Reconcile status with latest comments (detect stuck states)
	if state.Status == "waiting_for_clarification" {
		log.Printf("🔍 Checking if issue %s/%s #%d needs status reconciliation", owner, repo, issueNumber)
		if err := p.reconcileStatus(owner, repo, issueNumber, state); err != nil {
			log.Printf("⚠️  Warning: failed to reconcile status for issue #%d: %v", issueNumber, err)
		}
		// Reload state after potential reconciliation
		state, err = p.stateManager.GetState(owner, repo, issueNumber)
		if err != nil {
			return fmt.Errorf("failed to reload state after reconciliation: %w", err)
		}
		log.Printf("📊 Issue %s/%s #%d status after reconciliation: %s", owner, repo, issueNumber, state.Status)
	}

	// If issue is ready to implement, start implementation
	if state.Status == "ready_to_implement" {
		log.Printf("Issue %s/%s #%d is ready to implement - starting implementation", owner, repo, issueNumber)
		if handlers.HandleImplementation != nil {
			return handlers.HandleImplementation(owner, repo, issueNumber)
		}
		return nil
	}

	// If issue is stuck in "implementing" status (failed during implementation), retry
	if state.Status == "implementing" {
		// Check how long it's been stuck (more than 10 minutes = definitely stuck)
		stuckDuration := time.Since(state.UpdatedAt)
		if stuckDuration > 10*time.Minute {
			log.Printf("⚠️  Issue %s/%s #%d stuck in 'implementing' for %v - retrying", owner, repo, issueNumber, stuckDuration)
			state.Status = "ready_to_implement"
			if err := p.stateManager.SaveState(state); err != nil {
				log.Printf("Error resetting stuck status: %v", err)
			}
			if handlers.HandleImplementation != nil {
				return handlers.HandleImplementation(owner, repo, issueNumber)
			}
		}
		return nil
	}

	// If we have state, check if there are new comments we need to process
	if state.Status == "waiting_for_clarification" {
		newComments, err := p.getNewComments(owner, repo, issueNumber, state)
		if err != nil {
			return fmt.Errorf("failed to check for new comments: %w", err)
		}

		if len(newComments) > 0 {
			log.Printf("New comments detected on issue %s/%s #%d - processing %d comment(s)", owner, repo, issueNumber, len(newComments))
			// Process each new comment
			for _, comment := range newComments {
				if handlers.HandleIssueComment != nil {
					if err := handlers.HandleIssueComment(owner, repo, issueNumber, comment.GetBody()); err != nil {
						log.Printf("Error handling comment on issue #%d: %v", issueNumber, err)
					}
				}
			}
		}
	}

	// Check if there are new PR review comments
	log.Printf("📊 Issue %s/%s #%d - Status: %s, PRNumber: %v", owner, repo, issueNumber, state.Status, state.PRNumber)

	if state.Status == "pr_created" || state.Status == "reviewing" {
		log.Printf("🔍 Checking for PR review comments (status allows it)")
		if state.PRNumber != nil {
			log.Printf("🔍 PR Number is set: #%d, checking for new review comments...", *state.PRNumber)
			newReviewComments, err := p.getNewPRComments(owner, repo, *state.PRNumber, state)
			if err != nil {
				log.Printf("❌ Error checking for new PR comments: %v", err)
				return fmt.Errorf("failed to check for new PR comments: %w", err)
			}

			log.Printf("📝 Found %d new PR review comment(s) on PR #%d", len(newReviewComments), *state.PRNumber)

			if len(newReviewComments) > 0 {
				log.Printf("New PR review comments detected on %s/%s #%d - processing %d comment(s)", owner, repo, *state.PRNumber, len(newReviewComments))
				// Process each new PR comment
				for _, comment := range newReviewComments {
					if handlers.HandlePRComment != nil {
						if err := handlers.HandlePRComment(owner, repo, *state.PRNumber, comment.GetBody()); err != nil {
							log.Printf("Error handling PR comment on #%d: %v", *state.PRNumber, err)
						}
					}
				}
			}
		} else {
			log.Printf("⚠️  PR Number is nil - cannot check for PR comments")
		}
	} else {
		log.Printf("⏭️  Skipping PR comment check (status is '%s', needs to be 'pr_created' or 'reviewing')", state.Status)
	}

	return nil
}

// reconcileStatus checks if the bot's last comment indicates readiness but status doesn't match
func (p *Poller) reconcileStatus(owner, repo string, issueNumber int, state *State) error {
	comments, err := p.github.ListIssueComments(owner, repo, issueNumber)
	if err != nil {
		return err
	}

	// Find the bot's last comment
	var lastBotComment *github.IssueComment
	for i := len(comments) - 1; i >= 0; i-- {
		if comments[i].GetUser().GetLogin() == p.username {
			lastBotComment = comments[i]
			break
		}
	}

	if lastBotComment == nil {
		return nil // No bot comments yet
	}

	// Check if the bot's last comment indicates readiness to implement
	commentBody := lastBotComment.GetBody()
	lowerComment := strings.ToLower(commentBody)

	previewLen := 100
	if len(commentBody) < previewLen {
		previewLen = len(commentBody)
	}
	log.Printf("🔍 Last bot comment (first 100 chars): %s", commentBody[:previewLen])

	indicatesReady := strings.Contains(lowerComment, "i'll create a pr") ||
		strings.Contains(lowerComment, "i will create a pr") ||
		strings.Contains(lowerComment, "i'll create the pr") ||
		strings.Contains(lowerComment, "i will create the pr") ||
		strings.Contains(lowerComment, "i'll start working") ||
		strings.Contains(lowerComment, "i will start working") ||
		strings.Contains(lowerComment, "proceeding with") ||
		strings.Contains(lowerComment, "i'll proceed") ||
		strings.Contains(lowerComment, "i will proceed")

	log.Printf("🔍 Indicates ready: %v, Current status: %s", indicatesReady, state.Status)

	if indicatesReady && state.Status == "waiting_for_clarification" {
		log.Printf("🔄 Reconciling status for issue %s/%s #%d: bot indicated readiness but status was waiting", owner, repo, issueNumber)
		state.Status = "ready_to_implement"
		return p.stateManager.SaveState(state)
	}

	return nil
}

// getNewComments returns new comments since last processing
func (p *Poller) getNewComments(owner, repo string, issueNumber int, state *State) ([]*github.IssueComment, error) {
	comments, err := p.github.ListIssueComments(owner, repo, issueNumber)
	if err != nil {
		return nil, err
	}

	var newComments []*github.IssueComment

	// Filter out bot's own comments and get new user comments
	for _, comment := range comments {
		// Skip if it's the bot's own comment
		if comment.GetUser().GetLogin() == p.username {
			continue
		}

		// Check if comment is newer than state update
		commentTime := comment.GetCreatedAt().Time
		if commentTime.After(state.UpdatedAt) {
			newComments = append(newComments, comment)
		}
	}

	return newComments, nil
}

// getNewPRComments returns new PR review comments since last processing
func (p *Poller) getNewPRComments(owner, repo string, prNumber int, state *State) ([]*github.PullRequestComment, error) {
	log.Printf("🔎 Fetching PR review comments for %s/%s #%d", owner, repo, prNumber)
	comments, err := p.github.ListPRComments(owner, repo, prNumber)
	if err != nil {
		return nil, err
	}

	log.Printf("📋 Total PR review comments found: %d", len(comments))

	var newComments []*github.PullRequestComment

	// Filter out bot's own comments and get new review comments
	for i, comment := range comments {
		commentUser := comment.GetUser().GetLogin()
		commentTime := comment.GetCreatedAt().Time
		commentBody := comment.GetBody()

		log.Printf("  Comment #%d: User=%s, Time=%v, Body=%s...",
			i+1, commentUser, commentTime,
			truncateString(commentBody, 50))

		// Skip if it's the bot's own comment
		if commentUser == p.username {
			log.Printf("    ⏭️  Skipping (bot's own comment)")
			continue
		}

		// Check if comment is newer than state update
		if commentTime.After(state.UpdatedAt) {
			log.Printf("    ✅ New comment (after %v)", state.UpdatedAt)
			newComments = append(newComments, comment)
		} else {
			log.Printf("    ⏭️  Skipping (too old - comment: %v, state: %v)", commentTime, state.UpdatedAt)
		}
	}

	log.Printf("📊 Filtered to %d new comment(s)", len(newComments))
	return newComments, nil
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
