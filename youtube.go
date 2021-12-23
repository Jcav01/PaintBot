package main

import "net/http"

func setupYouTubeNotification(channelID string) {

}

func handleYoutubeNotification(w http.ResponseWriter, r *http.Request) (err error) {
	w.Write([]byte("Hey bishes"))
	return
}
