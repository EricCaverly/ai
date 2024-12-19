package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

type VoiceSessions struct {
	vc   []*discordgo.VoiceConnection
	mutx sync.Mutex
}

var (
	Calls VoiceSessions
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

func handleVoice(c chan *discordgo.Packet) {
	for p := range c {
		log.Printf("%#v\n", p)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "ping" {
		join_voice(s, m)
	} else if m.Content == "leave" {
		leave_voice()
	}
}

func leave_voice() {
	Calls.mutx.Lock()
	for _, c := range Calls.vc {
		c.Disconnect()
		close(c.OpusRecv)
		c.Close()
	}
}

func join_voice(s *discordgo.Session, m *discordgo.MessageCreate) {

	g, vs, err := find_vc(s, m)
	if err != nil {
		log.Printf("Problem finding VC\n")
		return
	}

	vc, err := s.ChannelVoiceJoin(g.ID, vs.ChannelID, true, false)
	if err != nil {
		log.Printf("Failed to join VC: %s\n", err.Error())
	}

	Calls.mutx.Lock()
	Calls.vc = append(Calls.vc, vc)
	Calls.mutx.Unlock()

	handleVoice(vc.OpusRecv)
}

func find_vc(s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.Guild, *discordgo.VoiceState, error) {
	// Find Chanel
	c, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return nil, nil, err
	}

	// Find guild
	g, err := s.State.Guild(c.GuildID)
	if err != nil {
		return nil, nil, err
	}

	for _, vs := range g.VoiceStates {
		if vs.UserID == m.Author.ID {
			return g, vs, nil
		}
	}

	return nil, nil, fmt.Errorf("unable to find voice call")
}
