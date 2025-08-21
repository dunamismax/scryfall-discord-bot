"""Main MTG Card Bot Discord bot implementation."""

import asyncio
import io
import re
import time
from urllib.parse import urlparse

import discord
import httpx

from . import config, errors, logging
from .scryfall import Card, ScryfallClient


class MultiResolvedCard:
    """Container for a resolved card query in multi-card lookups."""

    def __init__(
        self,
        query: str,
        card: Card | None = None,
        used_fallback: bool = False,
        error: Exception | None = None,
    ) -> None:
        self.query = query
        self.card = card
        self.used_fallback = used_fallback
        self.error = error


class MTGCardBot(discord.Client):
    """Discord bot for Magic: The Gathering card lookups."""

    def __init__(self, cfg: config.MTGConfig) -> None:
        """Initialize the MTG Card Bot."""
        intents = discord.Intents.default()
        intents.message_content = True
        super().__init__(intents=intents)
        self.config = cfg

        self.logger = logging.with_component("mtg_card_bot")
        self.scryfall_client = ScryfallClient()
        self.http_client = httpx.AsyncClient(timeout=20.0)

        # Enhanced duplicate suppression structures
        # Track recent (author, normalized_content) to timestamp
        self._recent_commands: dict[tuple[int, str], float] = {}
        # Track processed Discord message IDs
        self._processed_message_ids: set[int] = set()
        # Background cleanup task
        self._cleanup_task: asyncio.Task | None = None

        # Performance improvements
        self._user_rate_limits: dict[int, float] = {}  # Track per-user rate limits

    async def start(self) -> None:
        # Prefer static token from config; support multiple field names
        token = getattr(self.config, "token", None) or getattr(
            self.config, "discord_token", ""
        )
        await super().start(token)

    async def setup_hook(self) -> None:
        """Called when the bot is starting up."""
        # Start background cleanup task
        self._cleanup_task = asyncio.create_task(
            self._cleanup_duplicates_periodically()
        )
        self.logger.info("MTG Card bot setup completed")

    async def on_ready(self) -> None:
        """Called when the bot is ready."""
        self.logger.info("Bot is ready", username=str(self.user))

    async def on_message(self, message: discord.Message) -> None:
        """Handle incoming messages."""
        # Ignore messages from bots
        if message.author.bot:
            return

        # If we've already processed this message (duplicate delivery), skip
        if message.id in self._processed_message_ids:
            return

        # Check for bracket syntax [[card name]] or prefix command
        bracket_match = self._extract_bracket_content(message.content)
        if bracket_match:
            content = bracket_match
        elif message.content.startswith(self.config.command_prefix):
            # Remove prefix
            content = message.content[len(self.config.command_prefix) :]
        else:
            return

        # Check per-user rate limiting
        user_id = message.author.id
        now = time.time()
        last_command = self._user_rate_limits.get(user_id, 0)
        if now - last_command < 3.0:  # 3 seconds between commands
            self.logger.debug(
                "Rate limited user",
                user_id=str(user_id),
                username=message.author.name,
                time_since_last=now - last_command,
            )
            return
        self._user_rate_limits[user_id] = now

        # Enhanced duplicate suppression with longer window and better logging
        normalized = " ".join(content.lower().split())
        key = (message.author.id, normalized)
        last = self._recent_commands.get(key)

        # Suppress duplicates within 2.5 seconds
        if last is not None and (now - last) < 2.5:
            self.logger.debug(
                "Suppressed duplicate command",
                user_id=str(message.author.id),
                username=message.author.name,
                content=normalized[:50],
                time_since_last=now - last,
            )
            return

        self._recent_commands[key] = now
        self._processed_message_ids.add(message.id)

        # If the content contains semicolons, treat as multi-card lookup
        if ";" in content:
            await self._handle_multi_card_lookup(message, content)
            return

        parts = content.split()
        if not parts:
            return

        command = parts[0].lower()
        args = parts[1:]

        # Handle specific commands with aliases
        if command in ["random", "rand", "r"]:
            # Support filtered random: "random e:who rarity:mythic"
            if args:
                filter_query = " ".join(args)
                await self._handle_random_card(message, filter_query)
            else:
                await self._handle_random_card(message)
        elif command in ["help", "h", "?"]:
            await self._handle_help(message)
        elif command == "rules":
            if args:
                card_query = " ".join(args)
                await self._handle_rules_lookup(message, card_query)
            else:
                await self._send_error_message(
                    message.channel, "Please provide a card name for rules lookup."
                )
        else:
            # Treat as card lookup
            card_query = " ".join(parts)
            await self._handle_card_lookup(message, card_query)

    async def _handle_random_card(
        self, message: discord.Message, filter_query: str = ""
    ) -> None:
        """Handle the random card command with optional filters."""
        if filter_query:
            self.logger.info(
                "Fetching filtered random card",
                user_id=str(message.author.id),
                username=message.author.name,
                filter_query=filter_query,
            )
        else:
            self.logger.info(
                "Fetching random card",
                user_id=str(message.author.id),
                username=message.author.name,
            )

        try:
            card = await self.scryfall_client.get_random_card(filter_query)
            await self._send_card_message(
                message.channel, card, False, filter_query or "random"
            )
        except Exception as e:
            self.logger.error(
                "Random card command failed",
                user_id=str(message.author.id),
                filter_query=filter_query,
                error=str(e),
            )

            # Provide helpful error messages
            if isinstance(e, errors.MTGError):
                if e.error_type == errors.ErrorType.NOT_FOUND and filter_query:
                    error_msg = f"No cards found matching filters: '{filter_query}'. Try broader criteria."
                elif e.error_type == errors.ErrorType.RATE_LIMIT:
                    error_msg = "API rate limit exceeded. Please try again in a moment."
                else:
                    error_msg = (
                        "Sorry, something went wrong while fetching a random card."
                    )
            else:
                error_msg = "Sorry, something went wrong while fetching a random card."

            await self._send_error_message(message.channel, error_msg)

    async def _handle_card_lookup(
        self, message: discord.Message, card_query: str
    ) -> None:
        """Handle card lookup with support for filtering parameters."""
        if not card_query:
            await self._send_error_message(
                message.channel, "Card query cannot be empty."
            )
            return

        self.logger.info(
            "Looking up card",
            user_id=str(message.author.id),
            username=message.author.name,
            card_query=card_query,
        )

        try:
            card, used_fallback = await self._resolve_card_query(card_query)
            await self._send_card_message(
                message.channel, card, used_fallback, card_query
            )
        except Exception as e:
            self.logger.error(
                "Card lookup failed",
                user_id=str(message.author.id),
                card_query=card_query,
                error=str(e),
            )

            # Provide helpful error messages based on error type
            if isinstance(e, errors.MTGError):
                if e.error_type == errors.ErrorType.NOT_FOUND:
                    if self._has_filter_parameters(card_query):
                        error_msg = f"No cards found for '{card_query}'. Try simpler filters like `e:set` or `is:foil`, or check the spelling."
                    else:
                        error_msg = f"Card '{card_query}' not found. Try partial names like 'bolt' for 'Lightning Bolt'."
                elif e.error_type == errors.ErrorType.RATE_LIMIT:
                    error_msg = "API rate limit exceeded. Please try again in a moment."
                else:
                    error_msg = (
                        "Sorry, something went wrong while searching for that card."
                    )
            else:
                error_msg = "Sorry, something went wrong while searching for that card."

            await self._send_error_message(message.channel, error_msg)

    async def _handle_rules_lookup(
        self, message: discord.Message, card_query: str
    ) -> None:
        """Handle rules lookup for a card."""
        if not card_query:
            await self._send_error_message(
                message.channel, "Card query cannot be empty."
            )
            return

        self.logger.info(
            "Looking up rules for card",
            user_id=str(message.author.id),
            username=message.author.name,
            card_query=card_query,
        )

        try:
            # First get the card
            card, used_fallback = await self._resolve_card_query(card_query)

            # Then get its rulings
            rulings = await self.scryfall_client.get_card_rulings(card.id)

            if not rulings:
                embed = discord.Embed(
                    title="No Rulings Found",
                    description=f"No official rulings found for **{card.get_display_name()}**.",
                    color=0x9B59B6,
                    url=card.scryfall_uri,
                )
                await message.channel.send(embed=embed)
                return

            # Create rulings embed
            embed = discord.Embed(
                title=f"Rulings for {card.get_display_name()}",
                url=card.scryfall_uri,
                color=self._get_rarity_color(card.rarity),
            )

            if used_fallback:
                embed.description = f"*Showing closest match for '{card_query}'*"

            # Add rulings as fields (Discord has a limit of 25 fields)
            ruling_count = min(len(rulings), 10)  # Limit to 10 rulings for readability

            for i, ruling in enumerate(rulings[:ruling_count]):
                source = "Wizards" if ruling.get("source") == "wotc" else "Scryfall"
                date = ruling.get("published_at", "Unknown date")
                comment = ruling.get("comment", "No ruling text")

                # Truncate long rulings
                if len(comment) > 1024:
                    comment = comment[:1021] + "..."

                embed.add_field(name=f"{source} ({date})", value=comment, inline=False)

            if len(rulings) > ruling_count:
                embed.set_footer(
                    text=f"Showing {ruling_count} of {len(rulings)} rulings. Visit Scryfall for complete rulings."
                )
            else:
                embed.set_footer(text=f"{len(rulings)} ruling(s) found.")

            await message.channel.send(embed=embed)

        except Exception as e:
            self.logger.error(
                "Rules lookup failed",
                user_id=str(message.author.id),
                card_query=card_query,
                error=str(e),
            )

            if isinstance(e, errors.MTGError):
                if e.error_type == errors.ErrorType.NOT_FOUND:
                    error_msg = f"Card '{card_query}' not found for rules lookup."
                else:
                    error_msg = "Sorry, something went wrong while looking up rules for that card."
            else:
                error_msg = (
                    "Sorry, something went wrong while looking up rules for that card."
                )

            await self._send_error_message(message.channel, error_msg)

    async def _resolve_card_query(self, card_query: str) -> tuple[Card, bool]:
        """Resolve a single card query into a card with caching and fallbacks."""
        card_query = card_query.strip()
        has_filters = self._has_filter_parameters(card_query)
        used_fallback = False

        if has_filters:
            # Use search API for filtered queries
            try:
                card = await self.scryfall_client.search_card_first(card_query)
            except Exception:
                # If filtered search fails, extract card name and try fallback
                card_name = self._extract_card_name(card_query)
                if card_name and len(card_name) >= 2:
                    card = await self.scryfall_client.get_card_by_name(card_name)
                    used_fallback = True
                else:
                    raise
        else:
            # Direct API call for simple name lookups
            card = await self.scryfall_client.get_card_by_name(card_query)

        if not card or not card.is_valid_card():
            raise errors.create_error(
                errors.ErrorType.NOT_FOUND, "No card found for query"
            )

        return card, used_fallback

    async def _handle_multi_card_lookup(
        self, message: discord.Message, raw_content: str
    ) -> None:
        """Handle a semicolon-separated list of card queries."""
        # Split on semicolons and trim spaces
        raw_parts = raw_content.split(";")
        queries = [q.strip() for q in raw_parts if q.strip()]

        if not queries:
            await self._send_error_message(
                message.channel, "No valid card queries provided."
            )
            return

        # If only one query, fallback to normal flow
        if len(queries) == 1:
            await self._handle_card_lookup(message, queries[0])
            return

        self.logger.info(
            "Multi-card lookup",
            user_id=str(message.author.id),
            username=message.author.name,
            query_count=len(queries),
        )

        # Resolve cards sequentially
        resolved_cards: list[MultiResolvedCard] = []
        for query in queries:
            try:
                card, used_fallback = await self._resolve_card_query(query)
                resolved_cards.append(MultiResolvedCard(query, card, used_fallback))
            except Exception as e:
                resolved_cards.append(MultiResolvedCard(query, error=e))

        # Check if any cards were successfully resolved
        success_count = sum(
            1
            for r in resolved_cards
            if r.error is None and r.card and r.card.is_valid_card()
        )

        if success_count == 0:
            await self._send_error_message(
                message.channel, "Failed to resolve any requested cards."
            )
            return

        # Send cards in chunks of 4 for nice layout
        max_per_message = 4
        for i in range(0, len(resolved_cards), max_per_message):
            chunk = resolved_cards[i : i + max_per_message]
            await self._send_card_grid_message(message.channel, chunk)

    async def _send_card_grid_message(
        self, channel: discord.abc.Messageable, items: list[MultiResolvedCard]
    ) -> None:
        """Send a grid of card images and information."""
        files: list[discord.File] = []
        md_lines: list[str] = []

        for item in items:
            if item.error or not item.card or not item.card.is_valid_card():
                md_lines.append(f"- {item.query}: not found")
                continue

            name = item.card.get_display_name()
            label = name
            if item.used_fallback:
                label += " (closest match)"

            # Add masked link for clean display
            if item.card.scryfall_uri:
                md_lines.append(f"- [{label}]({item.card.scryfall_uri})")
            else:
                md_lines.append(f"- {label}")

            # Fetch image if available
            if item.card.has_image():
                image_url = item.card.get_best_image_url()
                try:
                    image_data, filename = await self._fetch_image(image_url, name)
                    files.append(
                        discord.File(io.BytesIO(image_data), filename=filename)
                    )
                except Exception as e:
                    self.logger.warning(
                        "Failed to fetch image", image_url=image_url, error=str(e)
                    )

        # Send list embed first
        embed = discord.Embed(
            title="Requested Cards", description="\n".join(md_lines), color=0x5865F2
        )
        await channel.send(embed=embed)

        # Then send images if any were fetched
        if files:
            await channel.send(files=files)

    async def _fetch_image(self, url: str, card_name: str) -> tuple[bytes, str]:
        """Fetch image data and return bytes with filename."""
        response = await self.http_client.get(url)
        response.raise_for_status()

        # Determine file extension
        content_type = response.headers.get("content-type", "")
        if "png" in content_type:
            ext = ".png"
        elif "jpeg" in content_type or "jpg" in content_type:
            ext = ".jpg"
        else:
            # Try to guess from URL
            parsed_url = urlparse(url)
            path = parsed_url.path.lower()
            if path.endswith(".png"):
                ext = ".png"
            elif path.endswith((".jpg", ".jpeg")):
                ext = ".jpg"
            else:
                ext = ".jpg"  # Default

        # Create safe filename
        safe_name = self._safe_filename(card_name)
        filename = f"{safe_name}{ext}"

        return response.content, filename

    def _safe_filename(self, name: str) -> str:
        """Create a safe filename from a card name."""
        # Replace unsafe characters with hyphens
        safe = re.sub(r"[^a-zA-Z0-9._-]+", "-", name.lower())
        safe = safe.strip("-._")
        if not safe:
            return "card"
        return safe[:64]  # Limit length

    def _has_filter_parameters(self, query: str) -> bool:
        """Check if the query contains Scryfall filter syntax."""
        essential_filters = [
            "e:",
            "set:",
            "frame:",
            "border:",
            "is:foil",
            "is:nonfoil",
            "is:fullart",
            "is:textless",
            "is:borderless",
            "rarity:",
            "cn:",
            "number:",
        ]

        lower_query = query.lower()
        return any(filter_param in lower_query for filter_param in essential_filters)

    def _extract_card_name(self, query: str) -> str:
        """Extract the card name from a filtered query for fallback purposes."""
        words = query.split()
        card_name_parts = []

        for word in words:
            lower_word = word.lower()

            # Skip known filter patterns with colons
            if ":" in lower_word:
                continue

            # Skip standalone filter keywords
            essential_keywords = [
                "foil",
                "nonfoil",
                "fullart",
                "textless",
                "borderless",
            ]
            if lower_word not in essential_keywords:
                card_name_parts.append(word)

        return " ".join(card_name_parts).strip()

    def _extract_bracket_content(self, message_content: str) -> str | None:
        """Extract card name from bracket syntax [[card name]]."""
        import re

        # Look for [[content]] pattern
        bracket_pattern = r"\[\[([^\]]+)\]\]"
        match = re.search(bracket_pattern, message_content)

        if match:
            return match.group(1).strip()

        return None

    async def _send_card_message(
        self,
        channel: discord.abc.Messageable,
        card: Card,
        used_fallback: bool,
        original_query: str,
    ) -> None:
        """Send a card image and details to a Discord channel."""
        if not card.is_valid_card():
            await self._send_error_message(
                channel, "Received invalid card data from API."
            )
            return

        if not card.has_image():
            # Send text-only message if no image is available
            embed = discord.Embed(
                title=card.get_display_name(),
                description=f"**{card.type_line}**\n{card.oracle_text}",
                color=0x9B59B6,
                url=card.scryfall_uri,
            )

            embed.add_field(
                name="Set",
                value=f"{card.set_name} ({card.set_code.upper()})",
                inline=True,
            )

            embed.add_field(name="Rarity", value=card.rarity.title(), inline=True)

            # Add mana cost and pricing if available
            if card.mana_cost:
                mana_cost_text = card.mana_cost
                price_display = card.get_price_display()
                if price_display:
                    mana_cost_text += f" - {price_display}"
                embed.add_field(name="Mana Cost", value=mana_cost_text, inline=True)

            # Add format legality
            legality_text = card.get_format_legalities()
            if legality_text:
                embed.add_field(name="Legal in", value=legality_text, inline=False)

            if card.artist:
                embed.add_field(name="Artist", value=card.artist, inline=True)

            await channel.send(embed=embed)
            return

        # Create rich embed with card image
        embed = discord.Embed(
            title=card.get_display_name(),
            url=card.scryfall_uri,
            color=self._get_rarity_color(card.rarity),
        )

        embed.set_image(url=card.get_best_image_url())

        # Add mana cost and fallback notification
        descriptions = []

        if used_fallback:
            descriptions.append(
                f"*No exact match found for filters in `{original_query}`, showing closest match*"
            )
        elif (
            original_query
            and original_query != "random"
            and self._has_filter_parameters(original_query)
        ):
            descriptions.append(f"*Filtered result for: `{original_query}`*")

        if card.mana_cost:
            mana_cost_text = f"**Mana Cost:** {card.mana_cost}"
            price_display = card.get_price_display()
            if price_display:
                mana_cost_text += f" - **Cost:** {price_display}"
            descriptions.append(mana_cost_text)

        if descriptions:
            embed.description = "\n".join(descriptions)

        # Add format legality field
        legality_text = card.get_format_legalities()
        if legality_text:
            embed.add_field(name="Legal in", value=legality_text, inline=False)

        # Footer with set, rarity, and artist
        footer_parts = [card.set_name, card.rarity.title()]
        if card.artist:
            footer_parts.append(f"Art by {card.artist}")

        embed.set_footer(text=" • ".join(footer_parts))

        await channel.send(embed=embed)

    def _get_rarity_color(self, rarity: str) -> int:
        """Return a color based on card rarity."""
        rarity_colors = {
            "mythic": 0xFF8C00,  # Dark orange
            "rare": 0xFFD700,  # Gold
            "uncommon": 0xC0C0C0,  # Silver
            "common": 0x000000,  # Black
            "special": 0xFF1493,  # Deep pink
            "bonus": 0x9370DB,  # Medium purple
        }
        return rarity_colors.get(rarity.lower(), 0x9B59B6)  # Default purple

    async def _handle_help(self, message: discord.Message) -> None:
        """Handle the help command."""
        self.logger.info(
            "Showing help information",
            user_id=str(message.author.id),
            username=message.author.name,
        )

        prefix = self.config.command_prefix
        embed = discord.Embed(
            title="MTG Card Bot",
            description="**Fast Magic card lookup with live pricing & format legality**",
            color=0x5865F2,
        )

        embed.add_field(
            name="Basic Commands",
            value=(
                f"`{prefix}lightning bolt` or `[[Lightning Bolt]]` — Card lookup\n"
                f"`{prefix}rules counterspell` — Official rulings\n"
                f"`{prefix}random` — Random card\n"
                f"`{prefix}random rarity:mythic` — Filtered random\n"
                f"`{prefix}help` — This guide"
            ),
            inline=False,
        )

        embed.add_field(
            name="Advanced Filters",
            value=(
                "**Sets:** `e:mh3`, `e:ltr`, `e:who`\n"
                "**Rarity:** `rarity:mythic`, `rarity:rare`\n"
                "**Special:** `is:foil`, `is:showcase`, `frame:borderless`\n"
                "**Example:** `{prefix}random e:who rarity:mythic`"
            ),
            inline=True,
        )

        embed.add_field(
            name="Multi-Card Lookup",
            value=(
                "Use semicolons to get multiple cards:\n"
                f"`{prefix}bolt; counterspell; doom blade`\n"
                f"`{prefix}sol ring e:lea; mox ruby e:lea`"
            ),
            inline=True,
        )

        embed.add_field(
            name="Quick Tips",
            value=(
                "• **Aliases:** `!r`, `!rand`, `!h`, `!?`\n"
                "• **Fuzzy search:** `ragav` → `Ragavan, Nimble Pilferer`\n"
                "• **Live pricing** from TCGPlayer/Scryfall\n"
                "• **No caching** — always fresh data!"
            ),
            inline=False,
        )

        embed.set_footer(
            text="Powered by Scryfall API • github.com/dunamismax/mtg-card-bot"
        )

        await message.channel.send(embed=embed)

    async def _send_error_message(
        self, channel: discord.abc.Messageable, message: str
    ) -> None:
        """Send an error message to a Discord channel."""
        embed = discord.Embed(title="Error", description=message, color=0xE74C3C)

        try:
            await channel.send(embed=embed)
        except Exception as e:
            self.logger.error("Failed to send error message", error=str(e))

    def _format_duration(self, seconds: float) -> str:
        """Format a duration in seconds into a human-readable string."""
        seconds = int(seconds)
        days = seconds // 86400
        hours = (seconds % 86400) // 3600
        minutes = (seconds % 3600) // 60
        secs = seconds % 60

        if days > 0:
            return f"{days}d {hours}h {minutes}m {secs}s"
        if hours > 0:
            return f"{hours}h {minutes}m {secs}s"
        if minutes > 0:
            return f"{minutes}m {secs}s"
        return f"{secs}s"

    async def _cleanup_duplicates_periodically(self) -> None:
        """Background task to clean up old duplicate suppression data."""
        while True:
            try:
                await asyncio.sleep(60)  # Clean up every minute
                now = time.time()
                cutoff = now - 300  # Keep data for 5 minutes

                # Clean up old command timestamps
                old_keys = [
                    key
                    for key, timestamp in self._recent_commands.items()
                    if timestamp < cutoff
                ]
                for key in old_keys:
                    del self._recent_commands[key]

                # Clean up old rate limit timestamps (keep for 5 minutes)
                old_rate_limit_users = [
                    user_id
                    for user_id, timestamp in self._user_rate_limits.items()
                    if timestamp < cutoff
                ]
                for user_id in old_rate_limit_users:
                    del self._user_rate_limits[user_id]

                # Clean up old message IDs (keep last 1000)
                if len(self._processed_message_ids) > 1000:
                    # Convert to list, sort, and keep newest 500
                    sorted_ids = sorted(self._processed_message_ids)
                    self._processed_message_ids = set(sorted_ids[-500:])

                if old_keys or len(self._processed_message_ids) > 1000:
                    self.logger.debug(
                        "Cleaned up duplicate suppression data",
                        commands_removed=len(old_keys),
                        message_ids_kept=len(self._processed_message_ids),
                    )

            except asyncio.CancelledError:
                break
            except Exception as e:
                self.logger.error("Error in duplicate cleanup task", error=str(e))

    async def close(self) -> None:
        """Clean shutdown of the bot."""
        self.logger.info("Shutting down MTG Card bot")

        # Cancel cleanup task
        if self._cleanup_task and not self._cleanup_task.done():
            self._cleanup_task.cancel()
            try:
                await self._cleanup_task
            except asyncio.CancelledError:
                pass

        # Close HTTP clients
        try:
            await self.scryfall_client.close()
        except Exception as e:
            self.logger.warning("Error closing scryfall client", error=str(e))

        try:
            await self.http_client.aclose()
        except Exception as e:
            self.logger.warning("Error closing http client", error=str(e))

        # Clear duplicate suppression data
        self._recent_commands.clear()
        self._processed_message_ids.clear()
        self._user_rate_limits.clear()

        await super().close()
