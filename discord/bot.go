// Package discord provides Discord bot functionality for the MTG Card Bot.
package discord

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
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

// multiResolved represents a resolved card query used for multi-card responses.
type multiResolved struct {
	query        string
	card         *scryfall.Card
	usedFallback bool
	err          error
}

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

	// Remove prefix
	content := strings.TrimPrefix(m.Content, b.config.CommandPrefix)

	// If the content contains semicolons, treat as multi-card lookup.
	if strings.Contains(content, ";") {
		if err := b.handleMultiCardLookup(s, m, content); err != nil {
			logger := logging.WithComponent("discord").With(
				"user_id", m.Author.ID,
				"username", m.Author.Username,
				"raw_query", content,
			)
			logging.LogError(logger, err, "Multi-card lookup failed")
			metrics.RecordCommand(false)
			metrics.RecordError(err)
			b.sendErrorMessage(s, m.ChannelID, "Sorry, something went wrong processing your multi-card query.")
		} else {
			metrics.RecordCommand(true)
			logging.LogDiscordCommand(m.Author.ID, m.Author.Username, "multi_card_lookup", true)
		}
		return
	}

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

		// Provide more helpful error messages based on error type
		switch {
		case errors.IsErrorType(err, errors.ErrorTypeNotFound):
			if b.hasFilterParameters(cardQuery) {
				b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("No cards found for '%s'. Try simpler filters like `e:set` or `is:foil`, or check the spelling.", cardQuery))
			} else {
				b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("Card '%s' not found. Try partial names like 'bolt' for 'Lightning Bolt'.", cardQuery))
			}
		case errors.IsErrorType(err, errors.ErrorTypeRateLimit):
			b.sendErrorMessage(s, m.ChannelID, "API rate limit exceeded. Please try again in a moment.")
		default:
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

	card, usedFallback, err := b.resolveCardQuery(cardQuery)
	if err != nil {
		return err
	}

	return b.sendCardMessage(s, m.ChannelID, card, usedFallback, cardQuery)
}

