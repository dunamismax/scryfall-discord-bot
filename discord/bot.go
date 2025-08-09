// Package discord provides Discord bot functionality for the MTG Card Bot.
package discord

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dunamismax/MTG-Card-Bot/cache"
	"github.com/dunamismax/MTG-Card-Bot/config"
	"github.com/dunamismax/MTG-Card-Bot/errors"
	"github.com/dunamismax/MTG-Card-Bot/logging"
	"github.com/dunamismax/MTG-Card-Bot/metrics"
	"github.com/dunamismax/MTG-Card-Bot/scryfall"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Bot represents a Discord bot instance with all necessary components.
type Bot struct {
	session         *discordgo.Session
	config          *config.Config
	scryfallClient  *scryfall.Client
	cache           *cache.CardCache
	commandHandlers map[string]CommandHandler
}

// CommandHandler represents a function that handles Discord bot commands.
type CommandHandler func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error

// NewBot creates a new Discord bot instance.
func NewBot(cfg *config.Config, scryfallClient *scryfall.Client, cardCache *cache.CardCache) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, errors.NewDiscordError("failed to create Discord session", err)
	}

	bot := &Bot{
		session:         session,
		config:          cfg,
		scryfallClient:  scryfallClient,
		cache:           cardCache,
		commandHandlers: make(map[string]CommandHandler),
	}

	// Register command handlers.
	bot.registerCommands()

	// Add message handler.
	session.AddHandler(bot.messageCreate)

	// Set intents.
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	return bot, nil
}

// Start starts the Discord bot.
func (b *Bot) Start() error {
	logger := logging.WithComponent("discord")
	logger.Info("Starting bot", "bot_name", b.config.BotName)

	err := b.session.Open()
	if err != nil {
		return errors.NewDiscordError("failed to open Discord session", err)
	}

	logger.Info("Bot is now running", "username", b.session.State.User.Username)

	return nil
}

// Stop stops the Discord bot.
func (b *Bot) Stop() error {
	logger := logging.WithComponent("discord")
	logger.Info("Stopping bot", "bot_name", b.config.BotName)

	if err := b.session.Close(); err != nil {
		return errors.NewDiscordError("failed to close Discord session", err)
	}

	return nil
}

// registerCommands registers all command handlers.
func (b *Bot) registerCommands() {
	b.commandHandlers["random"] = b.handleRandomCard
	b.commandHandlers["help"] = b.handleHelp
	b.commandHandlers["stats"] = b.handleStats
	b.commandHandlers["cache"] = b.handleCacheStats
	// Card lookup is handled differently since it uses dynamic card names.
}

// messageCreate handles incoming messages.
func (b *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from bots.
	if m.Author.Bot {
		return
	}

	// Check if message starts with command prefix.
	if !strings.HasPrefix(m.Content, b.config.CommandPrefix) {
		return
	}

	// Remove prefix and split into command and args.
	content := strings.TrimPrefix(m.Content, b.config.CommandPrefix)

	parts := strings.Fields(content)
	if len(parts) == 0 {
		return
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	// Handle specific commands.
	if handler, exists := b.commandHandlers[command]; exists {
		if err := handler(s, m, args); err != nil {
			logger := logging.WithComponent("discord").With(
				"user_id", m.Author.ID,
				"username", m.Author.Username,
				"command", command,
			)
			logging.LogError(logger, err, "Command execution failed")
			metrics.RecordCommand(false)
			metrics.RecordError(err)
			b.sendErrorMessage(s, m.ChannelID, "Sorry, something went wrong processing your command.")
		} else {
			metrics.RecordCommand(true)
			logging.LogDiscordCommand(m.Author.ID, m.Author.Username, command, true)
		}

		return
	}

	// If no specific handler, treat it as a card lookup.
	cardQuery := strings.Join(parts, " ")
	if err := b.handleCardLookup(s, m, cardQuery); err != nil {
		logger := logging.WithComponent("discord").With(
			"user_id", m.Author.ID,
			"username", m.Author.Username,
			"card_query", cardQuery,
		)
		logging.LogError(logger, err, "Card lookup failed")
		metrics.RecordCommand(false)
		metrics.RecordError(err)

		// Provide different error messages based on error type.
		if errors.IsErrorType(err, errors.ErrorTypeNotFound) {
			b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("Sorry, I couldn't find a card matching '%s'. Try using different keywords or check the spelling.", cardQuery))
		} else {
			b.sendErrorMessage(s, m.ChannelID, "Sorry, something went wrong while searching for that card.")
		}
	} else {
		metrics.RecordCommand(true)
		logging.LogDiscordCommand(m.Author.ID, m.Author.Username, "card_lookup", true)
	}
}

