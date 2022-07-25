package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/twitch"
)

var (
	twitchToken  *oauth2.Token
	oauth2Config *clientcredentials.Config
	client       *http.Client
	config       *cofiguration
)

const cfgFile string = "cfg.txt"

func main() {
	logFile, err := os.OpenFile("paintbot.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)

	loadConfig()
	//bytes, err := json.Marshal(config)
	//log.Println(string(bytes))
	generateToken()
	log.Println("Token Generated")
	client = &http.Client{}

	go startListen()

	subs := getSubscriptions("webhook_callback_verification_failed")

	for _, subToDelete := range subs.Data {
		deleteSubscription(subToDelete.ID)
	}

	enabledSubs := getSubscriptions("enabled")
	for _, currStream := range config.Streams {
		if currStream.Type == twitchType {
			log.Println(currStream.StreamName)
			if len(currStream.UserId) < 1 {
				currStream.UserId = getTwitchUser(currStream.StreamName).ID
			}
			subEnabled := false
			for _, sub := range enabledSubs.Data {
				if sub.Condition["broadcaster_user_id"] == currStream.UserId {
					subEnabled = true
					break
				}
			}
			if subEnabled {
				continue
			}
			registerTwitchWebhook(client, currStream.UserId, "stream.online")
			registerTwitchWebhook(client, currStream.UserId, "stream.offline")
			registerTwitchWebhook(client, currStream.UserId, "channel.update")
		} else if currStream.Type == youtubeType {
			setupYouTubeNotification(currStream)
		}
	}

	discord := createDiscordSession()
	errCheck("error retrieving account", err)

	discord.AddHandler(func(discord *discordgo.Session, ready *discordgo.Ready) {
		servers := discord.State.Guilds
		log.Printf("PaintBot has started on %d servers\n", len(servers))
	})

	log.Println(getSubscriptions("enabled"))

	err = discord.Open()
	errCheck("Error opening connection to Discord", err)
	defer discord.Close()

	<-make(chan struct{})
}

func createDiscordSession() *discordgo.Session {
	log.Println("Starting bot...")
	discord, err := discordgo.New("Bot " + config.Secrets.BotToken)
	errCheck("error creating discord session", err)
	log.Println("New session created...")
	return discord
}

func errCheck(msg string, err error) {
	if err != nil {
		log.Printf("%s: %+v", msg, err)
		panic(err)
	}
}

func loadConfig() {
	content, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatal(err)
	}

	json.Unmarshal(content, &config)

	for _, channel := range config.Streams {
		if channel.Type == twitchType {
			colour, err := strconv.ParseInt(channel.ColourString, 0, 64)
			if err != nil {
				log.Fatal(err)
				return
			}
			channel.HighlightColour = colour
		}
	}
}

