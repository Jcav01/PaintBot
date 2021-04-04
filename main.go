package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/twitch"
)

type hub struct {
	Callback     string `json:"hub.callback"`
	Mode         string `json:"hub.mode"`
	Topic        string `json:"hub.topic"`
	LeaseSeconds int    `json:"hub.lease_seconds"`
	Secret       string `json:"hub.secret"`
	Challenge    string `json:"hub.challenge"`
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

type twitchStream struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	GameID      string `json:"game_id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	ViewerCount int    `json:"viewer_count"`
	StartedAt   string `json:"started_at"`
	Language    string `json:"language"`
	Thumbnail   string `json:"thumbnail_url"`
}
type twitchStreamJSON struct {
	Streams []twitchStream `json:"data"`
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

type channelInfo struct {
	ChannelName     string           `json:"channel_name"`
	Channels        []discordChannel `json:"discord_channel_ids"`
	ColourString    string           `json:"colour"`
	HighlightColour int64            `json:"highlight_colour"`
	LastLive        string           `json:"last_live"`
}

type secrets struct {
	BotToken           string `json:"bot_token"`
	TwitchClientID     string `json:"twitch_client_id"`
	TwitchClientSecret string `json:"twitch_client_secret"`
	CallbackURL        string `json:"callback_url"`
}

type cofiguration struct {
	Secrets  secrets        `json:"secrets"`
	Channels []*channelInfo `json:"channels"`
}

var (
	botID        string
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
	generateToken()
	client = &http.Client{}

	startListen()
	var wg sync.WaitGroup
	for _, channel := range config.Channels {
		wg.Add(1)
		go func(username string) {
			defer wg.Done()
			registerWebhook(client, username, "unsubscribe")
			registerWebhook(client, username, "subscribe")
		}(channel.ChannelName)
	}
	wg.Wait()

	discord := createDiscordSession()
	user, err := discord.User("@me")
	errCheck("error retrieving account", err)

	botID = user.ID
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

	for _, channel := range config.Channels {
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
		return
	}
	generateToken()
}

func handleNotification(w http.ResponseWriter, r *http.Request) {
	var streams twitchStreamJSON
	log.Printf("Handling notification: %v\n", r.URL)
	challenge := r.URL.Query().Get("hub.challenge")
	log.Printf("Challenge is: %v\n", challenge)

	if challenge != "" {
		w.Write([]byte(challenge))
	} else {
		w.WriteHeader(http.StatusNoContent)
		log.Printf("Responded to webhook\n")
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
		}

		err = json.Unmarshal(body, &streams)

		if err != nil {
			log.Println(err)
		}
		log.Println("Webhook request body: ", streams)
		if len(streams.Streams) > 0 {
			go postNotification(streams.Streams[0])
		}
	}
}

func registerWebhook(client *http.Client, username string, subAction string) {
	userid := getTwitchUser(username).ID
	hub := &hub{
		Callback:     config.Secrets.CallbackURL + "6969/notify",
		Mode:         subAction,
		Topic:        "https://api.twitch.tv/helix/streams?user_id=" + userid,
		LeaseSeconds: 864000,
	}
	body, _ := json.Marshal(hub)
	//log.Printf("Registering hub: %s", string(body))

	req, err := http.NewRequest("POST", "https://api.twitch.tv/helix/webhooks/hub", bytes.NewBuffer(body))
	req.Header.Add("Client-ID", config.Secrets.TwitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)
	req.Header.Add("Content-type", "application/json")

	validateToken()
	log.Println("Registering webhook")
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Panic at webhook POST")
		panic(err)
	}
	defer resp.Body.Close()
	log.Printf("Webhook returned: %s\n", resp.Status)
	if subAction == "subscribe" {
		go renewWebhook(client, username, subAction)
	}
}

func renewWebhook(client *http.Client, username string, subAction string) {
	time.Sleep(239 * time.Hour)
	registerWebhook(client, username, subAction)
}

func startListen() {
	mux := http.NewServeMux()
	mux.HandleFunc("/notify", handleNotification)

	log.Println("Listening on: :6969")
	go http.ListenAndServe(":6969", mux)
}

func getTwitchUser(username string) twitchUser {
	var users twitchUserJSON

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users?login="+username, nil)
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

func postNotification(stream twitchStream) {
	log.Println("Posting notification")
	user := getTwitchUser(stream.UserName)

	var game *twitchGame
	if len(stream.GameID) > 0 {
		game = getTwitchGame(stream.GameID)
	}
	if game == nil {
		game = &twitchGame{
			Name:   "N/A",
			BoxArt: "https://images.igdb.com/igdb/image/upload/t_cover_big/nocover_qhhlj6.jpg",
		}
	}

	channel := &channelInfo{}
	for _, currChannel := range config.Channels {
		if strings.ToLower(currChannel.ChannelName) == strings.ToLower(stream.UserName) {
			channel = currChannel
			break
		}
	}

	discord := createDiscordSession()
	defer discord.Close()
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			URL:     "https://www.twitch.tv/" + stream.UserName,
			Name:    stream.UserName,
			IconURL: strings.Replace(strings.Replace(user.ProfileImage, "{width}", "70", 1), "{height}", "70", 1),
		},
		Color: int(channel.HighlightColour),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Viewers",
				Value:  strconv.Itoa(stream.ViewerCount),
				Inline: true,
			},
			{
				Name:   "Game",
				Value:  game.Name,
				Inline: true,
			},
		},
		Image: &discordgo.MessageEmbedImage{
			URL: strings.Replace(strings.Replace(stream.Thumbnail+"?r="+time.Now().Format(time.RFC3339), "{width}", "320", 1), "{height}", "180", 1),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: strings.Replace(strings.Replace(game.BoxArt, "{width}", "50", 1), "{height}", "70", 1),
		},
		Title: stream.Title,
		URL:   "https://www.twitch.tv/" + stream.UserName,
	}

	lastNotify, errr := time.Parse(time.RFC3339, channel.LastLive)
	if errr != nil {
		log.Printf("Could not parse %v", channel.LastLive)
	}
	newNotify, errr := time.Parse(time.RFC3339, stream.StartedAt)
	if errr != nil {
		log.Printf("Could not parse %v", stream.StartedAt)
	}
	log.Printf("lastNotify: %v, newNotify: %v", lastNotify, newNotify)
	var msg *discordgo.Message
	var err error
	for _, channelID := range channel.Channels {
		if lastNotify.Equal(newNotify) {
			msg, err = discord.ChannelMessageEditEmbed(channelID.ChannelID, channelID.MessageID, embed)
		} else {
			msg, err = discord.ChannelMessageSendEmbed(channelID.ChannelID, embed)
		}

		if err != nil {
			log.Printf("%v did not send: %v\n", msg, err)
		} else {
			channel.LastLive = stream.StartedAt
			channelID.MessageID = msg.ID
		}
	}
}
