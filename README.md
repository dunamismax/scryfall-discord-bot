<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/discord-bots/mtg.png" alt="MTG" width="300" />
</p>

<p align="center">
  <a href="https://github.com/dunamismax/MTG-Card-Bot">
    <img src="https://readme-typing-svg.demolab.com/?font=Fira+Code&size=24&pause=1000&color=00ADD8&center=true&vCenter=true&width=900&lines=Magic+The+Gathering+Card+Lookup+Bot;Discord+Bot+in+Go;Scryfall+API+Integration+with+Rate+Limiting;Rich+Discord+Embeds+with+Card+Images;Fuzzy+Search+and+Random+Card+Features;Auto-Restart+Development+with+Mage;Environment+Configuration+Management;Single+Binary+Deployments" alt="Typing SVG" />
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

A dedicated Magic: The Gathering card lookup Discord bot built in Go. Features fuzzy search, random card discovery, and rich embeds powered by the Scryfall API.

**Highlights:**

* **Fuzzy Card Search** – Find cards with partial names like "jac bele" → "Jace Beleren"
* **Random Card Discovery** – Get random MTG cards with the `!random` command
* **Rich Embeds** – Card images, rarity colors, mana costs, and detailed info
* **Respectful API Use** – Built-in Scryfall rate limiting (10 requests/sec)
* **Development Tools** – Auto-restart, build scripts, quality checks with Mage
* **Easy Configuration** – Environment variables, `.env` support, and validation
* **Simple Deployment** – Single-binary builds with structured logging

---

## Quick Start

```bash
git clone https://github.com/dunamismax/MTG-Card-Bot.git
cd MTG-Card-Bot
go mod tidy
go install github.com/magefile/mage@latest
cp .env.example .env  # Add your Discord bot token
mage setup
mage dev
```

**Requirements:** Go 1.24+, Discord Bot Token

---

## Mage Commands

```bash
mage setup         # Install dev tools
mage dev           # Run bot with auto-restart
mage build         # Build binary
mage fmt / lint    # Format & lint checks
mage vulncheck     # Security check
```

---

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/discord-bots/mtg-card-bot-gopher.png" alt="mtg-card-bot-gopher" width="300" />
</p>

## Bot Commands

```bash
# Card lookup with fuzzy matching
!lightning bolt        # Finds "Lightning Bolt"
!the-one-ring         # Finds "The One Ring"
!jac bele             # Finds "Jace Beleren" (fuzzy search)

# Random card discovery
!random               # Get a random Magic: The Gathering card

# Bot information and statistics
!help                 # Show available commands
!stats                # Display bot performance metrics

# Examples of fuzzy matching
!counterspell         # Exact match
!counter              # Finds "Counterspell"
!force will           # Finds "Force of Will"
!ancestral            # Finds "Ancestral Recall"
```

## Bot in Action

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/discord-bots/mtg-card-bot-help.png" alt="Help Command Screenshot" width="500" />
  <br>
  <em>Help command showing all available features</em>
</p>

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/discord-bots/mtg-card-bot-lotus.png" alt="Fuzzy Search Example" width="500" />
  <br>
  <em>Fuzzy search in action - "!black lo" finds "Black Lotus"</em>
</p>

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/discord-bots/mtg-card-bot-stats.png" alt="Stats Command Screenshot" width="500" />
  <br>
  <em>Performance statistics and monitoring</em>
</p>

---

## Development

The bot uses a clean architecture with shared packages:

* `bots/mtg-card-bot/` - Main bot application
* `pkg/discord/` - Discord client and helpers
* `pkg/scryfall/` - Scryfall API integration

Start development with `mage dev` for auto-restart functionality.

---

<p align="center">
  <img src="https://github.com/dunamismax/images/blob/main/golang/go-logo.png" alt="MTG Card Bot Logo" width="300" />
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
  <strong>MTG Card Discord Bot</strong><br>
  <sub>DiscordGo • Scryfall API • Mage • slog • Config • Rate Limiting • Rich Embeds</sub>
</p>

---
