# MTG Card Bot

Discord bot for lightning-fast Magic: The Gathering lookups with live prices, legality, rulings, and polished embeds powered by the Scryfall API.

## Quick Start

### Requirements

- [uv](https://docs.astral.sh/uv/) package manager
- Python 3.13 (installed and pinned through `uv python`)
- Discord bot token with message content intent enabled

### Installation

```bash
uv python install 3.13
uv python pin 3.13

git clone https://github.com/dunamismax/mtg-card-bot.git
cd mtg-card-bot

cp .env.example .env  # add your MTG_DISCORD_TOKEN
uv sync

uv run python manage_bot.py start   # start with live logs
```

## Configuration

Environment variables can be set in `.env`:

| Variable | Description | Default |
| --- | --- | --- |
| `MTG_DISCORD_TOKEN` | Discord bot token (required) | — |
| `MTG_COMMAND_PREFIX` | Command prefix | `!` |
| `MTG_LOG_LEVEL` | `debug`, `info`, `warning`, `error` | `info` |
| `MTG_JSON_LOGGING` | Structured JSON logs | `false` |
| `MTG_COMMAND_COOLDOWN` | Seconds between commands per user | `2.0` |

## Using the Bot

### Core Commands

- ``!lightning bolt`` or `[[Lightning Bolt]]` – fuzzy card lookup with pricing
- ``!rules counterspell`` – official Gatherer/Scryfall rulings
- ``!random`` – random card, accepts filters (`!random rarity:mythic e:mh3`)
- ``!help`` – in-Discord guide with examples

### Filters, Sorting, and Multi-Card

- Any Scryfall filter works: `e:who`, `type:legendary`, `is:showcase`, `mana>={5}`
- Rank results with `order:` / `dir:`: `order:edhrec`, `order:usd dir:desc`
- Semicolons request multiple cards in one message: ``!bolt; counterspell; doom blade``
- Filtered lookups without an order pick varied prints automatically

### Aliases

`!r`, `!rand`, `!h`, and `!?` mirror the long-form commands.

## Features

- Smart fuzzy search with typo tolerance and bracket syntax
- Live pricing (USD/EUR/foil/tix) and legality summaries
- Random card discovery with Scryfall-compliant rate limiting (10 req/s)
- Multi-card grid display with image attachments
- Oracle tag and advanced filter support for cube/EDH searching
- Structured logging, graceful shutdown, and duplicate suppression

## Development

```bash
uv run ruff format .
uv run ruff check .
uv run mypy mtg_card_bot/
uv run python manage_bot.py logs
```

## Deployment Notes

- **Systemd:** run `manage_bot.py start` from a service pointing at your project directory.
- **Docker:** base on `python:3.13-slim`, install `uv`, copy the project, `uv sync --frozen`, then run `uv run python manage_bot.py start`.

## License

Apache 2.0. See [LICENSE](LICENSE).
