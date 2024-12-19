package main

import (
	"log"
	"net/url"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
)

func handleVoice(discord_packets chan *discordgo.Packet) {

	// Format URL
	u := url.URL{Scheme: "ws", Host: "vosk.netv.local:2700", Path: ""}
	log.Printf("connecting to %s\n", u.String())

	// Connect
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("error connecting to vosk server: %s\n", err.Error())
		return
	}
	defer c.Close()

	for p := range discord_packets {
		log.Printf("%#v\n", p)
	}
}
