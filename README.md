<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/go-logo.png" alt="Go Discord Bots Monorepo Logo" width="400" />
</p>

<p align="center">
  <a href="https://github.com/dunamismax/go-discord-bots">
    <img src="https://readme-typing-svg.demolab.com/?font=Fira+Code&size=24&pause=1000&color=00ADD8&center=true&vCenter=true&width=900&lines=Discord+Bot+Monorepo+in+Go;Magic+The+Gathering+Card+Lookup+Bot;Scryfall+API+Integration+with+Rate+Limiting;Rich+Discord+Embeds+with+Card+Images;Fuzzy+Search+and+Random+Card+Features;Auto-Restart+Development+with+Mage;Multi-Bot+Concurrent+Execution;Production-Ready+Build+System;Environment+Configuration+Management;Single+Binary+Deployments" alt="Typing SVG" />
  </a>
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go" alt="Go Version"></a>
  <a href="https://github.com/bwmarrin/discordgo"><img src="https://img.shields.io/badge/Discord-DiscordGo-5865F2.svg?logo=discord&logoColor=white" alt="DiscordGo"></a>
  <a href="https://scryfall.com/docs/api"><img src="https://img.shields.io/badge/API-Scryfall-FF6B35.svg" alt="Scryfall API"></a>
  <a href="https://magefile.org/"><img src="https://img.shields.io/badge/Build-Mage-purple.svg?logo=go" alt="Mage"></a>
  <a href="https://pkg.go.dev/log/slog"><img src="https://img.shields.io/badge/Logging-slog-00ADD8.svg?logo=go" alt="Go slog"></a>
  <a href="https://github.com/spf13/viper"><img src="https://img.shields.io/badge/Config-Environment-00ADD8.svg?logo=go" alt="Environment Config"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="MIT License"></a>
</p>

---

## About

A production-ready monorepo for Discord bots built with **Modern Go Architecture** - designed for scalability, maintainability, and easy deployment. Create multiple specialized Discord bots that can run individually or concurrently from a single codebase.

**Key Features:**

- **MTG Card Bot**: Magic: The Gathering card lookup with Scryfall API integration and fuzzy search
- **Rich Discord Embeds**: High-quality card images with rarity-based colors and comprehensive card information
- **Monorepo Architecture**: Shared packages for configuration, Discord utilities, and API clients
- **Rate Limiting**: Respectful API usage with 10 requests/second limitation as per Scryfall guidelines
- **Development Tools**: Hot reload with auto-restart, comprehensive build system, and quality checks
- **Environment Management**: Multi-source configuration with .env file support and validation
- **Production Ready**: Single binary deployment, structured logging, and graceful shutdown handling

## Tech Stack