func generateToken() {
	oauth2Config = &clientcredentials.Config{
		ClientID:     config.Secrets.TwitchClientID,
		ClientSecret: config.Secrets.TwitchClientSecret,
		TokenURL:     twitch.Endpoint.TokenURL,
	}

	token, err := oauth2Config.Token(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	twitchToken = token
}

func validateToken() {
	req, _ := http.NewRequest("GET", "https://id.twitch.tv/oauth2/validate", nil)
	req.Header.Add("Client-ID", config.Secrets.TwitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		log.Println("Token validated")
		return
	}
	generateToken()
}

func handleRoot(w http.ResponseWriter, r *http.Request) (err error) {
	w.Write([]byte("Hey bishes"))
	return
}
func handleTwitchNotification(w http.ResponseWriter, r *http.Request) (err error) {
	log.Printf("Handling notification: %v\n", r.Method)
	if r.Method != "POST" {
		log.Printf("Notification was not a POST: %v\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if r.Header.Get("Twitch-Eventsub-Message-Type") == "webhook_callback_verification" {
		body, _ := ioutil.ReadAll(r.Body)
		// if err != nil {
		// 	panic(err)
		// }
		var callbackVerification callbackVerification
		_ = json.Unmarshal(body, &callbackVerification)

		// if err != nil {
		// 	panic(err)
		// }
		w.Write([]byte(callbackVerification.Challenge))
		return
	}

	w.WriteHeader(http.StatusNoContent)
	log.Printf("Responded to webhook\n")
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
	}

	var twitchNotif notification
	err = json.Unmarshal(body, &twitchNotif)

	if err != nil {
		log.Println(err)
	}
	log.Println("Webhook notification for: ", twitchNotif.Event["broadcaster_user_name"], twitchNotif.SubscriptionInfo.Type)
	channel := findChannel(twitchNotif.Event["broadcaster_user_name"], twitchType)

	if twitchNotif.SubscriptionInfo.Type == "stream.online" {
		if len(channel.Title) == 0 {
			twitchChannel := getTwitchChannel(channel.UserId)
			channel.Title = twitchChannel.Title
			channel.Category = twitchChannel.GameID
		}
		onlineDate, _ := time.Parse(time.RFC3339, twitchNotif.Event["started_at"])

		if onlineDate.Unix()-channel.LastOffline > channel.OfflineTime {
			postNotification(channel)
		}
		channel.IsLive = true
	} else if twitchNotif.SubscriptionInfo.Type == "stream.offline" {
		if !channel.IsLive {
			log.Println("Channel is already offline, ignoring notification")
			return
		}
		channel.IsLive = false
		channel.LastOffline = time.Now().Unix()
		writeConfig()
	} else if twitchNotif.SubscriptionInfo.Type == "channel.update" {
		channel.Title = twitchNotif.Event["title"]
		channel.Category = twitchNotif.Event["category_id"]

		if channel.IsLive {
			go postNotification(channel)
		}
	}

	return
}

func startListen() {
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("paintbot.net"), //Your domain here
		Cache:      autocert.DirCache("certs"),             //Folder for storing certificates
		Email:      "jcav007@gmail.com",
	}

	server := &http.Server{
		Addr: ":https",
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	var middleware = func(h Handler) Handler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			// parse POST body, limit request size
			if err = r.ParseForm(); err != nil {
				log.Println("Something went wrong! Please try again.")
				return
			}

			return h(w, r)
		}
	}

	var errorHandling = func(handler func(w http.ResponseWriter, r *http.Request) error) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := handler(w, r); err != nil {
				var errorString string = "Something went wrong! Please try again."
				//var errorCode int = 500

				log.Println(err)
				w.Write([]byte(errorString))
				//w.WriteHeader(errorCode)
				return
			}
		})
	}

	var handleFunc = func(path string, handler Handler) {
		http.Handle(path, errorHandling(middleware(handler)))
	}
	handleFunc("/", handleRoot)
	handleFunc("/notify", handleTwitchNotification)
	handleFunc("/youtube", handleYoutubeNotification)

	go http.ListenAndServe(":http", certManager.HTTPHandler(nil))

	go log.Fatal(server.ListenAndServeTLS("", "")) //Key and cert are coming from Let's Encrypt
}

func postNotification(channel *streamInfo) {
	log.Println("Posting notification")
	user := getTwitchUser(channel.StreamName)

	var game *twitchGame
	if len(channel.Category) > 0 {
		game = getTwitchGame(channel.Category)
	}
	if game == nil {
		game = &twitchGame{
			Name:   "N/A",
			BoxArt: "https://images.igdb.com/igdb/image/upload/t_cover_big/nocover_qhhlj6.png",
		}
	}

	discord := createDiscordSession()
	defer discord.Close()
	message := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				URL:     "https://www.twitch.tv/" + channel.StreamName,
				Name:    channel.StreamName,
				IconURL: strings.Replace(strings.Replace(user.ProfileImage, "{width}", "70", 1), "{height}", "70", 1),
			},
			Color: int(channel.HighlightColour),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Game",
					Value:  game.Name,
					Inline: true,
				},
			},
			Image: &discordgo.MessageEmbedImage{
				URL: "https://static-cdn.jtvnw.net/previews-ttv/live_user_" + channel.StreamName + "-320x180.png" + "?r=" + time.Now().Format(time.RFC3339),
			},
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: strings.Replace(strings.Replace(game.BoxArt, "{width}", "500", 1), "{height}", "700", 1),
			},
			Title: channel.Title,
			URL:   "https://www.twitch.tv/" + channel.StreamName,
		},
	}

	if channel.Description != "" {
		message.Content = channel.Description
	}

	var msg *discordgo.Message
	var err error
	for i, channelID := range channel.Channels {
		if channel.IsLive {
			messageEdit := &discordgo.MessageEdit{
				ID:      channelID.MessageID,
				Channel: channelID.ChannelID,
				Content: &message.Content,
				Embed:   message.Embed,
			}
			msg, err = discord.ChannelMessageEditComplex(messageEdit)
		} else {
			msg, err = discord.ChannelMessageSendComplex(channelID.ChannelID, message)
		}

		if err != nil {
			log.Printf("%v did not send: %v\n", msg, err)
		} else {
			channel.Channels[i].MessageID = msg.ID
		}
	}
	writeConfig()
}

func writeConfig() {
	f, err := os.OpenFile("cfg.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	bytes, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}
	f.Write(bytes)
}

func findChannel(name string, channelType int) (channel *streamInfo) {
	for _, currChannel := range config.Streams {
		if (strings.EqualFold(currChannel.StreamName, name) || strings.EqualFold(currChannel.UserId, name)) && currChannel.Type == channelType {
			return currChannel
		}
	}
	return nil
}
