package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dunamismax/MTG-Card-Bot/cache"
	"github.com/dunamismax/MTG-Card-Bot/config"
	"github.com/dunamismax/MTG-Card-Bot/discord"
	"github.com/dunamismax/MTG-Card-Bot/logging"
	"github.com/dunamismax/MTG-Card-Bot/metrics"
	"github.com/dunamismax/MTG-Card-Bot/scryfall"
)

func main() {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		logging.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Validate configuration.
	if err := cfg.Validate(); err != nil {
		logging.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	// Initialize logging.
	logging.InitializeLogger(cfg.LogLevel, cfg.JSONLogging)

	// Initialize metrics.
	metrics.Initialize()

	// Log startup information.
	logging.LogStartup(cfg.BotName, cfg.CommandPrefix, cfg.LogLevel, cfg.DebugMode)

	if cfg.DebugMode {
		logging.Debug("Debug mode enabled")
	}

	// Create cache.
	cardCache := cache.NewCardCache(cfg.CacheTTL, cfg.CacheSize)
	logging.Info("Card cache initialized", "ttl", cfg.CacheTTL, "max_size", cfg.CacheSize)

	// Create Scryfall client.
	scryfallClient := scryfall.NewClient()

	logging.Info("Scryfall client initialized")

	// Create Discord bot.
	bot, err := discord.NewBot(cfg, scryfallClient, cardCache)
	if err != nil {
		logging.Error("Failed to create Discord bot", "error", err)
		os.Exit(1)
	}

	// Start the bot.
	if err := bot.Start(); err != nil {
		logging.Error("Failed to start Discord bot", "error", err)
		os.Exit(1)
	}

	// Print usage instructions.
	printUsageInstructions(cfg.CommandPrefix)

	// Setup graceful shutdown.
	gracefulShutdown(bot, scryfallClient, cardCache, cfg.ShutdownTimeout)
}

func printUsageInstructions(prefix string) {
	logger := logging.WithComponent("usage")
	logger.Info("MTG Card Bot Ready")
	logger.Info("Card lookup", "example", prefix+"<card-name>")
	logger.Info("Basic commands", "random", prefix+"random", "help", prefix+"help", "stats", prefix+"stats")
	logger.Info("Fuzzy matching enabled")
	logger.Info("Example queries", "partial", prefix+"bolt", "fuzzy", prefix+"jac bele", "filtered", prefix+"bolt frame:1993")
	logger.Info("Bot ready for Discord commands")
}

// gracefulShutdown handles graceful shutdown with timeout.
func gracefulShutdown(bot *discord.Bot, scryfallClient *scryfall.Client, _ *cache.CardCache, timeout time.Duration) {
	// Create a channel to receive OS signals.
	sigChan := make(chan os.Signal, 1)

	// Register the channel to receive specific signals.
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	logging.Info("Bot is running. Press Ctrl+C to stop.")

	// Wait for a signal.
	sig := <-sigChan
	logging.Info("Received signal, initiating graceful shutdown", "signal", sig.String())

	// Create a context with timeout for shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Channel to track shutdown completion.
	done := make(chan bool, 1)

	// Start shutdown process in a goroutine.
	go func() {
		defer func() { done <- true }()

		logging.Info("Closing Scryfall client...")
		scryfallClient.Close()

		logging.Info("Stopping Discord bot...")

		if err := bot.Stop(); err != nil {
			logging.Error("Error stopping Discord bot", "error", err)
		} else {
			logging.Info("Discord bot stopped successfully")
		}

		// Log final metrics.
		metricsSummary := metrics.Get().GetSummary()
		logging.Info("Final metrics", "commands_total", metricsSummary.CommandsTotal, "cache_hit_rate", metricsSummary.CacheHitRate)

		logging.Info("Cleanup completed")
	}()

	// Wait for shutdown to complete or timeout.
	select {
	case <-done:
		logging.Info("Graceful shutdown completed")
	case <-ctx.Done():
		logging.Warn("Shutdown timeout exceeded, forcing exit", "timeout", timeout)
	}

	// Give a moment for final log messages to be written.
	time.Sleep(100 * time.Millisecond)
	logging.LogShutdown()
}
