package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
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
)

const cfgFile string = "cfg.txt"

//https://twitter.com/search?q=from%3Apaintbrushpuke%20url%3Atwitch.tv%2Fpaintbrushpuke&src=typd
func main() {
	loadConfig()
	discord, err := discordgo.New("Bot NTk4MzE4OTgzNDg1MzI1MzQy.XSU6iQ.mwhNPfPAK7bZWaOwyX_NXpRyEMc")
	errCheck("error creating discord session", err)
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

	<-make(chan struct{})

}

func errCheck(msg string, err error) {
	if err != nil {
		fmt.Printf("%s: %+v", msg, err)
		panic(err)
	}
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
