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
	ssrc    uint32
	endTime time.Time
}

type WhisperResp struct {
	Transcription string `json:"transcription"`
	Username      string `json:"username"`
}

func handleVoice(s *discordgo.Session, chan_id string, discord_packets chan *discordgo.Packet) {
	users := []Speech{}
	var u_mut sync.Mutex
	var running bool = true

	go func() {
		for running {
			u_mut.Lock()
			// Go over each user is currently speaking users
			for i := range users {
				// If their sentance has not timed out, skip
				if users[i].endTime.After(time.Now()) {
					continue
				}

				// Otherwise, they have not made noise for sentance_end_time_ms and are probably done talking
				log.Printf("user %d stopped talking", i)

				// Close the OGG file
				users[i].file.Close()

				// If the number of audio packets is greater than 10, this is probably real data
				if users[i].length > 10 {
					// Call the send function, which will call the whisper API, returning transcription data
					whisper_data, err := send_speech_to_whisper(users[i].ssrc)
					if err != nil {
						log.Printf("problem sending data to whisper: %s\n", err.Error())
					}

					// If the length of the transcription is not 0, actual text was received
					if len(whisper_data.Transcription) > 0 {
						fmt.Printf("%s : %s\n", whisper_data.Username, whisper_data.Transcription)
						s.ChannelMessageSend(chan_id, whisper_data.Transcription)
						// Otherwise, if the length is 0, likely it was just hot mic, and should not be sent to LLM
					} else {
						fmt.Println("empty transcription returned")
					}
				}

				// Remove the OGG file
				os.Remove(fmt.Sprintf("%d.ogg", i))

				// Remove the member from the actively speaking users slice
				users = append(users[:i], users[i+1:]...)
			}

			u_mut.Unlock()
		}
	}()

	for p := range discord_packets {
		if len(p.Opus) < 4 {
			continue
		}

		u_mut.Lock()

		// See if the user who is talking has already been talking
		// If i is never changed from -1, we know this is a user who just started talking
		var i int = -1
		for j := range users {
			if users[j].ssrc == p.SSRC {
				i = j
			}
		}

		// Create a new entry for this user who just started talking
		if i == -1 {
			log.Printf("user %d started talking\n", p.SSRC)
			// Create new member of users talking slice
			users = append(users, Speech{
				endTime: time.Now().Add(500 * time.Millisecond),
				ssrc:    p.SSRC,
				length:  0,
			})
			i = len(users) - 1

			// Open a new OGG file
			var err error
			users[i].file, err = oggwriter.New(fmt.Sprintf("%d.ogg", p.SSRC), 48000, 2)
			if err != nil {
				fmt.Printf("failed to create file %d.ogg, giving up on recording: %v\n", p.SSRC, err)
				return
			}
		}

		// Construct pion RTP packet from DiscordGo's type.
		rtp := createPionRTPPacket(p)
		err := users[i].file.WriteRTP(rtp)
		if err != nil {
			fmt.Printf("failed to write to file %d.ogg, giving up on recording: %v\n", p.SSRC, err)
		}

		// Increase the tracked length of the audio, used to prevent sending super short files which are typically bogus
		users[i].length += len(p.Opus)

		// Increase time till end of sentance
		users[i].endTime = time.Now().Add(time.Millisecond * sentance_end_time_ms)

		u_mut.Unlock()
	}

	// Stop goroutine
	running = false

	// Once we made it here, we're done listening for packets. Close all files
	for _, u := range users {
		u.file.Close()
	}
}

func send_speech_to_whisper(ssrc uint32) (WhisperResp, error) {
	var output WhisperResp = WhisperResp{}

	// Create a default HTTP client, no timeout
	client := http.Client{}

	// Upload the ogg file
	log.Printf("Attempting to upload OGG file to Whsiper\n")
	resp, err := UploadMultipartFile(&client, whisper_uri, "file", fmt.Sprintf("%d.ogg", ssrc))
	if err != nil {
		return output, err
	}
	log.Printf("Response: %s\n", resp.Status)

	// Read the response body and then close it
	bdy, err := io.ReadAll(resp.Body)
	if err != nil {
		return output, err
	}
	resp.Body.Close()

	// Convert raw bytes into Response Struct
	err = json.Unmarshal(bdy, &output)
	if err != nil {
		return output, err
	}

	// Return response struct
	return output, nil
}

// Stolen from Stackoverflow
// https://stackoverflow.com/questions/20205796/post-data-using-the-content-type-multipart-form-data

func UploadMultipartFile(client *http.Client, uri, key, path string) (*http.Response, error) {
	body, writer := io.Pipe()
	mwriter := multipart.NewWriter(writer)

	// Generate new HTTP POST Request with a pipe as the body
	req, err := http.NewRequest(http.MethodPost, uri, body)
	if err != nil {
		return nil, err
	}

	// Add a header for content type of multi-part form
	req.Header.Add("Content-Type", mwriter.FormDataContentType())

	errchan := make(chan error)

	// Asynchronously read and "pipe" the file contents into the request body
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

	// Start making the request
	resp, err := client.Do(req)
	merr := <-errchan

	if err != nil || merr != nil {
		return resp, fmt.Errorf("http error: %v, multipart error: %v", err, merr)
	}

	return resp, nil
}
