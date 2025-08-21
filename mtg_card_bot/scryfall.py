"""Scryfall API client for Magic: The Gathering card data."""

import asyncio
import random
import time
from typing import Any
from urllib.parse import quote

import httpx

from . import errors, logging


class Card:
    """Represents a Magic: The Gathering card from the Scryfall API."""

    def __init__(self, data: dict[str, Any]) -> None:
        self.object = data.get("object", "")
        self.id = data.get("id", "")
        self.oracle_id = data.get("oracle_id", "")
        self.name = data.get("name", "")
        self.lang = data.get("lang", "")
        self.released_at = data.get("released_at", "")
        self.uri = data.get("uri", "")
        self.scryfall_uri = data.get("scryfall_uri", "")
        self.layout = data.get("layout", "")
        self.image_uris = data.get("image_uris", {})
        self.card_faces = [CardFace(face) for face in data.get("card_faces", [])]
        self.mana_cost = data.get("mana_cost", "")
        self.cmc = data.get("cmc", 0)
        self.type_line = data.get("type_line", "")
        self.oracle_text = data.get("oracle_text", "")
        self.colors = data.get("colors", [])
        self.set_name = data.get("set_name", "")
        self.set_code = data.get("set", "")
        self.rarity = data.get("rarity", "")
        self.artist = data.get("artist", "")
        self.prices = data.get("prices", {})
        self.legalities = data.get("legalities", {})
        self.image_status = data.get("image_status", "")
        self.highres_image = data.get("highres_image", False)

    def get_best_image_url(self) -> str:
        """Get the highest quality image URL available for the card."""
        image_uris = self.image_uris

        # For double-faced cards, prefer the first face
        if self.card_faces and self.card_faces[0].image_uris:
            image_uris = self.card_faces[0].image_uris

        if not image_uris:
            return ""

        # Prefer highest quality images in order
        image_preference = ["png", "large", "normal", "small"]

        for format_type in image_preference:
            if format_type in image_uris:
                return image_uris[format_type]

        # Return any available image if none of the preferred formats exist
        return next(iter(image_uris.values()), "")

    def get_display_name(self) -> str:
        """Get the appropriate display name for the card."""
        if self.name:
            return self.name

        # For multi-faced cards without a combined name
        if self.card_faces:
            names = [face.name for face in self.card_faces]
            return " // ".join(names)

        return "Unknown Card"

    def is_valid_card(self) -> bool:
        """Check if the card has valid data for display."""
        return self.object == "card" and (self.name or self.card_faces)

    def has_image(self) -> bool:
        """Check if the card has at least one image available."""
        return bool(self.get_best_image_url())

    def get_price_display(self) -> str:
        """Get a formatted price string for display."""
        if not self.prices:
            return ""

        # Prioritize USD prices
        usd_price = self.prices.get("usd")
        usd_foil_price = self.prices.get("usd_foil")

        if usd_price:
            try:
                # Convert to float and format as currency
                price_float = float(usd_price)
                return f"${price_float:.2f}"
            except (ValueError, TypeError):
                pass

        if usd_foil_price:
            try:
                # Convert to float and format as currency
                price_float = float(usd_foil_price)
                return f"${price_float:.2f} (foil)"
            except (ValueError, TypeError):
                pass

        # Fallback to EUR if USD not available
        eur_price = self.prices.get("eur")
        if eur_price:
            try:
                price_float = float(eur_price)
                return f"€{price_float:.2f}"
            except (ValueError, TypeError):
                pass

        # Fallback to MTGO tickets
        tix_price = self.prices.get("tix")
        if tix_price:
            try:
                price_float = float(tix_price)
                return f"{price_float:.2f} tix"
            except (ValueError, TypeError):
                pass

        return ""

    def get_format_legalities(self) -> str:
        """Get a formatted string of format legalities."""
        if not self.legalities:
            return ""

        # Define format display order and names
        format_names = {
            "standard": "Standard",
            "pioneer": "Pioneer",
            "modern": "Modern",
            "legacy": "Legacy",
            "vintage": "Vintage",
            "commander": "Commander",
            "oathbreaker": "Oathbreaker",
            "brawl": "Brawl",
            "historic": "Historic",
            "pauper": "Pauper",
            "penny": "Penny",
            "duel": "Duel",
        }

        legal_formats = []
        for format_key, format_name in format_names.items():
            if self.legalities.get(format_key) == "legal":
                legal_formats.append(format_name)

        if not legal_formats:
            return "Not legal in any major formats"

        return ", ".join(legal_formats)


