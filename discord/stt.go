package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"gopkg.in/hraban/opus.v2"
)

type Message struct {
	Result []struct {
		Conf  float64
		End   float64
		Start float64
		Word  string
	}
	Text string
}

const (
	sample_rate = 48000
	channels    = 1
	frameSizeMs = 20
	frameSize   = channels * frameSizeMs * sample_rate / 1000
)

func handleVoice(discord_packets chan *discordgo.Packet) {

	// make decoder
	dec, err := opus.NewDecoder(sample_rate, channels)
	if err != nil {
		log.Fatalf("unable to make decoder: %s", err.Error())
	}

	// Format URL
	u := url.URL{Scheme: "ws", Host: "localhost:2700", Path: ""}
	log.Printf("connecting to %s\n", u.String())

	// Connect to websocket
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("error connecting to vosk server: %s\n", err.Error())
	}
	defer c.Close()
	log.Println("new session with vosk")

	// Send configuration
	c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"config":{"sample_rate" : %d }}`, sample_rate)))

	// Parse discord packets
	for p := range discord_packets {
		//log.Printf("%#v\n", p)

		// Convert discord frame into PCM
		pcm := make([]int16, int(frameSize))

		var n int
		if len(p.Opus) < 4 {
			err = dec.DecodePLC(pcm)
			if err != nil {
				log.Printf("Failed to decode empty data: %s\n", err.Error())
				continue
			}
		} else {
			n, err = dec.Decode(p.Opus, pcm)
			if err != nil {
				log.Printf("Failed to decode opus data: %s\n", err.Error())
				continue
			}
		}

		// Convert PCM to []byte
		pcmBytes := make([]byte, frameSize*2) // Each sample is 2 bytes (int16)
		buf := bytes.NewBuffer(pcmBytes)
		for _, sample := range pcm[:n] {
			// Write each int16 sample as 2 bytes in little-endian format
			buf.WriteByte(byte(sample))
			buf.WriteByte(byte(sample >> 8))
		}
		pcmBytes = buf.Bytes()

		err = c.WriteMessage(websocket.BinaryMessage, pcmBytes)
		if err != nil {
			log.Printf("Error writing to vosk: %s\n", err.Error())
			continue
		}

		_, resp, err := c.ReadMessage()
		if err != nil {
			log.Printf("Error getting response from vosk: %s\n", err.Error())
			continue
		}

		//log.Println(string(resp))

		var msg Message
		json.Unmarshal(resp, &msg)
		if msg.Text != "" {
			log.Println(msg.Text)
		}
	}
}