| Layer          | Technology                                                  | Purpose                                |
| -------------- | ----------------------------------------------------------- | -------------------------------------- |
| **Language**   | [Go 1.24+](https://go.dev/doc/)                             | Latest performance & language features |
| **Discord API**| [DiscordGo](https://github.com/bwmarrin/discordgo)         | Robust Discord bot framework          |
| **Card API**   | [Scryfall API](https://scryfall.com/docs/api)             | Comprehensive MTG card database        |
| **Logging**    | [slog](https://pkg.go.dev/log/slog)                         | Structured logging with JSON output    |
| **Config**     | [Environment Variables](https://pkg.go.dev/os)              | Simple, secure configuration management |
| **Build**      | [Mage](https://magefile.org/)                               | Go-based build automation              |
| **Testing**    | [Go Testing](https://pkg.go.dev/testing)                    | Built-in testing framework             |
| **Assets**     | [Go Embed](https://pkg.go.dev/embed)                        | Single binary with embedded resources  |

---

## Quick Start

### Discord Bot Setup

```bash
# Clone repository
git clone https://github.com/dunamismax/go-discord-bots.git
cd go-discord-bots

# Install dependencies
go mod tidy

# Install Mage build tool
go install github.com/magefile/mage@latest

# Create your environment file
cp .env.example .env
# Edit .env with your Discord bot token from https://discord.com/developers/applications

# Install development tools
mage setup

# Start MTG card bot in development mode
mage dev mtg-card-bot

# Bot is now running! Test with: !lightning bolt
```

**Requirements:** Go 1.24+, Discord Bot Token

### Production Deployment

```bash
# Build optimized binary
mage build

# Deploy single binary (includes embedded assets)
# Binary available at: bin/mtg-card-bot (~6MB)

# Run in production with environment variables
DISCORD_TOKEN=your_token ./bin/mtg-card-bot
```

---

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/gopher-mage.svg" alt="Gopher Mage" width="150" />
</p>

## Mage Commands

Run `mage help` to see all available commands and their aliases.

**Development:**

```bash
mage setup (s)        # Install development tools and dependencies
mage dev <bot>        # Run bot in development mode with auto-restart
mage devAll           # Run all bots in development mode
mage run <bot>        # Build and run specific bot
mage runAll           # Run all bots concurrently
mage build (b)        # Build all Discord bot binaries
```

**Quality & Testing:**

```bash
mage fmt (f)          # Format code with goimports and tidy modules
mage vet (v)          # Run go vet static analysis
mage lint (l)         # Run golangci-lint comprehensive linting
mage test (t)         # Run all tests
mage testCoverage     # Run tests with HTML coverage report
mage vulncheck (vc)   # Check for security vulnerabilities
mage quality (q)      # Run all quality checks
mage ci               # Complete CI pipeline
```

**Utilities:**

```bash
mage listBots         # List all available Discord bots
mage status           # Show development environment status
mage clean (c)        # Clean build artifacts and temporary files
mage reset            # Reset repository to fresh state
```

## MTG Card Bot Features

### Interactive Commands

```bash
# Card lookup with fuzzy matching
!lightning bolt        # Finds "Lightning Bolt"
!the-one-ring         # Finds "The One Ring"
!jac bele             # Finds "Jace Beleren" (fuzzy search)

# Random card discovery
!random               # Get a random Magic: The Gathering card

# Examples of fuzzy matching
!counterspell         # Exact match
!counter              # Finds "Counterspell"
!force will           # Finds "Force of Will"
!ancestral            # Finds "Ancestral Recall"
```

### Rich Discord Embeds

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/discord-bots/the-one-ring.jpg" alt="MTG Card Embed Example" width="300" />
</p>

**Features:**

- **High-Quality Images**: Automatically selects highest resolution available (PNG > Large > Normal > Small)
- **Rarity Colors**: Dynamic embed colors based on card rarity (Mythic: Orange, Rare: Gold, etc.)
- **Comprehensive Info**: Card name, mana cost, type line, set information, and artist attribution
- **Direct Links**: Clickable links to Scryfall card pages for detailed information
- **Error Handling**: Helpful messages for card not found or API issues

### API Integration Details

**Scryfall API Compliance:**

- Rate limited to 10 requests per second (as recommended by Scryfall)
- Proper User-Agent headers and Accept headers
- Fuzzy search with fallback to exact matching
- Image optimization with quality preference
- Full attribution to artists and copyright holders
- Compliant with Wizards of the Coast Fan Content Policy

## Project Structure

```sh
go-discord-bots/
├── bots/                     # Individual bot implementations
│   └── mtg-card-bot/
│       └── main.go          # MTG card bot entry point
├── pkg/                     # Shared packages for all bots
│   ├── config/              # Environment configuration management
│   ├── discord/             # Discord bot utilities and embed handling
│   └── scryfall/            # Scryfall API client with rate limiting
├── bin/                     # Compiled binaries (gitignored)
├── magefile.go             # Mage build automation with comprehensive commands
├── go.mod/go.sum           # Go module dependencies
├── .env.example            # Environment configuration template
└── .golangci.yml           # Linter configuration (if needed)
```

---

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/gopher-aviator.jpg" alt="Go Gopher" width="400" />
</p>

## Environment Configuration

### Required Variables

```bash
# Discord Configuration
DISCORD_TOKEN=your_discord_bot_token_here    # Get from Discord Developer Portal

# Optional Configuration (with defaults)
COMMAND_PREFIX=!                             # Bot command prefix
LOG_LEVEL=info                              # Logging level: debug, info, warn, error
BOT_NAME=mtg-card-bot                       # Bot identifier for logging
```

### Configuration Management

The bot supports multiple configuration sources with the following priority:

1. **System Environment Variables** (highest priority)
2. **`.env` File** (if present)
3. **Default Values** (built-in fallbacks)

```go
// Example configuration usage
cfg, err := config.Load()
if err != nil {
    log.Fatal("Configuration error:", err)
}

// Automatic validation ensures required values are present
if err := cfg.Validate(); err != nil {
    log.Fatal("Invalid configuration:", err)
}
```

## Adding New Discord Bots

### 1. Create Bot Structure

```bash
# Create new bot directory
mkdir bots/my-awesome-bot

# Create main.go file
cat > bots/my-awesome-bot/main.go << 'EOF'
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/dunamismax/go-discord-bots/pkg/config"
    "github.com/dunamismax/go-discord-bots/pkg/discord"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    
    // Create your custom bot using the discord package
    bot, err := discord.NewBot(cfg, nil) // Add your own client here
    if err != nil {
        log.Fatalf("Failed to create bot: %v", err)
    }
    
    if err := bot.Start(); err != nil {
        log.Fatalf("Failed to start bot: %v", err)
    }
    
    // Wait for interrupt
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop
    
    bot.Stop()
}
EOF
```

### 2. Automatic Detection

The Magefile automatically detects new bots:

```bash
# List all available bots
mage listBots

# Your new bot will appear in the list
Available Discord bots:
  1. mtg-card-bot
  2. my-awesome-bot

Total: 2 bot(s)

# Run your new bot
mage dev my-awesome-bot
```

### 3. Shared Package Usage

Leverage existing shared packages:

```go
// Use the configuration package
cfg, _ := config.Load()

// Use Discord utilities
bot, _ := discord.NewBot(cfg, yourAPIClient)

// Create custom API clients following the scryfall package pattern
type YourAPIClient struct {
    httpClient   *http.Client
    rateLimiter  <-chan time.Time
}
```

---

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/discord/mtg-cards-showcase.png" alt="MTG Cards Showcase" width="800" />
</p>

## Production Deployment

### Single Binary Deployment

```bash
# Build optimized binary for production
mage build

# Binary includes embedded assets and is ready to deploy
ls -la bin/
# mtg-card-bot: ~6MB (includes all dependencies)

# Deploy anywhere with just the binary and environment variables
DISCORD_TOKEN=your_token ./bin/mtg-card-bot
```

### Systemd Service (Ubuntu)

```bash
# Create systemd service file
sudo tee /etc/systemd/system/mtg-card-bot.service > /dev/null << EOF
[Unit]
Description=MTG Card Discord Bot
After=network.target

[Service]
Type=simple
User=bot
WorkingDirectory=/opt/mtg-card-bot
ExecStart=/opt/mtg-card-bot/bin/mtg-card-bot
Restart=always
RestartSec=10
Environment=DISCORD_TOKEN=your_token_here
Environment=LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable mtg-card-bot
sudo systemctl start mtg-card-bot
```

### Docker Deployment

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o mtg-card-bot ./bots/mtg-card-bot/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/mtg-card-bot .
CMD ["./mtg-card-bot"]
```

## Key Features Demonstrated

**Modern Discord Bot Architecture:**

- DiscordGo integration with comprehensive event handling and message processing
- Rich embed creation with dynamic colors, images, and interactive components
- Rate-limited API clients with proper error handling and retry logic
- Structured logging with correlation IDs and request tracing
- Environment-based configuration with validation and sensible defaults

**Developer Experience:**

- Hot reloading with automatic restart on crash (max 10 restarts for safety)
- Comprehensive error handling with structured logging and monitoring
- Static analysis suite with golangci-lint, govulncheck, and go vet
- Mage build automation with quality checks and vulnerability scanning
- Single-command CI pipeline with formatting, linting, and testing

**Production Ready:**

- Single binary deployment with embedded assets (~6MB for MTG bot)
- Graceful shutdown handling with proper cleanup of Discord connections
- Multi-bot concurrent execution with individual process management
- Comprehensive middleware for logging, error handling, and monitoring
- Rate limiting and API compliance with external service guidelines

---

<p align="center">
  <a href="https://buymeacoffee.com/dunamismax" target="_blank">
    <img src="https://github.com/dunamismax/images/blob/main/golang/buy-coffee-go.gif" alt="Buy Me A Coffee" style="height: 150px !important;" />
  </a>
</p>

<p align="center">
  <a href="https://twitter.com/dunamismax" target="_blank"><img src="https://img.shields.io/badge/Twitter-%231DA1F2.svg?&style=for-the-badge&logo=twitter&logoColor=white" alt="Twitter"></a>
  <a href="https://bsky.app/profile/dunamismax.bsky.social" target="_blank"><img src="https://img.shields.io/badge/Bluesky-blue?style=for-the-badge&logo=bluesky&logoColor=white" alt="Bluesky"></a>
  <a href="https://reddit.com/user/dunamismax" target="_blank"><img src="https://img.shields.io/badge/Reddit-%23FF4500.svg?&style=for-the-badge&logo=reddit&logoColor=white" alt="Reddit"></a>
  <a href="https://discord.com/users/dunamismax" target="_blank"><img src="https://img.shields.io/badge/Discord-dunamismax-7289DA.svg?style=for-the-badge&logo=discord&logoColor=white" alt="Discord"></a>
  <a href="https://signal.me/#p/+dunamismax.66" target="_blank"><img src="https://img.shields.io/badge/Signal-dunamismax.66-3A76F0.svg?style=for-the-badge&logo=signal&logoColor=white" alt="Signal"></a>
</p>

## License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  <strong>Modern Go Discord Bot Architecture</strong><br>
  <sub>DiscordGo • Scryfall API • Mage • slog • Environment Config • Rate Limiting • Rich Embeds</sub>
</p>

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/gopher-running-jumping.gif" alt="Gopher Running and Jumping" width="600" />
</p>

---

"Discord bots built with Go offer exceptional performance, reliability, and deployment simplicity. This monorepo architecture provides a solid foundation for scaling from simple utility bots to complex multi-bot ecosystems." - Me

---
