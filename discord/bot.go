package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
)

const (
	bot_msg_prefix = '!'
)

type VoiceSession struct {
	connection *discordgo.VoiceConnection
	guild_id   string
}

var (
	calls        []VoiceSession
	calls_mutx   sync.Mutex
	ssrc_to_user = make(map[uint32]string)
)

// Message Created Handler
// - Called when a message is sent in any guild
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Dont do anything to messages sent by botself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// If the message does not have any content (images for example) do nothing
	if len(m.Content) == 0 {
		return
	}

	// If the first character is not the bot message prefix, ignore
	if m.Content[0] != bot_msg_prefix {
		return
	}

	// Switch based on the command
	switch m.Content[1:] {
	case "join":
		join_voice(s, m)
	case "leave":
		leave_voice(m.GuildID)
	}
}

func leave_voice(guild_id string) {
	calls_mutx.Lock()
	// Find the voice call which relates to the guild the message was sent in
	var i int
	for i < len(calls) {
		// If we found it, leave the voice call, destroying the call object
		if calls[i].guild_id == guild_id {
			calls[i].connection.Disconnect()
			close(calls[i].connection.OpusRecv)
			calls[i].connection.Close()
			calls = append(calls[:i], calls[i+1:]...)
		}
		i++
	}
	calls_mutx.Unlock()
}

func join_voice(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Find which voice channel the author of the message is in
	g, vs, err := find_vc(s, m)
	if err != nil {
		log.Printf("Problem finding VC\n")
		return
	}

	// Join that voice call
	vc, err := s.ChannelVoiceJoin(g.ID, vs.ChannelID, true, false)
	if err != nil {
		log.Printf("Failed to join VC: %s\n", err.Error())
	}

	// Make a new entry in the calls slice containing a reference to the call
	calls_mutx.Lock()
	calls = append(calls, VoiceSession{
		connection: vc,
		guild_id:   g.ID,
	})
	calls_mutx.Unlock()

	// Begin handling voice call related logic for transcription
	handleVoice(s, m.ChannelID, vc.OpusRecv)
}

func find_vc(s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.Guild, *discordgo.VoiceState, error) {
	// Find Chanel the message was sent in
	c, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return nil, nil, err
	}

	// Find guild the channel is within
	g, err := s.State.Guild(c.GuildID)
	if err != nil {
		return nil, nil, err
	}

	// Find voice state the author is in
	for _, vs := range g.VoiceStates {

		// If the author is in a voice call, return a reference to it
		if vs.UserID == m.Author.ID {
			return g, vs, nil
		}
	}

	// Otherwise, return an error
	return nil, nil, fmt.Errorf("unable to find voice call")
}
