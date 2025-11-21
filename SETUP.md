# NyteBubo Setup Guide

This guide will walk you through setting up NyteBubo for your GitHub repository.

## Prerequisites

Before you begin, make sure you have:

1. **Go 1.22+** installed
2. **A GitHub repository** where you want to use the agent
3. **An Anthropic API account** with credits

## Step 1: Get Your API Credentials

### Claude API Key

1. Go to https://console.anthropic.com/
2. Sign in or create an account
3. Navigate to API Keys section
4. Create a new API key
5. Copy and save the API key securely

### GitHub Personal Access Token

1. Go to https://github.com/settings/tokens
2. Click "Generate new token" > "Generate new token (classic)"
3. Give it a descriptive name (e.g., "NyteBubo Agent")
4. Select the following scopes:
   - `repo` (Full control of private repositories)
   - Optionally: `workflow` if you want the bot to trigger workflows
5. Click "Generate token"
6. Copy and save the token securely (you won't be able to see it again!)

### Webhook Secret (Recommended)

Generate a random secret for webhook verification:

```bash
openssl rand -hex 32
```

Copy and save this secret.

## Step 2: Build NyteBubo

```bash
# Clone the repository
git clone https://github.com/yourusername/NyteBubo.git
cd NyteBubo

# Install dependencies
go mod download

# Build the binary
go build -o nyte-bubo

# Verify the build
./nyte-bubo --help
```

## Step 3: Initialize Your Project

Run the interactive initialization:

```bash
./nyte-bubo init
```

Answer the prompts:
- **Project name**: Name of your project (e.g., "MyAwesomeApp")
- **Git remote**: Your repository URL (e.g., "https://github.com/yourusername/myrepo")
- **Programming language**: The main language of your project
- **Frameworks**: Comma-separated list of frameworks you use

This creates a `config.yaml` file.

## Step 4: Configure the Agent

Edit the generated `config.yaml`:

```yaml
project_name: MyAwesomeApp
git_remote: https://github.com/yourusername/myrepo
programming_language: Go
frameworks:
  - Cobra
  - HTTP

# Enable and configure the agent
agent:
  enabled: true                      # Set to true
  server_port: 8080                  # Port for webhook server
  working_dir: "./workspace"         # Directory for agent workspace
  state_db_path: "./agent_state.db"  # SQLite database path
```

## Step 5: Set Up Environment Variables

### Option A: Using .env file (Development)

1. Copy the example file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` and add your credentials:
   ```bash
   CLAUDE_API_KEY=sk-ant-...
   GITHUB_TOKEN=ghp_...
   WEBHOOK_SECRET=your-random-secret
   ```

3. Load the environment:
   ```bash
   source .env
   ```

### Option B: Export directly (Quick test)

```bash
export CLAUDE_API_KEY="sk-ant-..."
export GITHUB_TOKEN="ghp_..."
export WEBHOOK_SECRET="your-random-secret"
```

### Option C: Systemd service (Production)

Create `/etc/systemd/system/nyte-bubo.service`:

```ini
[Unit]
Description=NyteBubo GitHub Agent
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/NyteBubo
Environment="CLAUDE_API_KEY=sk-ant-..."
Environment="GITHUB_TOKEN=ghp_..."
Environment="WEBHOOK_SECRET=your-secret"
ExecStart=/path/to/NyteBubo/nyte-bubo agent
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Then:
```bash
sudo systemctl daemon-reload
sudo systemctl enable nyte-bubo
sudo systemctl start nyte-bubo
```

## Step 6: Start the Agent

```bash
./nyte-bubo agent
```

You should see:
```
╔═══════════════════════════════════════════════╗
║        NyteBubo Agent Server Starting         ║
╚═══════════════════════════════════════════════╝

Configuration:
  Project: MyAwesomeApp
  Server Port: 8080
  Working Directory: ./workspace
  State Database: ./agent_state.db

The agent is now listening for GitHub webhook events.
```

## Step 7: Expose Your Server (Production)

For production use, you need a publicly accessible endpoint.

### Option A: Use ngrok (Quick testing)

```bash
# Install ngrok
# Sign up at https://ngrok.com and get your auth token

ngrok http 8080
```

Copy the HTTPS URL (e.g., `https://abc123.ngrok.io`)

### Option B: Use a VPS with reverse proxy

1. Deploy to a VPS (DigitalOcean, AWS, etc.)
2. Set up Nginx or Caddy as a reverse proxy
3. Configure SSL/TLS with Let's Encrypt

Example Nginx config:
```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Step 8: Configure GitHub Webhook

1. Go to your repository on GitHub
2. Navigate to **Settings** > **Webhooks** > **Add webhook**
3. Fill in the form:
   - **Payload URL**: `https://your-server.com/webhook` (or ngrok URL)
   - **Content type**: `application/json`
   - **Secret**: Your `WEBHOOK_SECRET`
   - **Which events**: Select "Let me select individual events"
     - Check: Issues
     - Check: Issue comments
     - Check: Pull request review comments
   - **Active**: Checked
4. Click **Add webhook**

GitHub will send a ping event. Check the "Recent Deliveries" tab to verify it succeeded.

## Step 9: Create a Bot User (Recommended)

For best practices, create a dedicated GitHub user for the bot:

1. Create a new GitHub account (e.g., `myapp-bot`)
2. Add this user as a collaborator to your repository
3. Generate a Personal Access Token for this user
4. Use this token as your `GITHUB_TOKEN`

This way:
- PRs and comments are clearly from the bot
- You can easily identify and manage bot activity
- You can revoke access without affecting your personal account

## Step 10: Test the Agent

1. Create a new issue in your repository
2. Assign the bot user (or yourself if using your own token) to the issue
3. Watch for a comment from the agent analyzing the issue
4. Respond to any clarifying questions
5. Wait for the agent to create a PR
6. Review the PR and leave comments
7. Watch the agent update the code based on your feedback

## Troubleshooting

### Agent not responding to webhook

**Check server logs:**
```bash
# If running directly
./nyte-bubo agent

# If using systemd
journalctl -u nyte-bubo -f
```

**Verify webhook delivery:**
- Go to GitHub Settings > Webhooks
- Click on your webhook
- Check "Recent Deliveries"
- Look for error messages

**Common issues:**
- Server not publicly accessible
- Firewall blocking port 8080
- Invalid webhook secret
- Missing API credentials

### Authentication errors

**Verify environment variables:**
```bash
echo $CLAUDE_API_KEY
echo $GITHUB_TOKEN
```

**Check token permissions:**
- GitHub token needs `repo` scope
- Claude API key needs to be valid and have credits

### Database errors

**Reset the state database:**
```bash
rm agent_state.db
./nyte-bubo agent
```

### Building errors

**Clean and rebuild:**
```bash
go clean
go mod tidy
go build -o nyte-bubo
```

## Next Steps

- Read the [README.md](README.md) for detailed information
- Monitor the agent's behavior and adjust prompts in `internal/core/claude.go`
- Customize workflows in `internal/workflows/issue_to_pr.go`
- Add more event handlers in `server/webhook.go`

## Getting Help

- Check the [README.md](README.md) for common questions
- Review server logs for error messages
- Check GitHub webhook delivery logs
- Open an issue in the NyteBubo repository

## Security Checklist

Before going to production:

- [ ] Using environment variables (not config file) for credentials
- [ ] Webhook secret is set and verified
- [ ] Using HTTPS for webhook endpoint
- [ ] Using a dedicated bot user account
- [ ] Bot user has minimal required permissions
- [ ] `.env` file is in `.gitignore`
- [ ] Server logs don't expose credentials
- [ ] Regular rotation of API keys and tokens

---

Happy coding with NyteBubo!
