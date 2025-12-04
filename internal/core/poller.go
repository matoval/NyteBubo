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
	HandleIssue        func(owner, repo string, issueNumber int) error
	HandleIssueComment func(owner, repo string, issueNumber int, commentBody string) error
	HandlePRComment    func(owner, repo string, prNumber int, commentBody string) error
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
	if state.Status == "pr_created" || state.Status == "reviewing" {
		if state.PRNumber != nil {
			newReviewComments, err := p.getNewPRComments(owner, repo, *state.PRNumber, state)
			if err != nil {
				return fmt.Errorf("failed to check for new PR comments: %w", err)
			}

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
		}
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
	comments, err := p.github.ListPRComments(owner, repo, prNumber)
	if err != nil {
		return nil, err
	}

	var newComments []*github.PullRequestComment

	// Filter out bot's own comments and get new review comments
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
