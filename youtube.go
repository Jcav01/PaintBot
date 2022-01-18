package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed/atom"
)

func setupYouTubeNotification(channel *streamInfo) {
	hub := &hub{
		Callback:     "https://paintbot.net/youtube",
		Mode:         "subscribe",
		Topic:        "https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + channel.UserId,
		LeaseSeconds: 604800,
	}
	body, _ := json.Marshal(hub)
	//log.Printf("Registering hub: %s", string(body))

	req, _ := http.NewRequest("POST", "https://pubsubhubbub.appspot.com/subscribe?hub.verify=async&hub.callback="+hub.Callback+"&hub.mode="+hub.Mode+"&hub.topic="+hub.Topic+"&hub.lease_seconds="+fmt.Sprint(hub.LeaseSeconds), bytes.NewBuffer(body))
	req.Header.Add("Content-type", "application/json")

	log.Println("Registering webhook for channel: " + channel.UserId)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Panic at webhook POST")
		panic(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(string(b))
	go renewWebhook(channel)
}

func renewWebhook(channel *streamInfo) {
	time.Sleep(144 * time.Hour)
	setupYouTubeNotification(channel)
}

func handleYoutubeNotification(w http.ResponseWriter, r *http.Request) (err error) {
	log.Printf("Handling notification: %v\n", r.URL)
	challenge := r.URL.Query().Get("hub.challenge")

	if challenge != "" {
		log.Printf("Challenge is: %v\n", challenge)
		w.Write([]byte(challenge))
	} else {
		w.WriteHeader(http.StatusNoContent)
		log.Printf("Responded to webhook\n")
		defer r.Body.Close()

		atomParser := atom.Parser{}
		feed, atomError := atomParser.Parse(r.Body)
		if err != nil {
			log.Println(atomError)
			return
		}
		log.Println(feed)

		channel := findChannel(feed.Entries[0].Extensions["yt"]["channelId"][0].Value, youtubeType)
		if channel == nil {
			return
		}

		if feed.Entries[0].PublishedParsed.Before(time.Now().UTC().Add(-24 * time.Hour)) {
			log.Printf("Video is older than 24 hours\n")
			return
		}

		for _, video := range channel.VideoIds {
			if video == feed.Entries[0].Extensions["yt"]["videoId"][0].Value {
				log.Printf("Video %v has already been posted\n", video)
				return
			}
		}

		discord := createDiscordSession()
		defer discord.Close()

		for _, entry := range feed.Entries {
			for _, discordChannel := range channel.Channels {
				discord.ChannelMessageSend(discordChannel.ChannelID, entry.Authors[0].Name+" has posted a new video: "+entry.Links[0].Href)
			}
			channel.VideoIds = append(channel.VideoIds, feed.Entries[0].Extensions["yt"]["videoId"][0].Value)
		}
		writeConfig()
	}
	return
}
