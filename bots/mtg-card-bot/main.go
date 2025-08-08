package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dunamismax/MTG-Card-Bot/pkg/config"
	"github.com/dunamismax/MTG-Card-Bot/pkg/discord"
	"github.com/dunamismax/MTG-Card-Bot/pkg/scryfall"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting MTG Card Bot...")
	log.Printf("Bot Name: %s", cfg.BotName)
	log.Printf("Command Prefix: %s", cfg.CommandPrefix)
	log.Printf("Log Level: %s", cfg.LogLevel)
	log.Printf("Shutdown Timeout: %v", cfg.ShutdownTimeout)
	if cfg.DebugMode {
		log.Println("Debug mode enabled")
	}

	// Create Scryfall client
	scryfallClient := scryfall.NewClient()
	log.Println("Scryfall client initialized")

	// Create Discord bot
	bot, err := discord.NewBot(cfg, scryfallClient)
	if err != nil {
		log.Fatalf("Failed to create Discord bot: %v", err)
	}

	// Start the bot
	if err := bot.Start(); err != nil {
		log.Fatalf("Failed to start Discord bot: %v", err)
	}

	// Print usage instructions
	printUsageInstructions(cfg.CommandPrefix)

	// Setup graceful shutdown
	gracefulShutdown(bot, scryfallClient, cfg.ShutdownTimeout)
}

func printUsageInstructions(prefix string) {
	log.Println("")
	log.Println("=== MTG Card Bot Usage ===")
	log.Printf("%s<card-name> - Look up a Magic: The Gathering card by name", prefix)
	log.Printf("Example: %sthe-one-ring", prefix)
	log.Printf("Example: %sLightning Bolt", prefix)
	log.Printf("%srandom - Get a random Magic card", prefix)
	log.Println("")
	log.Println("The bot supports fuzzy matching, so partial names work!")
	log.Println("Examples of valid searches:")
	log.Printf("- %sjac bele (finds Jace Beleren)", prefix)
	log.Printf("- %sbol (finds Lightning Bolt)", prefix)
	log.Printf("- %sforce of will", prefix)
	log.Println("===========================")
	log.Println("")
}

// gracefulShutdown handles graceful shutdown with timeout
func gracefulShutdown(bot *discord.Bot, scryfallClient *scryfall.Client, timeout time.Duration) {
	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register the channel to receive specific signals
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	log.Println("Bot is running. Press Ctrl+C to stop.")

	// Wait for a signal
	sig := <-sigChan
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Channel to track shutdown completion
	done := make(chan bool, 1)

	// Start shutdown process in a goroutine
	go func() {
		defer func() { done <- true }()

		log.Println("Closing Scryfall client...")
		scryfallClient.Close()

		log.Println("Stopping Discord bot...")
		if err := bot.Stop(); err != nil {
			log.Printf("Error stopping Discord bot: %v", err)
		} else {
			log.Println("Discord bot stopped successfully")
		}

		log.Println("Cleanup completed")
	}()

	// Wait for shutdown to complete or timeout
	select {
	case <-done:
		log.Println("Graceful shutdown completed")
	case <-ctx.Done():
		log.Printf("Shutdown timeout exceeded (%v), forcing exit", timeout)
	}

	// Give a moment for final log messages to be written
	time.Sleep(100 * time.Millisecond)
	log.Println("Bot shutdown complete")
}
