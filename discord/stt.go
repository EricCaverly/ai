package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

const (
	//whisper_uri = "http://localhost:8080/transcribe/"
	whisper_uri          = "http://whisper.netv.local:8000/transcribe/"
	sentance_end_time_ms = 300
)

func createPionRTPPacket(p *discordgo.Packet) *rtp.Packet {
	return &rtp.Packet{
		Header: rtp.Header{
			Version: 2,
			// Taken from Discord voice docs
			PayloadType:    0x78,
			SequenceNumber: p.Sequence,
			Timestamp:      p.Timestamp,
			SSRC:           p.SSRC,
		},
		Payload: p.Opus,
	}
}

type Speech struct {
	file    media.Writer
	length  int
	endTime time.Time
}

type WhisperResp struct {
	Transcription string `json:"transcription"`
	Username      string `json:"username"`
}

func handleVoice(s *discordgo.Session, chan_id string, discord_packets chan *discordgo.Packet) {
	users := map[uint32]Speech{}
	var u_mut sync.Mutex
	var running bool = true

	go func() {
		for running {
			u_mut.Lock()
			for usr := range users {
				if users[usr].endTime.After(time.Now()) {
					continue
				}
				log.Printf("user %d stopped talking", usr)
				users[usr].file.Close()

				if users[usr].length > 10 {
					whisper_data, err := send_speech_to_whisper(usr)
					if err != nil {
						log.Printf("problem sending data to whisper: %s\n", err.Error())
					}

					if len(whisper_data.Transcription) > 0 {
						fmt.Printf("%s : %s\n", whisper_data.Username, whisper_data.Transcription)
						s.ChannelMessageSend(chan_id, whisper_data.Transcription)
					} else {
						fmt.Println("empty transcription returned")
					}

				}

				os.Remove(fmt.Sprintf("%d.ogg", usr))
				delete(users, usr)
			}
			u_mut.Unlock()
		}
	}()

	for p := range discord_packets {
		if len(p.Opus) < 4 {
			continue
		}

		u_mut.Lock()
		speech, ok := users[p.SSRC]
		if !ok {
			log.Printf("user %d started talking\n", p.SSRC)
			var err error
			speech.endTime = time.Now().Add(1 * time.Second)
			speech.file, err = oggwriter.New(fmt.Sprintf("%d.ogg", p.SSRC), 48000, 2)
			if err != nil {
				fmt.Printf("failed to create file %d.ogg, giving up on recording: %v\n", p.SSRC, err)
				return
			}
			users[p.SSRC] = speech
		}
		// Construct pion RTP packet from DiscordGo's type.
		rtp := createPionRTPPacket(p)
		err := speech.file.WriteRTP(rtp)
		if err != nil {
			fmt.Printf("failed to write to file %d.ogg, giving up on recording: %v\n", p.SSRC, err)
		}

		speech.length += len(p.Opus)
		speech.endTime = time.Now().Add(time.Millisecond * sentance_end_time_ms)
		users[p.SSRC] = speech
		u_mut.Unlock()
	}

	running = false

	// Once we made it here, we're done listening for packets. Close all files
	for _, u := range users {
		u.file.Close()
	}
}

func send_speech_to_whisper(SSRC uint32) (WhisperResp, error) {
	var output WhisperResp = WhisperResp{}

	client := http.Client{}
	resp, err := UploadMultipartFile(&client, whisper_uri, "file", fmt.Sprintf("%d.ogg", SSRC))
	if err != nil {
		return output, err
	}

	fmt.Println(resp.Status)

	bdy, err := io.ReadAll(resp.Body)
	if err != nil {
		return output, err
	}
	resp.Body.Close()

	err = json.Unmarshal(bdy, &output)
	if err != nil {
		return output, err
	}

	return output, nil
}

func UploadMultipartFile(client *http.Client, uri, key, path string) (*http.Response, error) {
	body, writer := io.Pipe()
	mwriter := multipart.NewWriter(writer)

	req, err := http.NewRequest(http.MethodPost, uri, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", mwriter.FormDataContentType())

	errchan := make(chan error)

	go func() {
		defer close(errchan)
		defer writer.Close()
		defer mwriter.Close()

		w, err := mwriter.CreateFormFile(key, path)
		if err != nil {
			errchan <- err
			return
		}

		in, err := os.Open(path)
		if err != nil {
			errchan <- err
			return
		}
		defer in.Close()

		if written, err := io.Copy(w, in); err != nil {
			errchan <- fmt.Errorf("error copying %s (%d bytes written): %v", path, written, err)
			return
		}

		if err := mwriter.Close(); err != nil {
			errchan <- err
			return
		}
	}()

	resp, err := client.Do(req)
	merr := <-errchan

	if err != nil || merr != nil {
		return resp, fmt.Errorf("http error: %v, multipart error: %v", err, merr)
	}

	return resp, nil
}
