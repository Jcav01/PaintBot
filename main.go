package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

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
	Callback     string `json:"callback"`
	Mode         string `json:"mode"`
	Topic        string `json:"topic"`
	LeaseSeconds int    `json:"lease_seconds"`
	Secret       string `json:"secret"`
	Challenge    string `json:"challenge"`
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

type twitchGame struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	BoxArt string `json:"box_art_url"`
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
	discord            *discordgo.Session
)

const cfgFile string = "cfg.txt"
const secretsFile string = "secrets.txt"

func main() {
	getSecrets()
	loadConfig()
	generateToken()

	client := &http.Client{}

	go startListen()
	registerWebhook(client)

	log.Println("Starting bot...")
	discord, err := discordgo.New("Bot " + botToken)
	errCheck("error creating discord session", err)
	log.Println("New session created...")
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

func errCheck(msg string, err error) {
	if err != nil {
		fmt.Printf("%s: %+v", msg, err)
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
		content := "https://github.com/jcav2011/PaintBot"
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

	scanner := bufio.NewScanner(file)
	for {
		i := 0
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
		a := announceInfo{
			guildID:    s[0],
			channelID:  s[1],
			twitchName: s[2],
		}
		info[i] = a

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

func validateToken(client *http.Client) {
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
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	log.Printf("Request body received: %s", string(body))
}

func registerWebhook(client *http.Client) {
	userid := getTwitchUser("paintbrushpuke", client).Id
	hub := &hub{
		Callback:     "ec2-3-134-113-251.us-east-2.compute.amazonaws.com:8080/notify",
		Mode:         "subscribe",
		Topic:        "https://api.twitch.tv/helix/streams?user_id=" + userid,
		LeaseSeconds: 864000,
	}
	body, _ := json.Marshal(hub)

	req, err := http.NewRequest("POST", "https://api.twitch.tv/helix/webhooks/hub", bytes.NewBuffer(body))
	req.Header.Add("Client-ID", twitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)

	validateToken(client)
	resp, err := client.Do(req)
	if err != nil {

	}
	defer resp.Body.Close()
}

func startListen() {
	mux := http.NewServeMux()
	mux.HandleFunc("/notify", handleNotification)

	err := http.ListenAndServe(":8080", mux)
	log.Fatal(err)
}

func getTwitchUser(username string, client *http.Client) twitchUser {
	var u twitchUser

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users?login="+username, nil)
	req.Header.Add("Client-ID", twitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)

	validateToken(client)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(body, &u)

	if err != nil {
		panic(err)
	}

	return u
}

func getGame(id string, client *http.Client) twitchGame {
	var g twitchGame

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/games?id="+id, nil)
	req.Header.Add("Client-ID", twitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)

	validateToken(client)
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

	return g
}
