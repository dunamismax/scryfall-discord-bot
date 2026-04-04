# MTG Card Bot

Discord bot for fast Magic: The Gathering card lookups with live pricing, legality, rulings, and embed-first responses powered by the Scryfall API.

## What It Does

- Fuzzy card lookup with bracket syntax like `[[Lightning Bolt]]`
- Live card pricing, legality summaries, and rulings
- Random card discovery with full Scryfall filter support
- Multi-card lookups in a single message
- Structured logging, duplicate suppression, and graceful shutdown

## Stack

| Component | Choice |
| --- | --- |
| Language | Python 3.13 |
| Package manager | `uv` |
| Discord client | `discord.py` |
| HTTP client | `httpx` |
| Packaging | `pyproject.toml` + Hatchling |
| Lint / format | Ruff |
| Type checking | Pyright |
| Tests | pytest |

## Quick Start

### Requirements

- [uv](https://docs.astral.sh/uv/) installed locally
- Python 3.13 available through `uv python`
- A Discord bot token with message content intent enabled

### Install

```bash
git clone https://github.com/dunamismax/mtg-card-bot.git
cd mtg-card-bot

uv python install 3.13
uv sync

cp .env.example .env
# edit .env and set MTG_DISCORD_TOKEN
```

### Run

```bash
# direct package entrypoint
uv run mtg-card-bot

# or use the local manager helper
uv run python manage_bot.py start
```

## Configuration

Set environment variables in `.env` or in the process environment:

| Variable | Description | Default |
| --- | --- | --- |
| `MTG_DISCORD_TOKEN` | Discord bot token | required |
| `MTG_COMMAND_PREFIX` | Command prefix | `!` |
| `MTG_LOG_LEVEL` | `debug`, `info`, `warning`, `error` | `info` |
| `MTG_JSON_LOGGING` | Emit JSON logs | `false` |
| `MTG_COMMAND_COOLDOWN` | Per-user cooldown in seconds | `2.0` |

## Usage

### Core Commands

- `!lightning bolt` or `[[Lightning Bolt]]` for fuzzy lookup
- `!rules counterspell` for rulings
- `!random` for a random card
- `!random rarity:mythic e:mh3` for filtered random discovery
- `!help` for the in-Discord command guide

### Query Features

- Any Scryfall search filter works, including `e:who`, `type:legendary`, and `mana>={5}`
- Use `order:` and `dir:` for ranked results like `order:usd dir:desc`
- Separate multiple lookups with semicolons: `!bolt; counterspell; doom blade`
- Aliases: `!r`, `!rand`, `!h`, and `!?`

## Development

```bash
uv sync
uv run pre-commit install
uv run ruff check .
uv run ruff format --check .
uv run pyright
uv run pytest
```

The same four quality gates run in CI. The test suite uses mocks and fake Discord/Scryfall interactions so local verification stays fast and deterministic.

## Project Layout

```text
src/mtg_card_bot/
  __main__.py   - package entrypoint
  bot.py        - Discord client and command handling
  config.py     - environment loading and config validation
  errors.py     - bot-specific error types
  logging.py    - structured logging wrapper
  scryfall.py   - async Scryfall client and card models
tests/
  test_*.py     - focused bot, config, error, and API client tests
manage_bot.py   - local process manager for start/stop/status/logs
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow and repo conventions.

## Deployment Notes

- `systemd`: run from the repo root after `uv sync`, either via `uv run mtg-card-bot` or `uv run python manage_bot.py start`
- Container builds: install `uv`, copy the repo, run `uv sync --frozen`, then start the package entrypoint

## License

GPL-3.0. See [LICENSE](LICENSE).
