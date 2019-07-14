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
		err = discord.UpdateStatus(0, "A friendly helpful bot!")
		if err != nil {
			fmt.Println("Error attempting to set my status")
		}
		servers := discord.State.Guilds
		fmt.Printf("PaintBot has started on %d servers\n", len(servers))
	})

	err = discord.Open()
	errCheck("Error opening connection to Discord", err)
	defer discord.Close()

	commandPrefix = "+"
	log.Println(info[0].twitchName)

	//<-make(chan struct{})

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

	if message.Content == "+test" {
		content := "This is a test"
		discord.ChannelMessageSend(message.ChannelID, content)
	}

}

func loadConfig() {
	file, err := os.Open("cfg.txt")
	if err != nil {
		log.Fatal(err)
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
