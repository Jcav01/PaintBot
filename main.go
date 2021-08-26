package main

import (
	"bytes"
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

type createSubscription struct {
	EventType string            `json:"type"`
	Version   string            `json:"version"`
	Condition map[string]string `json:"condition"`
	Transport transport         `json:"transport"`
}
type transport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

type twitchUser struct {
	ID              string `json:"id"`
	Login           string `json:"login"`
	DisplayName     string `json:"display_name"`
	UserType        string `json:"type"`
	BroadcasterType string `json:"broadcaster_type"`
	Description     string `json:"description"`
	ProfileImage    string `json:"profile_image_url"`
	OfflineImage    string `json:"offline_image_url"`
	ViewCount       int    `json:"view_count"`
	Email           string `json:"email"`
}
type twitchUserJSON struct {
	Users []twitchUser `json:"data"`
}

type twitchChannel struct {
	ID          string `json:"broadcaster_id"`
	Login       string `json:"broadcaster_login"`
	DisplayName string `json:"broadcaster_name"`
	Language    string `json:"broadcaster_language"`
	GameID      string `json:"game_id"`
	GameName    string `json:"game_name"`
	Title       string `json:"title"`
	Delay       int    `json:"delay"`
}
type twitchChannelJSON struct {
	Channel []twitchChannel `json:"data"`
}

type subscriptionInfo struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Cost      int               `json:"cost"`
	Condition map[string]string `json:"condition"`
	Transport map[string]string `json:"transport"`
	CreatedAt string            `json:"created_at"`
}
type notification struct {
	SubscriptionInfo subscriptionInfo  `json:"subscription"`
	Event            map[string]string `json:"event"`
}
type callbackVerification struct {
	SubscriptionInfo subscriptionInfo `json:"subscription"`
	Challenge        string           `json:"challenge"`
}

type twitchGame struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	BoxArt string `json:"box_art_url"`
}
type twitchGameJSON struct {
	Games []twitchGame `json:"data"`
}

type discordChannel struct {
	ChannelID string `json:"id"`
	MessageID string `json:"message_id"`
}

type streamInfo struct {
	StreamName      string           `json:"stream_name"`
	Channels        []discordChannel `json:"discord_channel_ids"`
	ColourString    string           `json:"colour"`
	HighlightColour int64            `json:"highlight_colour"`
	CurrentStreamID string           `json:"current_stream"`
	Description     string           `json:"description"`
}

type secrets struct {
	BotToken           string `json:"bot_token"`
	TwitchClientID     string `json:"twitch_client_id"`
	TwitchClientSecret string `json:"twitch_client_secret"`
}

type cofiguration struct {
	Secrets secrets       `json:"secrets"`
	Streams []*streamInfo `json:"streams"`
}

type Handler func(http.ResponseWriter, *http.Request) error

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

	log.Println(config.Streams[0].StreamName)
	registerWebhook(client, config.Streams[0].StreamName)

	discord := createDiscordSession()
	errCheck("error retrieving account", err)

	discord.AddHandler(func(discord *discordgo.Session, ready *discordgo.Ready) {
		servers := discord.State.Guilds
		log.Printf("PaintBot has started on %d servers\n", len(servers))
	})

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
		colour, err := strconv.ParseInt(channel.ColourString, 0, 64)
		if err != nil {
			log.Fatal(err)
			return
		}
		channel.HighlightColour = colour
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
	req, err := http.NewRequest("GET", "https://id.twitch.tv/oauth2/validate", nil)
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
func handleNotification(w http.ResponseWriter, r *http.Request) (err error) {
	log.Printf("Handling notification: %v\n", r.URL)
	if r.Method != "POST" {
		log.Printf("Notification was not a POST: %v\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
	}
	messageType := r.Header.Get("Twitch-Eventsub-Message-Type")
	if messageType == "webhook_callback_verification" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		var callbackVerification callbackVerification
		err = json.Unmarshal(body, &callbackVerification)

		if err != nil {
			panic(err)
		}
		w.Write([]byte(callbackVerification.Challenge))
	} else if messageType == "stream.online" {
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
		log.Println("Webhook request body: ", twitchNotif)
		go postNotification(twitchNotif)
	}
	return
}