// resolveCardQuery encapsulates the logic to resolve a single card query into a card,
// applying caching, filter detection, and fallbacks consistent with single lookups.
func (b *Bot) resolveCardQuery(cardQuery string) (*scryfall.Card, bool, error) {
	cardQuery = strings.TrimSpace(cardQuery)
	hasFilters := b.hasFilterParameters(cardQuery)

	var (
		card         *scryfall.Card
		err          error
		usedFallback bool
	)

	if hasFilters {
		card, err = b.scryfallClient.SearchCardFirst(cardQuery)
		if err != nil {
			// If filtered search fails, extract card name and try fallback
			cardName := b.extractCardName(cardQuery)
			if cardName != "" && len(cardName) >= 2 {
				card, err = b.cache.GetOrSet(cardName, func(name string) (*scryfall.Card, error) {
					return b.scryfallClient.GetCardByName(name)
				})
				if err == nil {
					usedFallback = true
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
		if err == nil {
			cacheStats := b.cache.Stats()
			metrics.Get().UpdateCacheStats(cacheStats.Hits, cacheStats.Misses, int64(cacheStats.Size))
		}
	}

	if err != nil {
		return nil, false, errors.NewAPIError("failed to fetch card", err)
	}

	if card == nil {
		return nil, false, errors.NewValidationError("no card found for query")
	}

	return card, usedFallback, nil
}

// handleMultiCardLookup handles a semicolon-separated list of card queries, returning images in a grid.
func (b *Bot) handleMultiCardLookup(s *discordgo.Session, m *discordgo.MessageCreate, rawContent string) error {
	// Split on semicolons and trim spaces.
	rawParts := strings.Split(rawContent, ";")
	var queries []string
	for _, p := range rawParts {
		q := strings.TrimSpace(p)
		if q != "" {
			queries = append(queries, q)
		}
	}

	if len(queries) == 0 {
		return errors.NewValidationError("no valid card queries provided")
	}

	// If only one, fallback to normal flow.
	if len(queries) == 1 {
		return b.handleCardLookup(s, m, queries[0])
	}

	// Discord allows up to 10 attachments; we will group into grids of 4 for nicer layout.
	const maxPerMessage = 4

	// Resolve cards sequentially (Scryfall is rate-limited; keep it simple here).
	var all []multiResolved
	for _, q := range queries {
		card, usedFallback, err := b.resolveCardQuery(q)
		all = append(all, multiResolved{query: q, card: card, usedFallback: usedFallback, err: err})
	}

	// If all failed, return a combined error message.
	successCount := 0
	for _, r := range all {
		if r.err == nil && r.card != nil && r.card.IsValidCard() {
			successCount++
		}
	}
	if successCount == 0 {
		return errors.NewAPIError("failed to resolve any requested cards", fmt.Errorf("all lookups failed"))
	}

	// Chunk into groups and send as grids.
	for i := 0; i < len(all); i += maxPerMessage {
		end := i + maxPerMessage
		if end > len(all) {
			end = len(all)
		}
		chunk := all[i:end]
		if err := b.sendCardGridMessage(s, m.ChannelID, chunk); err != nil {
			return err
		}
	}

	return nil
}

// sendCardGridMessage fetches images and sends them as attachments in a single message for grid layout.
func (b *Bot) sendCardGridMessage(s *discordgo.Session, channelID string, items []multiResolved) error {
	// Prepare HTTP client for image downloads with timeouts.
	httpClient := &http.Client{Timeout: 20 * time.Second}

	var files []*discordgo.File
	var lines []string
	var mdLines []string
	for _, it := range items {
		if it.err != nil || it.card == nil || !it.card.IsValidCard() {
			lines = append(lines, fmt.Sprintf("%s: not found", it.query))
			continue
		}

		name := it.card.GetDisplayName()
		label := name
		if it.usedFallback {
			label += " (closest match)"
		}
		// Prepare human-friendly link lines
		if it.card.ScryfallURI != "" {
			// Text form (kept for fallback/plain messages)
			lines = append(lines, fmt.Sprintf("%s: %s", label, it.card.ScryfallURI))
			// Embed-friendly masked link
			mdLines = append(mdLines, fmt.Sprintf("- [%s](%s)", label, it.card.ScryfallURI))
		} else {
			lines = append(lines, label)
			mdLines = append(mdLines, fmt.Sprintf("- %s", label))
		}

		// Fetch image if available.
		if it.card.HasImage() {
			url := it.card.GetBestImageURL()
			data, filename, err := fetchImage(httpClient, url, safeFilename(name))
			if err != nil {
				// If image fetch fails, keep text line only.
				continue
			}
			files = append(files, &discordgo.File{
				Name:   filename,
				Reader: bytes.NewReader(data),
			})
		}
	}

	// Always send a compact embed with masked links for a clean look.
	embed := &discordgo.MessageEmbed{
		Title:       "Requested Cards",
		Description: strings.Join(mdLines, "\n"),
		Color:       0x5865F2,
	}

	// If no images, send just the embed; otherwise send embed first, then images
	if len(files) == 0 {
		_, err := s.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			return errors.NewDiscordError("failed to send multi-card embed message", err)
		}
		return nil
	}

	// Send the list embed first so it appears above the image grid
	if _, err := s.ChannelMessageSendEmbed(channelID, embed); err != nil {
		return errors.NewDiscordError("failed to send multi-card list embed", err)
	}

	// Then send the image grid as a separate message with only attachments
	if _, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Files: files}); err != nil {
		return errors.NewDiscordError("failed to send multi-card grid attachments", err)
	}
	return nil
}

// fetchImage downloads the image data, returning bytes and a reasonable filename.
func fetchImage(client *http.Client, url string, base string) ([]byte, string, error) {
	// Context with timeout per request.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "MTGCardBot-ImageFetcher/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Determine extension from Content-Type or URL.
	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		ext = ".jpg"
	default:
		// try URL hint
		if strings.HasSuffix(strings.ToLower(url), ".png") {
			ext = ".png"
		} else if strings.HasSuffix(strings.ToLower(url), ".jpg") || strings.HasSuffix(strings.ToLower(url), ".jpeg") {
			ext = ".jpg"
		}
	}

	filename := base + ext
	// Ensure extension is clean.
	filename = filepath.Base(filename)
	return data, filename, nil
}

var filenameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeFilename(name string) string {
	lower := strings.ToLower(name)
	replaced := filenameSanitizer.ReplaceAllString(lower, "-")
	replaced = strings.Trim(replaced, "-._")
	if replaced == "" {
		return "card"
	}
	if len(replaced) > 64 {
		return replaced[:64]
	}
	return replaced
}