class CardFace:
    """Represents one face of a multi-faced card."""

    def __init__(self, data: dict[str, Any]) -> None:
        self.object = data.get("object", "")
        self.name = data.get("name", "")
        self.mana_cost = data.get("mana_cost", "")
        self.type_line = data.get("type_line", "")
        self.oracle_text = data.get("oracle_text", "")
        self.colors = data.get("colors", [])
        self.artist = data.get("artist", "")
        self.image_uris = data.get("image_uris", {})


class SearchResult:
    """Represents the result of a card search query."""

    def __init__(self, data: dict[str, Any]) -> None:
        self.object = data.get("object", "")
        self.total_cards = data.get("total_cards", 0)
        self.has_more = data.get("has_more", False)
        self.next_page = data.get("next_page", "")
        self.data = [Card(card_data) for card_data in data.get("data", [])]


class ScryfallError(Exception):
    """Error response from the Scryfall API."""

    def __init__(self, data: dict[str, Any]) -> None:
        self.object = data.get("object", "")
        self.code = data.get("code", "")
        self.status = data.get("status", 0)
        self.details = data.get("details", "")
        self.type = data.get("type", "")
        self.warnings = data.get("warnings", [])
        super().__init__(f"Scryfall API error: {self.details} (status: {self.status})")

    def get_error_type(self) -> errors.ErrorType:
        """Return the error type for metrics tracking."""
        if self.status == 404:
            return errors.ErrorType.NOT_FOUND
        if self.status == 429:
            return errors.ErrorType.RATE_LIMIT
        return errors.ErrorType.API


