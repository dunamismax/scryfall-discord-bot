# MTG Card Bot Commands Reference

This document provides a comprehensive guide to all commands and filtering options available with the MTG Card Bot.

📋 **[← Back to Main README](../README.md)** | 🏠 **[Project Home](https://github.com/dunamismax/mtg-card-bot)**

## Basic Commands

### Card Lookup

- **Syntax:** `!<card-name>`
- **Description:** Look up any Magic: The Gathering card by name
- **Examples:**
  - `!lightning bolt`
  - `!the one ring`
  - `!jace beleren`

### Random Card

- **Syntax:** `!random`
- **Description:** Get a random Magic: The Gathering card
- **Example:** `!random`

### Help

- **Syntax:** `!help`
- **Description:** Show basic help information and examples
- **Example:** `!help`

### Statistics

- **Syntax:** `!stats`
- **Description:** Show bot performance and usage statistics
- **Example:** `!stats`

### Cache Statistics

- **Syntax:** `!cache`
- **Description:** Show detailed cache performance statistics
- **Example:** `!cache`

## Advanced Filtering

The bot supports advanced filtering using Scryfall search syntax. You can combine card names with filters to find specific versions, art styles, frames, and editions.

### Frame Filters

Control the visual frame style of cards:

- **Modern Frame (2015):** `frame:2015`
- **8th Edition Frame (2003):** `frame:2003`
- **7th Edition Frame (1997):** `frame:1997`
- **Original Frame (1993):** `frame:1993`
- **Future Sight Frame:** `frame:future`
- **Legendary Frame:** `frame:legendary`
- **Colorshifted Frame:** `frame:colorshifted`
- **Tombstone Frame:** `frame:tombstone`

**Examples:**

- `!lightning bolt frame:1993` - Original frame Lightning Bolt
- `!sol ring frame:2015` - Modern frame Sol Ring
- `!akroma frame:future` - Future sight frame Akroma

### Border Filters

Filter by border color and style:

- **Black Border:** `border:black`
- **White Border:** `border:white`
- **Silver Border:** `border:silver`
- **Borderless:** `border:borderless`

**Examples:**

- `!lightning bolt border:white` - White border Lightning Bolt
- `!the one ring border:borderless` - Borderless The One Ring
- `!brainstorm border:black` - Black border Brainstorm

### Finish and Treatment Filters

Find cards with specific finishes and treatments:

- **Foil:** `is:foil`
- **Non-foil:** `is:nonfoil`
- **Etched:** `is:etched`
- **Glossy:** `is:glossy`
- **Available in both foil and non-foil:** `is:foil is:nonfoil`

**Examples:**

- `!lightning bolt is:foil` - Foil Lightning Bolt
- `!brainstorm is:etched` - Etched Brainstorm
- `!sol ring is:nonfoil` - Non-foil Sol Ring

### Art and Visual Style Filters

Find cards with specific visual treatments:

- **Full Art:** `is:fullart`
- **Textless:** `is:textless`
- **New Art:** `new:art`
- **Promo:** `is:promo`
- **Borderless:** `border:borderless` (same as border filter)

**Examples:**

- `!lightning bolt is:fullart` - Full art Lightning Bolt
- `!brainstorm new:art` - Brainstorm with new artwork
- `!sol ring is:promo` - Promotional Sol Ring

### Set and Edition Filters

Find cards from specific sets or editions:

- **Set Code:** `e:setcode` or `set:setcode`
- **Set Name:** `set:"Set Name"`
- **Year:** `year:2023`
- **Reprint Status:** `is:reprint` or `not:reprint`

**Popular Set Codes:**

- `e:lea` - Limited Edition Alpha
- `e:leb` - Limited Edition Beta
- `e:2ed` - Unlimited Edition
- `e:m21` - Core Set 2021
- `e:stx` - Strixhaven
- `e:neo` - Kamigawa: Neon Dynasty
- `e:snc` - Streets of New Capenna
- `e:dom` - Dominaria
- `e:war` - War of the Spark
- `e:eld` - Throne of Eldraine

**Examples:**

- `!lightning bolt e:lea` - Lightning Bolt from Alpha
- `!brainstorm set:"eternal masters"` - Brainstorm from Eternal Masters
- `!sol ring year:2020` - Sol Ring printed in 2020
- `!counterspell not:reprint` - Original printing of Counterspell

### Combining Filters

You can combine multiple filters to find very specific card versions:

**Examples:**

- `!lightning bolt frame:1993 e:lea` - Alpha Lightning Bolt with original frame
- `!brainstorm is:foil border:borderless` - Foil borderless Brainstorm
- `!sol ring frame:2015 is:nonfoil e:c21` - Modern frame non-foil Sol Ring from Commander 2021
- `!the one ring border:borderless is:foil` - Foil borderless The One Ring
- `!counterspell frame:1997 is:foil e:7ed` - 7th Edition frame foil Counterspell

### Card Property Filters

Search by specific card properties:

- **Color:** `c:red`, `c:blue`, `c:green`, `c:white`, `c:black`, `c:colorless`
- **Mana Cost:** `cmc:3`, `cmc>=4`, `cmc<=2`
- **Power/Toughness:** `pow:2`, `tou:3`, `pow>=5`
- **Type:** `t:creature`, `t:instant`, `t:sorcery`, `t:artifact`
- **Rarity:** `r:common`, `r:uncommon`, `r:rare`, `r:mythic`
- **Artist:** `a:"artist name"`

**Examples:**

- `!lightning bolt c:red r:common` - Red common Lightning Bolt
- `!counterspell t:instant cmc:2` - 2 mana instant Counterspell
- `!sol ring t:artifact cmc:1` - 1 mana artifact Sol Ring

## Tips and Best Practices

### Fuzzy Matching

The bot supports fuzzy matching, so you don't need to type exact card names:

- `!jac bele` finds "Jace Beleren"
- `!bol` finds "Lightning Bolt"
- `!force will` finds "Force of Will"

### Quotation Marks

Use quotes for multi-word set names or when you need exact matches:

- `set:"eternal masters"`
- `a:"rebecca guay"`

### Fallback Behavior

If no card matches your exact filters, the bot will attempt to find the closest match or default version of the card.

### Performance Notes

- Simple name lookups are cached for faster responses
- Filtered searches bypass the cache and query the API directly
- The bot respects Scryfall's rate limits automatically

## Troubleshooting

### No Results Found

If the bot can't find a card:

1. Check your spelling
2. Try using fewer or different filters
3. Use fuzzy matching (partial names)
4. Check if the card exists in the specified set

### Wrong Card Version

If you get a different version than expected:

1. Add more specific filters
2. Check the set code is correct
3. Verify the frame/border combination exists

### Rate Limiting

The bot automatically handles rate limiting, but during heavy usage you might experience slight delays.

## Support

For issues or feature requests:

- GitHub: [https://github.com/dunamismax/mtg-card-bot](https://github.com/dunamismax/mtg-card-bot)
- Use the `!stats` command to check bot health
- All searches are logged for debugging purposes

---

*This bot uses the Scryfall API. For more details on search syntax, visit [Scryfall's official documentation](https://scryfall.com/docs/syntax).*