// handleRandomCard handles the !random command.
func (b *Bot) handleRandomCard(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	logger := logging.WithComponent("discord").With(
		"user_id", m.Author.ID,
		"username", m.Author.Username,
		"command", "random",
	)
	logger.Info("Fetching random card")

	card, err := b.scryfallClient.GetRandomCard()
	if err != nil {
		return errors.NewAPIError("failed to fetch random card", err)
	}

	return b.sendCardMessage(s, m.ChannelID, card, false, "")
}

// handleCardLookup handles card lookup with support for filtering parameters.
func (b *Bot) handleCardLookup(s *discordgo.Session, m *discordgo.MessageCreate, cardQuery string) error {
	if cardQuery == "" {
		return errors.NewValidationError("card query cannot be empty")
	}

	logger := logging.WithComponent("discord").With(
		"user_id", m.Author.ID,
		"username", m.Author.Username,
		"card_query", cardQuery,
	)
	logger.Info("Looking up card")

	// Check if query contains filter parameters
	hasFilters := b.hasFilterParameters(cardQuery)

	var (
		card         *scryfall.Card
		err          error
		usedFallback bool
	)

	if hasFilters {
		// Use search endpoint for filtered queries
		logger.Debug("Using search endpoint for filtered query")

		card, err = b.scryfallClient.SearchCardFirst(cardQuery)

		// If filtered search fails, extract card name and try fallback
		if err != nil {
			cardName := b.extractCardName(cardQuery)
			if cardName != "" {
				logger.Debug("Filtered search failed, trying fallback with card name", "fallback_name", cardName)

				card, err = b.cache.GetOrSet(cardName, func(name string) (*scryfall.Card, error) {
					return b.scryfallClient.GetCardByName(name)
				})
				if err == nil {
					usedFallback = true

					logger.Info("Fallback search successful", "original_query", cardQuery, "fallback_name", cardName)
					// Update cache metrics for the fallback lookup
					cacheStats := b.cache.Stats()
					metrics.Get().UpdateCacheStats(cacheStats.Hits, cacheStats.Misses, int64(cacheStats.Size))
				}
			}
		}
	} else {
		// Try to get from cache first for simple name lookups, then fetch from API if not found.
		card, err = b.cache.GetOrSet(cardQuery, func(name string) (*scryfall.Card, error) {
			return b.scryfallClient.GetCardByName(name)
		})
	}

	if err != nil {
		return errors.NewAPIError("failed to fetch card", err)
	}

	// Update cache metrics only for cached lookups
	if !hasFilters {
		cacheStats := b.cache.Stats()
		metrics.Get().UpdateCacheStats(cacheStats.Hits, cacheStats.Misses, int64(cacheStats.Size))
	}

	return b.sendCardMessage(s, m.ChannelID, card, usedFallback, cardQuery)
}

