package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/twitch"
)

type announceInfo struct {
	guildID    string
	channelID  string
	twitchName string
}

type hub struct {
	Callback     string `json:"hub.callback"`
	Mode         string `json:"hub.mode"`
	Topic        string `json:"hub.topic"`
	LeaseSeconds int    `json:"hub.lease_seconds"`
	Secret       string `json:"hub.secret"`
	Challenge    string `json:"hub.challenge"`
}

type twitchUser struct {
	Id              string `json:"id"`
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
type twitchUserJson struct {
	Users []twitchUser `json:"data"`
}

type twitchStream struct {
	Id          string `json:"id"`
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
type twitchStreamJson struct {
	Streams []twitchStream `json:"data"`
}

type twitchGame struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	BoxArt string `json:"box_art_url"`
}
type twitchGameJson struct {
	Games []twitchGame `json:"data"`
}

type embedInfo struct {
	ChannelID string
	LastLive  string
	MessageID string
	Colour    int
}

var (
	commandPrefix      string
	botID              string
	info               [5]announceInfo
	botToken           string
	twitchClientID     string
	twitchClientSecret string
	twitchToken        *oauth2.Token
	oauth2Config       *clientcredentials.Config
	client             *http.Client
	channelMap         map[string]*embedInfo
)

const cfgFile string = "cfg.txt"
const countFile string = "count.txt"
const secretsFile string = "secrets.txt"

func main() {
	getSecrets()
	loadConfig()
	generateToken()
	client = &http.Client{}

	startListen()
	registerWebhook(client, "paintbrushpuke", "unsubscribe")
	registerWebhook(client, "paintbrushpuke", "subscribe")

	discord := createDiscordSession()
	user, err := discord.User("@me")
	errCheck("error retrieving account", err)

	botID = user.ID
	discord.AddHandler(commandHandler)
	discord.AddHandler(func(discord *discordgo.Session, ready *discordgo.Ready) {
		servers := discord.State.Guilds
		log.Printf("PaintBot has started on %d servers\n", len(servers))
	})

	err = discord.Open()
	errCheck("Error opening connection to Discord", err)
	defer discord.Close()

	commandPrefix = "+"

	<-make(chan struct{})

}

func createDiscordSession() *discordgo.Session {
	log.Println("Starting bot...")
	discord, err := discordgo.New("Bot " + botToken)
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

func getSecrets() {
	data, err := ioutil.ReadFile(secretsFile)
	if err != nil {
		log.Fatal(err)
	}

	s := strings.Split(string(data), "\r\n")
	botToken = s[0]
	twitchClientID = s[1]
	twitchClientSecret = s[2]
}

func commandHandler(discord *discordgo.Session, message *discordgo.MessageCreate) {
	user := message.Author
	if user.ID == botID || user.Bot {
		//Do nothing because the bot is talking
		return
	}

	m := strings.Split(message.Content, " ")

	if m[0] == "+repo" {
		content := "https://github.com/jcav01/PaintBot"
		discord.ChannelMessageSend(message.ChannelID, content)
	}

	if m[0] == "+invite" {
		content := "https://discordapp.com/api/oauth2/authorize?client_id=598318983485325342&permissions=11264&scope=bot"
		discord.ChannelMessageSend(message.ChannelID, content)
	}

	if m[0] == "+loadConfig" {
		loadConfig()
		discord.ChannelMessageDelete(message.ChannelID, message.ID)
	}

	if m[0] == "+config" {
		a := announceInfo{
			guildID:    message.GuildID,
			channelID:  message.ChannelID,
			twitchName: m[1],
		}
		writeConfig(a)
		loadConfig()
		content := "Wrote info to config"
		discord.ChannelMessageSend(message.ChannelID, content)
		discord.ChannelMessageDelete(message.ChannelID, message.ID)
	}
}

func writeConfig(cfg announceInfo) {
	file, err := os.OpenFile(cfgFile, os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	bytesWritten, err := file.WriteString(cfg.guildID + " " + cfg.channelID + " " + cfg.twitchName + "\n")
	if err != nil {
		log.Println(err)
	}
	log.Println("Wrote " + string(bytesWritten) + " bytes to config ")

	file.Sync()
}

func loadConfig() {
	file, err := os.Open(cfgFile)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	channelMap = make(map[string]*embedInfo)
	scanner := bufio.NewScanner(file)
	for {
		success := scanner.Scan()
		if success == false {
			// False on error or EOF. Check error
			err = scanner.Err()
			if err == nil {
				log.Println("Scan completed and reached EOF")
				return
			} else {
				log.Fatal(err)
				return
			}
		}

		// Get data from scan with Bytes() or Text()
		//fmt.Println("First word found:", scanner.Text())
		s := strings.Split(scanner.Text(), " ")
		colour, err := strconv.Atoi(s[2])
		if err != nil {
			log.Fatal(err)
			return
		}
		channelMap[s[0]] = &embedInfo{
			ChannelID: s[1],
			LastLive:  "",
			Colour:    colour,
		}

	}
}

func generateToken() {
	oauth2Config = &clientcredentials.Config{
		ClientID:     twitchClientID,
		ClientSecret: twitchClientSecret,
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
	req.Header.Add("Client-ID", twitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return
	} else {
		generateToken()
	}
}

func handleNotification(w http.ResponseWriter, r *http.Request) {
	var streams twitchStreamJson
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
	userid := getTwitchUser(username).Id
	hub := &hub{
		Callback:     "http://ec2-3-134-113-251.us-east-2.compute.amazonaws.com/notify",
		Mode:         subAction,
		Topic:        "https://api.twitch.tv/helix/streams?user_id=" + userid,
		LeaseSeconds: 864000,
	}
	body, _ := json.Marshal(hub)
	//log.Printf("Registering hub: %s", string(body))

	req, err := http.NewRequest("POST", "https://api.twitch.tv/helix/webhooks/hub", bytes.NewBuffer(body))
	req.Header.Add("Client-ID", twitchClientID)
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

	log.Println("Listening on: :80")
	go http.ListenAndServe(":80", mux)
}

func getTwitchUser(username string) twitchUser {
	var users twitchUserJson

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users?login="+username, nil)
	req.Header.Add("Client-ID", twitchClientID)
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

func getTwitchGame(id string) twitchGame {
	var g twitchGameJson

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/games?id="+id, nil)
	req.Header.Add("Client-ID", twitchClientID)
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
	return g.Games[0]
}

func postNotification(stream twitchStream) {
	log.Println("Posting notification")
	user := getTwitchUser(stream.UserName)
	game := getTwitchGame(stream.GameID)

	discord := createDiscordSession()
	defer discord.Close()
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			URL:     "https://www.twitch.tv/" + stream.UserName,
			Name:    stream.UserName,
			IconURL: strings.Replace(strings.Replace(user.ProfileImage, "{width}", "70", 1), "{height}", "70", 1),
		},
		Color: channelMap[strings.ToLower(stream.UserName)].Colour,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:   "Viewers",
				Value:  strconv.Itoa(stream.ViewerCount),
				Inline: true,
			},
			&discordgo.MessageEmbedField{
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
	lastNotify, errr := time.Parse(time.RFC3339, channelMap[strings.ToLower(stream.UserName)].LastLive)
	if errr != nil {
		log.Printf("Could not parse %v", channelMap[strings.ToLower(stream.UserName)].LastLive)
	}
	newNotify, errr := time.Parse(time.RFC3339, stream.StartedAt)
	if errr != nil {
		log.Printf("Could not parse %v", stream.StartedAt)
	}
	log.Printf("lastNotify: %v, newNotify: %v", lastNotify, newNotify)
	var msg *discordgo.Message
	var err error
	if lastNotify.Equal(newNotify) {
		msg, err = discord.ChannelMessageEditEmbed(channelMap[strings.ToLower(stream.UserName)].ChannelID, channelMap[strings.ToLower(stream.UserName)].MessageID, embed)
	} else {
		msg, err = discord.ChannelMessageSendEmbed(channelMap[strings.ToLower(stream.UserName)].ChannelID, embed)
	}

	if err != nil {
		log.Printf("%v did not send: %v\n", msg, err)
	} else {
		channelMap[strings.ToLower(stream.UserName)].LastLive = stream.StartedAt
		channelMap[strings.ToLower(stream.UserName)].MessageID = msg.ID
	}
}