class ScryfallClient:
    """Client for interacting with the Scryfall API."""

    BASE_URL = "https://api.scryfall.com"
    USER_AGENT = "MTGCardBot/2.0"
    RATE_LIMIT = 0.05  # 50ms between requests (20 requests per second max)

    def __init__(self) -> None:
        self.client = httpx.AsyncClient(
            timeout=30.0,
            headers={
                "User-Agent": self.USER_AGENT,
                "Accept": "application/json",
            },
        )
        self.logger = logging.with_component("scryfall")
        self._last_request_time = 0.0

    async def close(self) -> None:
        """Close the HTTP client."""
        await self.client.aclose()

    async def _request(self, endpoint: str) -> httpx.Response:
        """Make a rate-limited request to the Scryfall API."""
        start_time = time.time()

        # Rate limiting
        time_since_last = start_time - self._last_request_time
        if time_since_last < self.RATE_LIMIT:
            await asyncio.sleep(self.RATE_LIMIT - time_since_last)

        self._last_request_time = time.time()

        url = f"{self.BASE_URL}{endpoint}"
        self.logger.debug("Making API request", endpoint=endpoint)

        try:
            response = await self.client.get(url)
            response_time = (time.time() - start_time) * 1000  # Convert to milliseconds

            if response.status_code >= 400:
                try:
                    error_data = response.json()
                    scryfall_error = ScryfallError(error_data)
                    raise scryfall_error
                except ValueError:
                    # Invalid JSON in error response
                    error = errors.create_error(
                        errors.ErrorType.API, f"HTTP error {response.status_code}"
                    )
                    raise error

            self.logger.debug(
                "API request successful",
                endpoint=endpoint,
                response_time_ms=response_time,
            )
            return response

        except httpx.RequestError as e:
            response_time = (time.time() - start_time) * 1000
            error = errors.create_error(
                errors.ErrorType.NETWORK, f"Request failed: {e}"
            )
            self.logger.error("API request failed", endpoint=endpoint, error=str(e))
            raise error

    async def get_card_by_name(self, name: str) -> Card:
        """Search for a card by name using fuzzy matching."""
        if not name:
            raise errors.create_error(
                errors.ErrorType.VALIDATION, "Card name cannot be empty"
            )

        self.logger.debug("Looking up card by name", card_name=name)
        endpoint = f"/cards/named?fuzzy={quote(name)}"

        response = await self._request(endpoint)
        data = response.json()
        card = Card(data)

        self.logger.debug("Successfully retrieved card", card_name=card.name)
        return card

    async def get_card_by_exact_name(self, name: str) -> Card:
        """Search for a card by exact name match."""
        if not name:
            raise errors.create_error(
                errors.ErrorType.VALIDATION, "Card name cannot be empty"
            )

        self.logger.debug("Looking up card by exact name", card_name=name)
        endpoint = f"/cards/named?exact={quote(name)}"

        response = await self._request(endpoint)
        data = response.json()
        card = Card(data)

        self.logger.debug(
            "Successfully retrieved card by exact name", card_name=card.name
        )
        return card

    async def get_random_card(self, query: str = "") -> Card:
        """Get a random Magic card, optionally filtered by search query."""
        if query:
            self.logger.debug("Fetching filtered random card", query=query)
            # For filtered queries, we need to search and then pick randomly
            # First, get the total count
            search_result = await self.search_cards(query)

            if search_result.total_cards == 0:
                raise errors.create_error(
                    errors.ErrorType.NOT_FOUND, f"No cards found matching '{query}'"
                )

            # Pick a random page if there are multiple pages
            cards_per_page = 175  # Scryfall's default page size
            max_page = min(
                10, (search_result.total_cards + cards_per_page - 1) // cards_per_page
            )  # Limit to first 10 pages for performance
            random_page = random.randint(1, max_page)

            # If we're on page 1 and already have the data, use it
            if random_page == 1 and search_result.data:
                cards = search_result.data
            else:
                # Fetch the random page
                endpoint = f"/cards/search?q={quote(query)}&page={random_page}"
                response = await self._request(endpoint)
                data = response.json()
                if not data.get("data"):
                    # Fallback to first page if random page fails
                    cards = search_result.data
                else:
                    cards = [Card(card_data) for card_data in data["data"]]

            # Pick a random card from the page
            if not cards:
                raise errors.create_error(
                    errors.ErrorType.NOT_FOUND, f"No cards found matching '{query}'"
                )

            card = random.choice(cards)
        else:
            self.logger.debug("Fetching random card")
            endpoint = "/cards/random"
            response = await self._request(endpoint)
            data = response.json()
            card = Card(data)

        self.logger.debug("Successfully retrieved random card", card_name=card.name)
        return card

    async def search_cards(self, query: str) -> SearchResult:
        """Perform a full-text search for cards."""
        if not query:
            raise errors.create_error(
                errors.ErrorType.VALIDATION, "Search query cannot be empty"
            )

        self.logger.debug("Searching cards", query=query)
        endpoint = f"/cards/search?q={quote(query)}"

        response = await self._request(endpoint)
        data = response.json()
        result = SearchResult(data)

        self.logger.debug(
            "Successfully searched cards", query=query, results=result.total_cards
        )
        return result

    async def search_card_first(self, query: str) -> Card:
        """Perform a search and return the first result."""
        if not query:
            raise errors.create_error(
                errors.ErrorType.VALIDATION, "Search query cannot be empty"
            )

        self.logger.debug("Searching for first card", query=query)

        # Add order by relevance for best results
        search_query = f"({query}) order:relevance"
        endpoint = f"/cards/search?q={quote(search_query)}"

        response = await self._request(endpoint)
        data = response.json()
        result = SearchResult(data)

        if result.total_cards == 0 or not result.data:
            raise errors.create_error(
                errors.ErrorType.NOT_FOUND, "No cards found matching query"
            )

        card = result.data[0]
        self.logger.debug(
            "Successfully found card via search",
            card_name=card.name,
            total_results=result.total_cards,
        )
        return card

    async def get_card_rulings(self, card_id: str) -> list[dict[str, Any]]:
        """Get rulings for a card by its Scryfall ID."""
        if not card_id:
            raise errors.create_error(
                errors.ErrorType.VALIDATION, "Card ID cannot be empty"
            )

        self.logger.debug("Fetching card rulings", card_id=card_id)
        endpoint = f"/cards/{card_id}/rulings"

        response = await self._request(endpoint)
        data = response.json()

        # Return the data array from the rulings response
        rulings = data.get("data", [])
        self.logger.debug(
            "Successfully retrieved card rulings",
            card_id=card_id,
            ruling_count=len(rulings),
        )
        return rulings