// hasFilterParameters checks if the query contains Scryfall filter syntax.
func (b *Bot) hasFilterParameters(query string) bool {
	filterPrefixes := []string{
		"frame:", "border:", "is:", "e:", "set:", "new:", "not:", "year:", "rarity:", "c:", "cmc:", "pow:", "tou:", "t:", "o:", "a:", "flavor:", "lore:", "function:", "unique:", "artist:", "watermark:", "stamp:", "foil", "nonfoil", "etched", "glossy", "textless", "fullart", "borderless", "colorshifted", "tombstone", "legendary", "reprint", "promo", "funny", "timeshifted",
	}

	lowerQuery := strings.ToLower(query)
	for _, prefix := range filterPrefixes {
		if strings.Contains(lowerQuery, prefix) {
			return true
		}
	}

	return false
}

// extractCardName attempts to extract the card name from a filtered query for fallback purposes.
func (b *Bot) extractCardName(query string) string {
	// Split query into words
	words := strings.Fields(query)

	var cardNameParts []string

	for _, word := range words {
		lowerWord := strings.ToLower(word)

		// Skip known filter patterns
		if strings.Contains(lowerWord, ":") {
			continue
		}

		// Skip standalone filter keywords that don't use colons
		filterKeywords := []string{"foil", "nonfoil", "etched", "glossy", "textless", "fullart", "borderless", "colorshifted", "tombstone", "legendary", "reprint", "promo", "funny", "timeshifted"}
		isFilterKeyword := false

		for _, keyword := range filterKeywords {
			if lowerWord == keyword {
				isFilterKeyword = true
				break
			}
		}

		if !isFilterKeyword {
			cardNameParts = append(cardNameParts, word)
		}
	}

	// Join the remaining parts to form the card name
	cardName := strings.Join(cardNameParts, " ")
	cardName = strings.TrimSpace(cardName)

	return cardName
}

// sendCardMessage sends a card image and details to a Discord channel.
func (b *Bot) sendCardMessage(s *discordgo.Session, channelID string, card *scryfall.Card, usedFallback bool, originalQuery string) error {
	if !card.IsValidCard() {
		return errors.NewValidationError("received invalid card data from API")
	}

	if !card.HasImage() {
		// Send text-only message if no image is available.
		embed := &discordgo.MessageEmbed{
			Title:       card.GetDisplayName(),
			Description: fmt.Sprintf("**%s**\n%s", card.TypeLine, card.OracleText),
			Color:       0x9B59B6, // Purple color.
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Set",
					Value:  fmt.Sprintf("%s (%s)", card.SetName, strings.ToUpper(card.SetCode)),
					Inline: true,
				},
				{
					Name:   "Rarity",
					Value:  cases.Title(language.English).String(card.Rarity),
					Inline: true,
				},
			},
			URL: card.ScryfallURI,
		}

		if card.Artist != "" {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "Artist",
				Value:  card.Artist,
				Inline: true,
			})
		}

		_, err := s.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			return errors.NewDiscordError("failed to send text-only card embed", err)
		}

		return nil
	}

	// Get the highest quality image URL.
	imageURL := card.GetBestImageURL()
	if imageURL == "" {
		return errors.NewValidationError("no image available for card")
	}

	// Create rich embed with card image.
	embed := &discordgo.MessageEmbed{
		Title: card.GetDisplayName(),
		URL:   card.ScryfallURI,
		Image: &discordgo.MessageEmbedImage{
			URL: imageURL,
		},
		Color: b.getRarityColor(card.Rarity),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%s • %s", card.SetName, cases.Title(language.English).String(card.Rarity)),
		},
	}

	// Add mana cost and fallback notification.
	var descriptions []string

	if usedFallback {
		descriptions = append(descriptions, fmt.Sprintf("⚠️ *No exact match found for filters in `%s`, showing closest match*", originalQuery))
	}

	if card.ManaCost != "" {
		descriptions = append(descriptions, fmt.Sprintf("**Mana Cost:** %s", card.ManaCost))
	}

	if len(descriptions) > 0 {
		embed.Description = strings.Join(descriptions, "\n")
	}

	// Add artist if available.
	if card.Artist != "" {
		embed.Footer.Text += fmt.Sprintf(" • Art by %s", card.Artist)
	}

	_, err := s.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		return errors.NewDiscordError("failed to send card embed with image", err)
	}

	return nil
}

