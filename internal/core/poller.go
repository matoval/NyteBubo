package core

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/go-github/v63/github"
)

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
func (p *Poller) Start(handleIssue func(owner, repo string, issueNumber int) error) error {
	log.Printf("Starting poller for user: %s", p.username)
	log.Printf("Monitoring repositories: %v", p.repositories)
	log.Printf("Poll interval: %v", p.pollInterval)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Do an initial poll immediately
	if err := p.poll(handleIssue); err != nil {
		log.Printf("Error during initial poll: %v", err)
	}

	// Then poll at intervals
	for range ticker.C {
		if err := p.poll(handleIssue); err != nil {
			log.Printf("Error during poll: %v", err)
		}
	}

	return nil
}

// poll checks for new assigned issues and processes them
func (p *Poller) poll(handleIssue func(owner, repo string, issueNumber int) error) error {
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
			if err := p.processIssue(owner, repo, issue, handleIssue); err != nil {
				log.Printf("Error processing issue #%d in %s: %v", issue.GetNumber(), repoFullName, err)
			}
		}
	}

	return nil
}

// processIssue checks if an issue needs to be processed and handles it
func (p *Poller) processIssue(owner, repo string, issue *github.Issue, handleIssue func(owner, repo string, issueNumber int) error) error {
	issueNumber := issue.GetNumber()

	// Check if we've already processed this issue
	state, err := p.stateManager.GetState(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	// If we have no state for this issue, it's new - process it
	if state == nil {
		log.Printf("New issue detected: %s/%s #%d - %s", owner, repo, issueNumber, issue.GetTitle())
		return handleIssue(owner, repo, issueNumber)
	}

	// If we have state, check if there are new comments we need to process
	if state.Status == "waiting_for_clarification" {
		hasNewComments, err := p.checkForNewComments(owner, repo, issueNumber, state)
		if err != nil {
			return fmt.Errorf("failed to check for new comments: %w", err)
		}

		if hasNewComments {
			log.Printf("New comments detected on issue %s/%s #%d", owner, repo, issueNumber)
			// We'll need to handle this in the workflow layer
			// For now, just log it - the workflow will need to be updated to handle polling
		}
	}

	// Check if there are new PR review comments
	if state.Status == "pr_created" || state.Status == "reviewing" {
		if state.PRNumber != nil {
			hasNewReviewComments, err := p.checkForNewPRComments(owner, repo, *state.PRNumber, state)
			if err != nil {
				return fmt.Errorf("failed to check for new PR comments: %w", err)
			}

			if hasNewReviewComments {
				log.Printf("New PR review comments detected on %s/%s #%d", owner, repo, *state.PRNumber)
				// We'll need to handle this in the workflow layer
			}
		}
	}

	return nil
}

// checkForNewComments checks if there are new comments since last processing
func (p *Poller) checkForNewComments(owner, repo string, issueNumber int, state *State) (bool, error) {
	comments, err := p.github.ListIssueComments(owner, repo, issueNumber)
	if err != nil {
		return false, err
	}

	// Filter out bot's own comments and check for new user comments
	for _, comment := range comments {
		// Skip if it's the bot's own comment
		if comment.GetUser().GetLogin() == p.username {
			continue
		}

		// Check if this comment was made after our last conversation update
		commentTime := comment.GetCreatedAt().Time

		// Simple check: if there are more comments than conversation messages, there are new ones
		// This is a simple heuristic - in production you'd want to track last processed comment ID
		if len(comments) > len(state.Conversation) {
			return true, nil
		}

		// Check if comment is newer than state update
		if commentTime.After(state.UpdatedAt) {
			return true, nil
		}
	}

	return false, nil
}

// checkForNewPRComments checks if there are new PR review comments since last processing
func (p *Poller) checkForNewPRComments(owner, repo string, prNumber int, state *State) (bool, error) {
	comments, err := p.github.ListPRComments(owner, repo, prNumber)
	if err != nil {
		return false, err
	}

	// Filter out bot's own comments and check for new review comments
	for _, comment := range comments {
		// Skip if it's the bot's own comment
		if comment.GetUser().GetLogin() == p.username {
			continue
		}

		// Check if comment is newer than state update
		commentTime := comment.GetCreatedAt().Time
		if commentTime.After(state.UpdatedAt) {
			return true, nil
		}
	}

	return false, nil
}
