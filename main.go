package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dghubble/go-twitter/twitter"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type announceInfo struct {
	guildID     string
	channelID   string
	twitterName string
	twitchName  string
}

var (
	commandPrefix string
	botID         string
	info          [5]announceInfo
	botToken      string
	twitterKey    string
	twitterSecret string
	twitterClient *twitter.Client
)

const cfgFile string = "cfg.txt"
const secretsFile string = "secrets.txt"

//https://twitter.com/search?q=from%3Apaintbrushpuke%20url%3Atwitch.tv%2Fpaintbrushpuke&src=typd
func main() {
	getSecrets()
	loadConfig()
	setupTwitter()

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
		fmt.Printf("PaintBot has started on %d servers\n", len(servers))
	})

	err = discord.Open()
	errCheck("Error opening connection to Discord", err)
	defer discord.Close()

	commandPrefix = "+"

	go searchForTweets(discord)

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
	twitterKey = s[1]
	twitterSecret = s[2]
}

func commandHandler(discord *discordgo.Session, message *discordgo.MessageCreate) {
	user := message.Author
	if user.ID == botID || user.Bot {
		//Do nothing because the bot is talking
		return
	}

	m := strings.Split(message.Content, " ")

	if m[0] == "+test" {
		content := "<@!" + message.Author.ID + ">"
		discord.ChannelMessageSend(message.ChannelID, content)
	}

	if m[0] == "+config" {
		a := announceInfo{
			guildID:     message.GuildID,
			channelID:   message.ChannelID,
			twitterName: m[1],
			twitchName:  m[2],
		}
		writeConfig(a)
		content := "Wrote info to config"
		discord.ChannelMessageSend(message.ChannelID, content)
		discord.ChannelMessageDelete(message.ChannelID, message.ID)
	}
}

func writeConfig(cfg announceInfo) {
	var file *os.File
	fileInfo, err := os.Stat(cfgFile)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(cfgFile)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Created new cfg file. Info:")
			log.Println(fileInfo)
		}
	} else {
		file, err = os.Open(cfgFile)
		if err != nil {
			log.Fatal(err)
		}
	}
	defer file.Close()

	bytesWritten, err := file.WriteString(cfg.guildID + " " + cfg.channelID + " " + cfg.twitterName + " " + cfg.twitchName + "\n")
	log.Printf("Wrote %d bytes to config\n", bytesWritten)

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
			guildID:     s[0],
			channelID:   s[1],
			twitterName: s[2],
			twitchName:  s[3],
		}
		info[i] = a

	}
}

func setupTwitter() {
	config := &clientcredentials.Config{
		ClientID:     twitterKey,
		ClientSecret: twitterSecret,
		TokenURL:     "https://api.twitter.com/oauth2/token",
	}
	// http.Client will automatically authorize Requests
	httpClient := config.Client(oauth2.NoContext)

	// Twitter client
	twitterClient = twitter.NewClient(httpClient)
}

func searchForTweets(discord *discordgo.Session) {
	for {
		q := "from:" + info[0].twitterName + " url:twitch.tv/" + info[0].twitchName
		fmt.Println(q)
		search, resp, err := twitterClient.Search.Tweets(&twitter.SearchTweetParams{
			Query: q,
		})
		if err != nil {
			log.Println(err)
		} else if resp.StatusCode == 200 {
			discord.ChannelMessageSend(info[0].channelID, search.Statuses[0].Text)
		}
		time.Sleep(5 * time.Minute)
	}
}
