"""Main MTG Card Bot Discord bot implementation."""

import asyncio
import re
import time
from contextlib import suppress

import discord

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

        # Enhanced duplicate suppression structures
        # Track recent (author, normalized_content) to timestamp
        self._recent_commands: dict[tuple[int, str], float] = {}
        # Track processed Discord message IDs
        self._processed_message_ids: set[int] = set()
        # Background cleanup task
        self._cleanup_task: asyncio.Task[None] | None = None

        # Performance improvements
        self._user_rate_limits: dict[int, float] = {}  # Track per-user rate limits

    async def start(self, token: str | None = None, *, reconnect: bool = True) -> None:
        """Start the Discord client with the configured token by default."""
        resolved_token = token or self.config.discord_token
        await super().start(resolved_token, reconnect=reconnect)

    async def setup_hook(self) -> None:
        """Called when the bot is starting up."""
        # Start background cleanup task
        self._cleanup_task = asyncio.create_task(
            self._cleanup_duplicates_periodically()
        )
        self.logger.info("MTG Card bot setup completed")

    async def on_ready(self) -> None:
        """Called when the bot is ready."""
        guild_info = [
            f"{g.name} (id={g.id}, members={g.member_count})" for g in self.guilds
        ]
        self.logger.info(
            "Bot is ready",
            username=str(self.user),
            guild_count=len(self.guilds),
            guilds=", ".join(guild_info) if guild_info else "NO GUILDS",
        )
        if not self.guilds:
            self.logger.warning(
                "Bot is not in any guilds! "
                "Invite it using the OAuth2 URL from the Discord Developer Portal."
            )

    async def on_disconnect(self) -> None:
        """Called when the bot disconnects from Discord."""
        self.logger.warning("Bot disconnected from Discord gateway")

    async def on_resumed(self) -> None:
        """Called when the bot resumes a session after disconnect."""
        self.logger.info("Bot resumed Discord gateway session")

    async def on_error(
        self, event_method: str, *args: object, **kwargs: object
    ) -> None:
        """Called when an event handler raises an exception."""
        import traceback

        self.logger.error(
            "Unhandled exception in event handler",
            event=event_method,
            error=traceback.format_exc(),
        )

    async def on_message(self, message: discord.Message) -> None:
        """Handle incoming messages."""
        # Ignore messages from bots
        if message.author.bot:
            return

        # Diagnostic: log every non-bot message received (debug level)
        guild = getattr(message, "guild", None)
        guild_name = getattr(guild, "name", "DM") if guild else "DM"
        channel = getattr(message, "channel", None)
        channel_name = getattr(channel, "name", "unknown") if channel else "unknown"
        self.logger.debug(
            "Message received",
            author=message.author.name,
            guild=str(guild_name),
            channel=str(channel_name),
            content_length=len(message.content),
            content_preview=message.content[:80] if message.content else "<EMPTY>",
        )

        # Warn if message content is empty — likely means MESSAGE_CONTENT
        # privileged intent is not enabled in the Discord Developer Portal
        if not message.content:
            self.logger.warning(
                "Received message with empty content — the MESSAGE_CONTENT "
                "privileged intent may not be enabled in the Discord Developer "
                "Portal (https://discord.com/developers/applications)",
                author=message.author.name,
                guild=str(guild_name),
            )
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
        cooldown = self.config.command_cooldown
        if cooldown > 0 and now - last_command < cooldown:
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
            await self._send_error_message(
                message.channel, self._user_error_message(e, filter_query, "random")
            )

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
            await self._send_error_message(
                message.channel, self._user_error_message(e, card_query)
            )

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
                    description=(
                        f"No official rulings found for **{card.get_display_name()}**."
                    ),
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

            for ruling in rulings[:ruling_count]:
                source = "Wizards" if ruling.get("source") == "wotc" else "Scryfall"
                date = ruling.get("published_at", "Unknown date")
                comment = ruling.get("comment", "No ruling text")

                # Truncate long rulings
                if len(comment) > 1024:
                    comment = comment[:1021] + "..."

                embed.add_field(name=f"{source} ({date})", value=comment, inline=False)

            if len(rulings) > ruling_count:
                embed.set_footer(
                    text=(
                        f"Showing {ruling_count} of {len(rulings)} rulings. "
                        "Visit Scryfall for complete rulings."
                    )
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
            await self._send_error_message(
                message.channel, self._user_error_message(e, card_query)
            )

    async def _resolve_card_query(self, card_query: str) -> tuple[Card, bool]:
        """Resolve a single card query into a card with caching and fallbacks."""
        card_query = card_query.strip()
        clean_query, order_hint, direction_hint = self._extract_sort_parameters(
            card_query
        )
        search_query = clean_query or card_query
        has_filters = bool(order_hint) or self._has_filter_parameters(search_query)
        used_fallback = False

        if has_filters:
            # Use search API for filtered queries
            try:
                card = await self.scryfall_client.search_card_first(
                    search_query, order_hint, direction_hint
                )
            except Exception:
                # If filtered search fails, extract card name and try fallback
                card_name = self._extract_card_name(search_query)
                if card_name and len(card_name) >= 2:
                    card = await self.scryfall_client.get_card_by_name(card_name)
                    used_fallback = True
                else:
                    raise
        else:
            # Direct API call for simple name lookups
            card = await self.scryfall_client.get_card_by_name(search_query)

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

        # Resolve cards concurrently
        async def _resolve_one(query: str) -> MultiResolvedCard:
            try:
                parts = query.split()
                command = parts[0].lower() if parts else ""
                # Handle random commands within multi-card lookups
                if command in ("random", "rand", "r"):
                    filter_query = " ".join(parts[1:])
                    card = await self.scryfall_client.get_random_card(filter_query)
                    return MultiResolvedCard(query, card)
                card, used_fallback = await self._resolve_card_query(query)
                return MultiResolvedCard(query, card, used_fallback)
            except Exception as e:
                return MultiResolvedCard(query, error=e)

        resolved_cards = list(await asyncio.gather(*[_resolve_one(q) for q in queries]))

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

        # Build one embed per card (Discord allows up to 10 embeds per message)
        embeds: list[discord.Embed] = []
        errors: list[str] = []

        for item in resolved_cards:
            if item.error or not item.card or not item.card.is_valid_card():
                errors.append(item.query)
                continue

            card = item.card
            embed = discord.Embed(
                title=card.get_display_name(),
                url=card.scryfall_uri,
                color=self._get_rarity_color(card.rarity),
            )

            if item.used_fallback:
                embed.description = f"*Closest match for '{item.query}'*"

            image_url = card.get_best_image_url(("large", "normal", "small"))
            if image_url:
                embed.set_image(url=image_url)

            # Compact footer with set and price
            footer_parts = [f"{card.set_name} ({card.set_code.upper()})"]
            price = card.get_price_display()
            if price:
                footer_parts.append(price)
            embed.set_footer(text=" · ".join(footer_parts))

            embeds.append(embed)

        # Report any failures
        if errors:
            error_embed = discord.Embed(
                description="\n".join(f"- {q}: not found" for q in errors),
                color=0xE74C3C,
            )
            embeds.append(error_embed)

        # Send in chunks of 10 (Discord embed limit per message)
        for i in range(0, len(embeds), 10):
            await message.channel.send(embeds=embeds[i : i + 10])

    def _has_filter_parameters(self, query: str) -> bool:
        """Check if the query contains Scryfall filter syntax."""
        essential_filters = [
            "e:",
            "s:",
            "set:",
            "frame:",
            "border:",
            "is:",
            "rarity:",
            "r:",
            "cn:",
            "number:",
            "c:",
            "color:",
            "id:",
            "t:",
            "type:",
            "o:",
            "oracle:",
            "pow:",
            "tou:",
            "cmc:",
            "mv:",
            "f:",
            "format:",
        ]

        lower_query = query.lower()
        return any(filter_param in lower_query for filter_param in essential_filters)

    def _extract_sort_parameters(
        self, query: str
    ) -> tuple[str, str | None, str | None]:
        """Extract order/dir hints from the query and return the cleaned query."""
        tokens = query.split()
        order: str | None = None
        direction: str | None = None
        remaining_tokens: list[str] = []

        for token in tokens:
            lower_token = token.lower()
            if lower_token.startswith(("order:", "sort:")):
                value = token.split(":", 1)[1].strip().strip("()[]{}.,;'\"")
                if value:
                    order = value.lower()
                continue
            if lower_token.startswith(("dir:", "direction:")):
                value = token.split(":", 1)[1].strip().strip("()[]{}.,;'\"")
                value_lower = value.lower()
                if value_lower in {"asc", "desc", "auto"}:
                    direction = value_lower
                continue
            remaining_tokens.append(token)

        cleaned_query = " ".join(remaining_tokens).strip()
        return cleaned_query, order, direction

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
                "*No exact match found for filters in "
                f"`{original_query}`, showing closest match*"
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

    def _user_error_message(
        self, exc: Exception, query: str, context: str = "lookup"
    ) -> str:
        """Return a user-friendly error message based on exception type."""
        if isinstance(exc, errors.MTGError):
            if exc.error_type == errors.ErrorType.NOT_FOUND:
                if context == "random" and query:
                    return (
                        f"No cards found matching filters: '{query}'. "
                        "Try broader criteria."
                    )
                if context == "random":
                    return (
                        "Scryfall API couldn't return a random card right now. "
                        "Please try again in a moment."
                    )
                if self._has_filter_parameters(query):
                    return (
                        f"No cards found for '{query}'. Try simpler "
                        "filters like `e:set` or `is:foil`, or check the spelling."
                    )
                return (
                    f"Card '{query}' not found. Try partial names "
                    "like 'bolt' for 'Lightning Bolt'."
                )
            if exc.error_type == errors.ErrorType.RATE_LIMIT:
                return "API rate limit exceeded. Please try again in a moment."
            if exc.error_type == errors.ErrorType.NETWORK:
                return "Could not reach the Scryfall API. Please try again in a moment."
            if exc.error_type == errors.ErrorType.API:
                return (
                    "The Scryfall API is temporarily unavailable. "
                    "Please try again in a moment."
                )
        return "Sorry, something went wrong. Please try again."

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
            description=(
                "**Fast Discord lookups with live pricing, legality, and rich embeds**"
            ),
            color=0x5865F2,
        )

        embed.add_field(
            name="Essential Commands",
            value=(
                f"`{prefix}lightning bolt` • single-card lookup\n"
                f"`[[Lightning Bolt]]` • bracket style lookup\n"
                f"`{prefix}rules counterspell` • official rulings\n"
                f"`{prefix}random` • pull a random card (supports filters)"
            ),
            inline=False,
        )

        embed.add_field(
            name="Filtering & Sorting",
            value=(
                "Mix Scryfall syntax directly in the query.\n"
                "• Sets: `e:mh3`, `s:ltr`\n"
                "• Showcase/foils: `is:showcase is:foil`\n"
                "• Sort results: `order:edhrec`, `order:usd dir:desc`\n"
                f"• Example: `{prefix}cultivate order:edhrec dir:desc`"
            ),
            inline=True,
        )

        embed.add_field(
            name="Multiple Cards",
            value=(
                "Semicolons resolve several cards in a grid.\n"
                f"`{prefix}bolt; counterspell; doom blade`\n"
                f"`{prefix}sol ring e:lea; mox ruby e:lea`"
            ),
            inline=True,
        )

        embed.add_field(
            name="Power Tips",
            value=(
                "• Fuzzy match typos: `ragav` ⇒ Ragavan\n"
                "• Rate limiting keeps chats friendly\n"
                "• Search pools pick varied prints unless you sort\n"
                f"• Aliases: `{prefix}r`, `{prefix}rand`, `{prefix}h`, `{prefix}?`"
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
            with suppress(asyncio.CancelledError):
                await self._cleanup_task

        # Close HTTP clients
        try:
            await self.scryfall_client.close()
        except Exception as e:
            self.logger.warning("Error closing scryfall client", error=str(e))

        # Clear duplicate suppression data
        self._recent_commands.clear()
        self._processed_message_ids.clear()
        self._user_rate_limits.clear()

        await super().close()
