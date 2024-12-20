package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type VoiceSessions struct {
	vc   []*discordgo.VoiceConnection
	mutx sync.Mutex
}

var (
	Calls        VoiceSessions
	ssrc_to_user = make(map[uint32]string)
)

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content[0] != '!' {
		return
	}

	switch m.Content[1:] {
	case "join":
		join_voice(s, m)
	case "leave":
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
	Calls.vc = []*discordgo.VoiceConnection{}
	Calls.mutx.Unlock()
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

	handleVoice(s, m.ChannelID, vc.OpusRecv)
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