// sendErrorMessage sends an error message to a Discord channel.
func (b *Bot) sendErrorMessage(s *discordgo.Session, channelID, message string) {
	embed := &discordgo.MessageEmbed{
		Title:       "Error",
		Description: message,
		Color:       0xE74C3C, // Red color.
	}

	if _, err := s.ChannelMessageSendEmbed(channelID, embed); err != nil {
		logger := logging.WithComponent("discord")
		logger.Error("Failed to send error message", "error", err)
	}
}

// getRarityColor returns a color based on card rarity.
func (b *Bot) getRarityColor(rarity string) int {
	switch strings.ToLower(rarity) {
	case "mythic":
		return 0xFF8C00 // Dark orange.
	case "rare":
		return 0xFFD700 // Gold.
	case "uncommon":
		return 0xC0C0C0 // Silver.
	case "common":
		return 0x000000 // Black.
	case "special":
		return 0xFF1493 // Deep pink.
	case "bonus":
		return 0x9370DB // Medium purple.
	default:
		return 0x9B59B6 // Default purple.
	}
}

// handleHelp handles the !help command.
func (b *Bot) handleHelp(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	logger := logging.WithComponent("discord").With(
		"user_id", m.Author.ID,
		"username", m.Author.Username,
		"command", "help",
	)
	logger.Info("Showing help information")

	embed := &discordgo.MessageEmbed{
		Title:       "MTG Card Bot Help",
		Description: "I can help you look up Magic: The Gathering cards with advanced filtering!",
		Color:       0x3498DB, // Blue color.
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Basic Commands",
				Value: fmt.Sprintf("`%s<card-name>` - Look up a card by name\n`%srandom` - Get a random card\n`%shelp` - Show this help\n`%sstats` - Show bot statistics",
					b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix),
				Inline: false,
			},
			{
				Name: "Advanced Filtering",
				Value: fmt.Sprintf("Use filters to find specific versions:\n`%sthe one ring border:borderless e:ltr` - Borderless One Ring from LOTR\n`%slightning bolt is:fullart e:sta` - Full art Lightning Bolt from Strixhaven Mystical Archive\n`%sblack lotus is:vintage` - Vintage Black Lotus\n`%smox ruby is:foil e:vma` - Foil Mox Ruby from Vintage Masters\n`%sancestral recall frame:1993` - Original frame Ancestral Recall",
					b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix),
				Inline: false,
			},
			{
				Name:   "Filter Types",
				Value:  "**Frame:** `frame:2015`, `frame:1997`, `frame:future`, `frame:1993`\n**Border:** `border:borderless`, `border:white`, `border:black`, `border:gold`\n**Finish:** `is:foil`, `is:nonfoil`, `is:etched`, `is:glossy`\n**Set:** `e:ltr`, `e:sta`, `e:vma`, `set:\"lord of the rings\"`\n**Art:** `is:fullart`, `new:art`, `is:showcase`, `is:textless`\n**Rarity:** `is:mythic`, `is:rare`, `is:uncommon`, `is:vintage`",
				Inline: false,
			},
			{
				Name:   "📖 Full Command Reference",
				Value:  "[Complete Command Cheat Sheet](https://github.com/dunamismax/mtg-card-bot/blob/main/docs/commands.md)",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "💡 Tip: Fuzzy matching works! Mix card names with filters for precise results.",
		},
	}

	_, err := s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return errors.NewDiscordError("failed to send help message", err)
	}

	return nil
}

