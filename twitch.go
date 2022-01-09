package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func getTwitchUser(userId string) twitchUser {
	var users twitchUserJSON

	log.Println(userId)
	req, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/users?login="+userId, nil)
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

	req, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/channels?broadcaster_id="+userId, nil)
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

	req, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/games?id="+id, nil)
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

func getSubscriptions(status string) twitchSubscription {
	var s twitchSubscription

	req, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/eventsub/subscriptions?status="+status, nil)
	req.Header.Add("Client-ID", config.Secrets.TwitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)

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

	err = json.Unmarshal(body, &s)

	if err != nil {
		panic(err)
	}

	return s
}

func deleteSubscription(subID string) {
	req, _ := http.NewRequest("DELETE", "https://api.twitch.tv/helix/eventsub/subscriptions?id="+subID, nil)
	req.Header.Add("Client-ID", config.Secrets.TwitchClientID)
	req.Header.Add("Authorization", "Bearer "+twitchToken.AccessToken)

	validateToken()
	_, err := client.Do(req)
	if err != nil {
		panic(err)
	}

}

func registerTwitchWebhook(client *http.Client, userId string, eventType string) {
	conditions := make(map[string]string)
	conditions["broadcaster_user_id"] = userId
	createSubscription := &createSubscription{
		EventType: eventType,
		Version:   "1",
		Condition: conditions,
		Transport: transport{
			Method:   "webhook",
			Callback: "https://paintbot.net/notify",
			Secret:   "ThisIsASecret",
		},
	}
	body, _ := json.Marshal(createSubscription)
	//log.Printf("Registering createSubscription: %s\n", string(body))

	validateToken()

	req, _ := http.NewRequest("POST", "https://api.twitch.tv/helix/eventsub/subscriptions", bytes.NewBuffer(body))
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
	// body, _ = ioutil.ReadAll(resp.Body)
	log.Printf("Webhook returned: %s\n", resp.Status)
}