// hasFilterParameters checks if the query contains essential Scryfall filter syntax.
func (b *Bot) hasFilterParameters(query string) bool {
	// Simplified essential filters - most commonly used and reliable
	essentialFilters := []string{
		"e:", "set:", "frame:", "border:", "is:foil", "is:nonfoil", "is:fullart", "is:textless", "is:borderless", "rarity:",
	}

	lowerQuery := strings.ToLower(query)
	for _, filter := range essentialFilters {
		if strings.Contains(lowerQuery, filter) {
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

		// Skip known filter patterns with colons
		if strings.Contains(lowerWord, ":") {
			continue
		}

		// Skip essential standalone filter keywords only
		essentialKeywords := []string{"foil", "nonfoil", "fullart", "textless", "borderless"}
		isFilterKeyword := false

		for _, keyword := range essentialKeywords {
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
		descriptions = append(descriptions, fmt.Sprintf("*No exact match found for filters in `%s`, showing closest match*", originalQuery))
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
		Description: "Look up cards, build grids, and filter versions.",
		Color:       0x3498DB,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Commands",
				Value: fmt.Sprintf("`%s<card>` – Look up a card\n`%s<card1>; <card2>; ...` – Grid lookup (up to 10)\n`%srandom` – Random card\n`%sstats` – Bot statistics\n`%scache` – Cache stats\n`%shelp` – This menu",
					b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix),
				Inline: false,
			},
			{
				Name: "Old-School Favorites (pre-2003)",
				Value: fmt.Sprintf("`%sblack lotus e:lea` – Alpha 1993\n`%sancestral recall e:lea` – Alpha 1993\n`%stime walk e:lea` – Alpha 1993\n`%ssol ring e:lea` – Alpha 1993",
					b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix, b.config.CommandPrefix,
				),
				Inline: false,
			},
			{
				Name: "Multi-Card Demo (4-card grid)",
				Value: fmt.Sprintf(
					"`%scity of brass e:arn; library of alexandria e:arn; juzam djinn e:arn; serendib efreet e:arn`",
					b.config.CommandPrefix,
				),
				Inline: false,
			},
			{
				Name: "More Classic Grids",
				Value: fmt.Sprintf("`%sshivan dragon e:lea; serra angel e:lea; lightning bolt e:lea; ancestral recall e:lea`\n`%sserra's sanctum e:usg; yawgmoth's will e:usg; wasteland e:tmp; necropotence e:ice`",
					b.config.CommandPrefix, b.config.CommandPrefix,
				),
				Inline: false,
			},
			{
				Name:   "Filters",
				Value:  "Set `e:lea|arn|leg|usg|tmp|ice` • Frame `frame:1993|1997` • Border `border:white` • Finish `is:foil|is:nonfoil` • Rarity `rarity:mythic|rare`",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Fuzzy and partial name matching supported.",
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
		Title:       "Bot Stats",
		Description: fmt.Sprintf("Uptime: %s • Started: <t:%d:R>", uptimeStr, time.Now().Add(-uptime).Unix()),
		Color:       0x2ECC71,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Commands",
				Value: fmt.Sprintf("Total: %d • Success: %.1f%% (%d/%d)",
					summary.CommandsTotal, summary.CommandSuccessRate, summary.CommandsSuccessful, summary.CommandsTotal),
				Inline: false,
			},
			{
				Name: "API",
				Value: fmt.Sprintf("Total: %d • Success: %.1f%% • Avg: %.0fms",
					summary.APIRequestsTotal, summary.APISuccessRate, summary.AverageResponseTime),
				Inline: false,
			},
			{
				Name: "Cache",
				Value: fmt.Sprintf("Size: %d • Hit Rate: %.1f%% (%d/%d)",
					summary.CacheSize, summary.CacheHitRate, summary.CacheHits, summary.CacheHits+summary.CacheMisses),
				Inline: false,
			},
			{
				Name: "Throughput",
				Value: fmt.Sprintf("Cmds/s: %.2f • API/s: %.2f",
					summary.CommandsPerSecond, summary.APIRequestsPerSecond),
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Live metrics since process start",
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
				Name:   "Error Summary",
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
		Title:       "Cache Performance Statistics",
		Description: "Card caching system metrics and utilization",
		Color:       0xE67E22,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Storage Utilization",
				Value: fmt.Sprintf("**Current Size:** %d cards\n**Maximum Size:** %d cards\n**Utilization:** %.1f%%",
					cacheStats.Size, cacheStats.MaxSize, float64(cacheStats.Size)/float64(cacheStats.MaxSize)*100),
				Inline: true,
			},
			{
				Name: "Hit Performance",
				Value: fmt.Sprintf("**Hit Rate:** %.1f%%\n**Cache Hits:** %d\n**Cache Misses:** %d",
					cacheStats.HitRate, cacheStats.Hits, cacheStats.Misses),
				Inline: true,
			},
			{
				Name: "Cache Management",
				Value: fmt.Sprintf("**Evictions:** %d\n**TTL Duration:** %v",
					cacheStats.Evictions, cacheStats.TTL),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Efficient caching reduces API calls and improves response times",
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