// handleStats handles the !stats command.
func (b *Bot) handleStats(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	logger := logging.WithComponent("discord").With(
		"user_id", m.Author.ID,
		"username", m.Author.Username,
		"command", "stats",
	)
	logger.Info("Showing bot statistics")

	summary := metrics.Get().GetSummary()
	uptime := time.Duration(summary.UptimeSeconds * float64(time.Second))

	// Format uptime nicely.
	uptimeStr := formatDuration(uptime)

	embed := &discordgo.MessageEmbed{
		Title: "Bot Statistics",
		Color: 0x2ECC71, // Green color.
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "📊 Commands",
				Value: fmt.Sprintf("Total: %d\nSuccessful: %d\nFailed: %d\nSuccess Rate: %.1f%%",
					summary.CommandsTotal, summary.CommandsSuccessful, summary.CommandsFailed, summary.CommandSuccessRate),
				Inline: true,
			},
			{
				Name: "🌐 API Requests",
				Value: fmt.Sprintf("Total: %d\nSuccess Rate: %.1f%%\nAvg Response: %.0fms",
					summary.APIRequestsTotal, summary.APISuccessRate, summary.AverageResponseTime),
				Inline: true,
			},
			{
				Name: "💾 Cache Performance",
				Value: fmt.Sprintf("Size: %d cards\nHit Rate: %.1f%%\nHits: %d\nMisses: %d",
					summary.CacheSize, summary.CacheHitRate, summary.CacheHits, summary.CacheMisses),
				Inline: true,
			},
			{
				Name: "⚡ Performance",
				Value: fmt.Sprintf("Commands/sec: %.2f\nAPI Requests/sec: %.2f",
					summary.CommandsPerSecond, summary.APIRequestsPerSecond),
				Inline: true,
			},
			{
				Name:   "⏱️ Uptime",
				Value:  uptimeStr,
				Inline: true,
			},
			{
				Name:   "🚀 Started",
				Value:  fmt.Sprintf("<t:%d:R>", time.Now().Add(-uptime).Unix()),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Statistics since bot startup",
		},
	}

	// Add error information if there are errors.
	if len(summary.ErrorsByType) > 0 {
		errorInfo := make([]string, 0, len(summary.ErrorsByType))
		for errorType, count := range summary.ErrorsByType {
			if count > 0 {
				errorInfo = append(errorInfo, fmt.Sprintf("%s: %d", string(errorType), count))
			}
		}

		if len(errorInfo) > 0 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "⚠️ Errors",
				Value:  strings.Join(errorInfo, "\n"),
				Inline: false,
			})
		}
	}

	_, err := s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return errors.NewDiscordError("failed to send stats message", err)
	}

	return nil
}

// handleCacheStats handles the !cache command (detailed cache stats).
func (b *Bot) handleCacheStats(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	logger := logging.WithComponent("discord").With(
		"user_id", m.Author.ID,
		"username", m.Author.Username,
		"command", "cache",
	)
	logger.Info("Showing cache statistics")

	cacheStats := b.cache.Stats()

	embed := &discordgo.MessageEmbed{
		Title: "Cache Statistics",
		Color: 0xE67E22, // Orange color.
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "📦 Storage",
				Value: fmt.Sprintf("Size: %d / %d cards\nUtilization: %.1f%%",
					cacheStats.Size, cacheStats.MaxSize, float64(cacheStats.Size)/float64(cacheStats.MaxSize)*100),
				Inline: true,
			},
			{
				Name: "🎯 Performance",
				Value: fmt.Sprintf("Hit Rate: %.1f%%\nHits: %d\nMisses: %d",
					cacheStats.HitRate, cacheStats.Hits, cacheStats.Misses),
				Inline: true,
			},
			{
				Name: "♻️ Management",
				Value: fmt.Sprintf("Evictions: %d\nTTL: %v",
					cacheStats.Evictions, cacheStats.TTL),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Cache helps reduce API calls and improve response times",
		},
	}

	_, err := s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return errors.NewDiscordError("failed to send cache stats message", err)
	}

	return nil
}

// formatDuration formats a duration into a human-readable string.
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	case hours > 0:
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
