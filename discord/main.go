package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func main() {
	// Read Token from File
	contents, err := os.ReadFile("./token.hide")
	if err != nil {
		log.Fatalf("problem reading token file: %s", err.Error())
	}

	token := strings.Trim(string(contents), "\n ")

	// Initialize Bot
	bot, err := discordgo.New("Bot " + token)

	// Handlers
	bot.AddHandler(messageCreate)

	// Intents
	bot.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates

	// Open the connection to discord
	err = bot.Open()
	if err != nil {
		log.Fatalf("error opening connection to discord: %s", err.Error())
	}

	log.Printf("bot is now running")

	// Stop logic
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	bot.Close()
}