func registerWebhook(client *http.Client, username string) {
	log.Println("Getting user")
	userid := getTwitchUser(username).ID
	conditions := make(map[string]string)
	conditions["broadcaster_user_id"] = userid
	createSubscription := &createSubscription{
		EventType: "stream.online",
		Version:   "1",
		Condition: conditions,
		Transport: transport{
			Method:   "webhook",
			Callback: "https://paintbot.net/notify",
			Secret:   "ThisIsASecret",
		},
	}
	body, _ := json.Marshal(createSubscription)
	log.Printf("Registering createSubscription: %s\n", string(body))

	validateToken()

	req, err := http.NewRequest("POST", "https://api.twitch.tv/helix/eventsub/subscriptions", bytes.NewBuffer(body))
	req.Header.Add("Client-ID", config.Secrets.TwitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)
	req.Header.Add("Content-type", "application/json")

	log.Println("Registering webhook")
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Panic at webhook POST")
		panic(err)
	}
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	log.Printf("Webhook returned: %s\n%s\n", resp.Status, string(body))
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
				var errorCode int = 500

				log.Println(err)
				w.Write([]byte(errorString))
				w.WriteHeader(errorCode)
				return
			}
		})
	}

	var handleFunc = func(path string, handler Handler) {
		http.Handle(path, errorHandling(middleware(handler)))
	}
	handleFunc("/", handleRoot)
	handleFunc("/notify", handleNotification)

	go http.ListenAndServe(":http", certManager.HTTPHandler(nil))

	go log.Fatal(server.ListenAndServeTLS("", "")) //Key and cert are coming from Let's Encrypt
}

func getTwitchUser(userId string) twitchUser {
	var users twitchUserJSON

	log.Println(userId)
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users?login="+userId, nil)
	req.Header.Add("Client-ID", config.Secrets.TwitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)
	req.Header.Add("Content-type", "application/json")

	validateToken()
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(body, &users)

	if err != nil {
		panic(err)
	}

	log.Println(users)
	return users.Users[0]
}

func getTwitchChannel(userId string) twitchChannel {
	var channels twitchChannelJSON

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/channels?broadcaster_id="+userId, nil)
	req.Header.Add("Client-ID", config.Secrets.TwitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)
	req.Header.Add("Content-type", "application/json")

	validateToken()
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(body, &channels)

	if err != nil {
		panic(err)
	}

	log.Println(channels)
	return channels.Channel[0]
}

func getTwitchGame(id string) *twitchGame {
	var g twitchGameJSON

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/games?id="+id, nil)
	req.Header.Add("Client-ID", config.Secrets.TwitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)
	req.Header.Add("Content-type", "application/json")

	validateToken()
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(body, &g)

	if err != nil {
		panic(err)
	}

	log.Println(g)
	return &g.Games[0]
}

func postNotification(twitchNotif notification) {
	log.Println("Posting notification")
	user := getTwitchUser(twitchNotif.Event["broadcaster_user_login"])

	channel := &streamInfo{}
	for _, currChannel := range config.Streams {
		if strings.EqualFold(currChannel.StreamName, twitchNotif.Event["broadcaster_user_name"]) {
			channel = currChannel
			break
		}
	}

	twitchChannel := getTwitchChannel(twitchNotif.Event["broadcaster_user_id"])

	var game *twitchGame
	if len(twitchChannel.GameID) > 0 {
		game = getTwitchGame(twitchChannel.GameID)
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
				URL:     "https://www.twitch.tv/" + twitchNotif.Event["broadcaster_user_name"],
				Name:    twitchNotif.Event["broadcaster_user_name"],
				IconURL: strings.Replace(strings.Replace(user.ProfileImage, "{width}", "70", 1), "{height}", "70", 1),
			},
			Color: int(channel.HighlightColour),
			Fields: []*discordgo.MessageEmbedField{
				// {
				// 	Name:   "Viewers",
				// 	Value:  strconv.Itoa(twitchNotif.ViewerCount),
				// 	Inline: true,
				// },
				{
					Name:   "Game",
					Value:  game.Name,
					Inline: true,
				},
			},
			Image: &discordgo.MessageEmbedImage{
				URL: "https://static-cdn.jtvnw.net/previews-ttv/live_user_" + twitchNotif.Event["broadcaster_user_name"] + "-320x180.png" + "?r=" + time.Now().Format(time.RFC3339),
			},
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: strings.Replace(strings.Replace(game.BoxArt, "{width}", "50", 1), "{height}", "70", 1),
			},
			Title: twitchChannel.Title,
			URL:   "https://www.twitch.tv/" + twitchNotif.Event["broadcaster_user_name"],
		},
	}

	if channel.Description != "" {
		message.Content = channel.Description
	}

	var msg *discordgo.Message
	var err error
	for i, channelID := range channel.Channels {
		if channel.CurrentStreamID == twitchNotif.Event["id"] {
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
