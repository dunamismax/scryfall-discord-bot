package discord

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/dunamismax/MTG-Card-Bot/pkg/config"
	"github.com/dunamismax/MTG-Card-Bot/pkg/scryfall"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Bot struct {
	session         *discordgo.Session
	config          *config.Config
	scryfallClient  *scryfall.Client
	commandHandlers map[string]CommandHandler
}

type CommandHandler func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error

// NewBot creates a new Discord bot instance
func NewBot(cfg *config.Config, scryfallClient *scryfall.Client) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("creating Discord session: %w", err)
	}

	bot := &Bot{
		session:         session,
		config:          cfg,
		scryfallClient:  scryfallClient,
		commandHandlers: make(map[string]CommandHandler),
	}

	// Register command handlers
	bot.registerCommands()

	// Add message handler
	session.AddHandler(bot.messageCreate)

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	return bot, nil
}

// Start starts the Discord bot
func (b *Bot) Start() error {
	log.Printf("Starting %s bot...", b.config.BotName)

	err := b.session.Open()
	if err != nil {
		return fmt.Errorf("opening Discord session: %w", err)
	}

	log.Printf("Bot is now running as %s", b.session.State.User.Username)
	return nil
}

// Stop stops the Discord bot
func (b *Bot) Stop() error {
	log.Printf("Stopping %s bot...", b.config.BotName)
	return b.session.Close()
}

// registerCommands registers all command handlers
func (b *Bot) registerCommands() {
	b.commandHandlers["random"] = b.handleRandomCard
	// Card lookup is handled differently since it uses dynamic card names
}

// messageCreate handles incoming messages
func (b *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from bots
	if m.Author.Bot {
		return
	}

	// Check if message starts with command prefix
	if !strings.HasPrefix(m.Content, b.config.CommandPrefix) {
		return
	}

	// Remove prefix and split into command and args
	content := strings.TrimPrefix(m.Content, b.config.CommandPrefix)
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	// Handle specific commands
	if handler, exists := b.commandHandlers[command]; exists {
		if err := handler(s, m, args); err != nil {
			log.Printf("Error handling command '%s': %v", command, err)
			b.sendErrorMessage(s, m.ChannelID, "Sorry, something went wrong processing your command.")
		}
		return
	}

	// If no specific handler, treat it as a card lookup
	cardName := strings.Join(parts, " ")
	if err := b.handleCardLookup(s, m, cardName); err != nil {
		log.Printf("Error looking up card '%s': %v", cardName, err)
		b.sendErrorMessage(s, m.ChannelID, fmt.Sprintf("Sorry, I couldn't find a card named '%s'.", cardName))
	}
}

// handleRandomCard handles the !random command
func (b *Bot) handleRandomCard(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	log.Printf("Fetching random card for user %s", m.Author.Username)

	card, err := b.scryfallClient.GetRandomCard()
	if err != nil {
		return fmt.Errorf("fetching random card: %w", err)
	}

	return b.sendCardMessage(s, m.ChannelID, card)
}

// handleCardLookup handles card name lookup
func (b *Bot) handleCardLookup(s *discordgo.Session, m *discordgo.MessageCreate, cardName string) error {
	if cardName == "" {
		return fmt.Errorf("card name cannot be empty")
	}

	log.Printf("Looking up card '%s' for user %s", cardName, m.Author.Username)

	card, err := b.scryfallClient.GetCardByName(cardName)
	if err != nil {
		return fmt.Errorf("fetching card by name: %w", err)
	}

	return b.sendCardMessage(s, m.ChannelID, card)
}

// sendCardMessage sends a card image and details to a Discord channel
func (b *Bot) sendCardMessage(s *discordgo.Session, channelID string, card *scryfall.Card) error {
	if !card.IsValidCard() {
		return fmt.Errorf("invalid card data")
	}

	if !card.HasImage() {
		// Send text-only message if no image is available
		embed := &discordgo.MessageEmbed{
			Title:       card.GetDisplayName(),
			Description: fmt.Sprintf("**%s**\n%s", card.TypeLine, card.OracleText),
			Color:       0x9B59B6, // Purple color
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
		return err
	}

	// Get the highest quality image URL
	imageURL := card.GetBestImageURL()
	if imageURL == "" {
		return fmt.Errorf("no image available for card")
	}

	// Create rich embed with card image
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

	// Add mana cost if available
	if card.ManaCost != "" {
		embed.Description = fmt.Sprintf("**Mana Cost:** %s", card.ManaCost)
	}

	// Add artist if available
	if card.Artist != "" {
		embed.Footer.Text += fmt.Sprintf(" • Art by %s", card.Artist)
	}

	_, err := s.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// sendErrorMessage sends an error message to a Discord channel
func (b *Bot) sendErrorMessage(s *discordgo.Session, channelID, message string) {
	embed := &discordgo.MessageEmbed{
		Title:       "Error",
		Description: message,
		Color:       0xE74C3C, // Red color
	}

	if _, err := s.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.Printf("Failed to send error message: %v", err)
	}
}

// getRarityColor returns a color based on card rarity
func (b *Bot) getRarityColor(rarity string) int {
	switch strings.ToLower(rarity) {
	case "mythic":
		return 0xFF8C00 // Dark orange
	case "rare":
		return 0xFFD700 // Gold
	case "uncommon":
		return 0xC0C0C0 // Silver
	case "common":
		return 0x000000 // Black
	case "special":
		return 0xFF1493 // Deep pink
	case "bonus":
		return 0x9370DB // Medium purple
	default:
		return 0x9B59B6 // Default purple
	}
}
