<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/go-logo.png" alt="Go Discord Bots Monorepo Logo" width="400" />
</p>

<p align="center">
  <a href="https://github.com/dunamismax/go-discord-bots">
    <img src="https://readme-typing-svg.demolab.com/?font=Fira+Code&size=24&pause=1000&color=00ADD8&center=true&vCenter=true&width=900&lines=Discord+Bot+Monorepo+in+Go;Magic+The+Gathering+Card+Lookup+Bot;Scryfall+API+Integration+with+Rate+Limiting;Rich+Discord+Embeds+with+Card+Images;Fuzzy+Search+and+Random+Card+Features;Auto-Restart+Development+with+Mage;Multi-Bot+Concurrent+Execution;Flexible+Build+System;Environment+Configuration+Management;Single+Binary+Deployments" alt="Typing SVG" />
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

A Go-based monorepo for creating and running multiple Discord bots from one codebase. Built with a focus on clean architecture, easy maintenance, and quick deployment.

You can run each bot on its own or several at once. The first example bot is a Magic: The Gathering card lookup tool using the Scryfall API.

**Highlights:**

* **MTG Card Bot** – Look up cards with fuzzy search, random card feature, and rich embeds
* **Nice-looking Embeds** – Card images, rarity colors, and detailed info
* **Monorepo Setup** – Shared packages for config, Discord helpers, and API clients
* **Respectful API Use** – Built-in Scryfall rate limiting (10 requests/sec)
* **Handy Dev Tools** – Auto-restart, build scripts, quality checks
* **Config Management** – Environment variables, `.env` support, and validation
* **Easy Deployment** – Single-binary builds with structured logging and graceful shutdown

---

## Quick Start

```bash
git clone https://github.com/dunamismax/go-discord-bots.git
cd go-discord-bots
go mod tidy
go install github.com/magefile/mage@latest
cp .env.example .env  # Add your Discord bot token
mage setup
mage dev mtg-card-bot
```

**Requirements:** Go 1.24+, Discord Bot Token

---

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/gopher-mage.svg" alt="Gopher Mage" width="150" />
</p>

## Mage Commands

```bash
mage setup         # Install dev tools
mage dev <bot>     # Run bot with auto-restart
mage devAll        # Run all bots
mage build         # Build binaries
mage fmt / lint    # Format & lint checks
mage test          # Run tests
mage vulncheck     # Security check
```

---

## MTG Card Bot Commands

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

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/discord-bots/the-one-ring.jpg" alt="MTG Card Embed Example" width="300" />
</p>

---

## Adding New Bots

1. Make a folder in `bots/` and add a `main.go`.
2. Use shared packages from `pkg/` for config and Discord helpers.
3. Run `mage listBots` to confirm it’s detected.
4. Launch it with `mage dev your-bot-name`.

---

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/discord/mtg-cards-showcase.png" alt="MTG Cards Showcase" width="800" />
</p>

## Deployment Options

* **Single Binary** – Build with `mage build`, copy the file, and run with env vars.
* **Systemd** – Create a service to keep it running on Linux.
* **Docker** – Lightweight container build included.

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

MIT – see [LICENSE](LICENSE) for details.

---

<p align="center">
  <strong>Go Discord Bot Monorepo</strong><br>
  <sub>DiscordGo • Scryfall API • Mage • slog • Config • Rate Limiting • Rich Embeds</sub>
</p>

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/gopher-running-jumping.gif" alt="Gopher Running and Jumping" width="600" />
</p>

---
