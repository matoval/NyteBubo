# NyteBubo

An AI-powered GitHub assistant that automatically analyzes issues, asks clarifying questions, and creates pull requests using Claude AI.

## Overview

NyteBubo is a Go-based CLI tool and server that integrates with GitHub to provide an intelligent agent that:

1. **Analyzes Issues**: When assigned to a GitHub issue, the agent reads and understands what needs to be done
2. **Asks Questions**: If anything is unclear, it comments on the issue asking for clarification
3. **Creates PRs**: Once it understands the task, it automatically creates a pull request with the implementation
4. **Responds to Feedback**: When you review the PR with comments or suggestions, the agent updates the code accordingly

## Features

- GitHub webhook integration for real-time event processing
- Claude AI integration using the Anthropic SDK
- Persistent conversation state management with SQLite
- Interactive CLI for project initialization
- Secure credential management via environment variables
- Comprehensive logging and error handling

## Prerequisites

- Go 1.22 or higher
- GitHub account with repository access
- Anthropic Claude API key
- GitHub Personal Access Token with repo permissions

## Installation

### Build from Source

```bash
git clone https://github.com/matoval/NyteBubo.git
cd NyteBubo
go build -o nyte-bubo
```

### Install Binary

```bash
go install github.com/matoval/NyteBubo@latest
```

## Quick Start

### 1. Initialize Your Project

Run the interactive initialization wizard:

```bash
./nyte-bubo init
```

This will prompt you for:
- Project name
- Git remote URL
- Programming language
- Frameworks (comma-separated)

A `config.yaml` file will be created with your project configuration.

### 2. Configure the Agent

Edit `config.yaml` and enable the agent:

```yaml
agent:
  enabled: true
  server_port: 8080
  working_dir: "./workspace"
  state_db_path: "./agent_state.db"
```

### 3. Set Environment Variables

For security, set your API credentials as environment variables:

```bash
export CLAUDE_API_KEY="your-claude-api-key"
export GITHUB_TOKEN="your-github-personal-access-token"
export WEBHOOK_SECRET="your-webhook-secret"  # Optional but recommended
```

### 4. Start the Agent Server

```bash
./nyte-bubo agent
```

The server will start on the configured port (default: 8080).

### 5. Configure GitHub Webhook

In your GitHub repository:

1. Go to **Settings > Webhooks > Add webhook**
2. Set **Payload URL** to: `http://your-server:8080/webhook`
3. Set **Content type** to: `application/json`
4. Set **Secret** to your `WEBHOOK_SECRET` (if configured)
5. Select these events:
   - Issues
   - Issue comments
   - Pull request review comments
6. Click **Add webhook**

## How It Works

### Workflow

1. **Issue Assignment**:
   - Assign the bot to a GitHub issue
   - The agent receives a webhook event
   - Claude analyzes the issue and posts a comment with its understanding
   - If anything is unclear, it asks clarifying questions

2. **Clarification** (if needed):
   - You respond to the agent's questions in the issue comments
   - The agent processes your responses and asks follow-ups if needed
   - This continues until the agent has a clear understanding

3. **Implementation**:
   - Once ready, the agent creates a new branch
   - Claude generates the necessary code changes
   - The agent creates a pull request with the implementation
   - Links the PR to the original issue

4. **Code Review**:
   - You review the PR and leave comments or suggestions
   - The agent processes your feedback
   - Claude generates updated code based on your comments
   - The agent pushes the changes to the PR branch

### Architecture

```
┌─────────────┐
│   GitHub    │
│  Webhooks   │
└──────┬──────┘
       │
       v
┌─────────────────────────────────┐
│  NyteBubo Webhook Server         │
│  ┌─────────────────────────┐    │
│  │  Event Handler          │    │
│  └───────────┬─────────────┘    │
│              v                   │
│  ┌─────────────────────────┐    │
│  │  Issue Agent Workflow   │    │
│  └───────────┬─────────────┘    │
│              v                   │
│  ┌──────────────┬──────────┐    │
│  │ GitHub API   │ Claude AI│    │
│  │ Client       │ Client   │    │
│  └──────────────┴──────────┘    │
│              v                   │
│  ┌─────────────────────────┐    │
│  │  State Manager (SQLite) │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
```

## Project Structure

```
NyteBubo/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command
│   ├── init.go            # Init command
│   └── agent.go           # Agent server command
├── internal/
│   ├── cli/               # CLI logic
│   │   └── init.go        # Interactive init prompts
│   ├── core/              # Core functionality
│   │   ├── github.go      # GitHub API client
│   │   ├── claude.go      # Claude AI client
│   │   └── state.go       # State management
│   ├── types/             # Type definitions
│   │   └── config.go      # Configuration types
│   ├── templates/         # Templates
│   │   └── init.tmpl      # Config template
│   └── workflows/         # Agent workflows
│       └── issue_to_pr.go # Issue-to-PR workflow
├── server/                # Server implementation
│   ├── main.go           # Server entry point
│   └── webhook.go        # Webhook handlers
├── main.go               # Application entry point
├── config.yaml           # Configuration file (generated)
├── go.mod                # Go module definition
└── README.md             # This file
```

## Configuration

### config.yaml

```yaml
project_name: YourProject
git_remote: https://github.com/matoval/yourrepo
programming_language: Go
frameworks:
  - Cobra
  - HTTP

agent:
  enabled: true
  server_port: 8080
  working_dir: "./workspace"
  state_db_path: "./agent_state.db"
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `CLAUDE_API_KEY` | Your Anthropic Claude API key | Yes |
| `GITHUB_TOKEN` | GitHub Personal Access Token with repo access | Yes |
| `WEBHOOK_SECRET` | Secret for webhook signature verification | Recommended |

## Security Best Practices

1. **Never commit credentials**: Use environment variables for API keys and tokens
2. **Set webhook secret**: Always configure a webhook secret to verify GitHub events
3. **Use HTTPS**: In production, use HTTPS for your webhook endpoint
4. **Restrict bot permissions**: Create a dedicated GitHub user for the bot with minimal required permissions
5. **Review PRs carefully**: Always review agent-generated code before merging

## API Usage

### GitHub API

The agent uses the following GitHub API endpoints:
- Get issue details
- Create and list issue comments
- Create branches and pull requests
- Get and update file contents
- List PR comments

### Claude AI API

The agent uses Claude 3.7 Sonnet Latest model with:
- Conversational message API
- System prompts for context
- Multi-turn conversations for clarification
- Code generation capabilities

## Troubleshooting

### Webhook not receiving events

1. Check that your server is publicly accessible
2. Verify webhook configuration in GitHub
3. Check webhook delivery logs in GitHub Settings
4. Ensure firewall allows incoming connections on the server port

### Agent not responding

1. Check server logs for errors
2. Verify API credentials are set correctly
3. Check `agent_state.db` for conversation state
4. Ensure Claude API key has sufficient quota

### Build errors

```bash
# Clean and rebuild
go clean
go mod tidy
go build -o nyte-bubo
```

## Development

### Running Tests

```bash
go test ./...
```

### Adding New Workflows

1. Create a new file in `internal/workflows/`
2. Implement the workflow interface
3. Add webhook handlers in `server/webhook.go`
4. Update the agent command in `cmd/agent.go`

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

[Add your license here]

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI
- Uses [Tap](https://github.com/yarlson/tap) for interactive prompts
- Powered by [Claude AI](https://www.anthropic.com/claude) from Anthropic
- GitHub integration via [go-github](https://github.com/google/go-github)
