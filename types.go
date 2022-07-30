package main

import "net/http"

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

type twitchSubscription struct {
	Total        int                `json:"total"`
	Data         []subscriptionInfo `json:"data"`
	TotalCost    int                `json:"total_cost"`
	MaxTotalCost int                `json:"max_total_cost"`
}

type discordChannel struct {
	ChannelID string `json:"id"`
	MessageID string `json:"message_id"`
}

type streamInfo struct {
	StreamName      string           `json:"stream_name"`
	UserId          string           `json:"twitch_user_id"`
	Channels        []discordChannel `json:"discord_channel_ids"`
	ColourString    string           `json:"colour"`
	HighlightColour int64            `json:"highlight_colour"`
	CurrentStreamID string           `json:"current_stream"`
	Description     string           `json:"description"`
	IsLive          bool             `json:"is_live"`
	Category        string           `json:"category"`
	Title           string           `json:"title"`
	OfflineTime     int64            `json:"offline_time"`
	LastOffline     int64            `json:"last_offline"`
	Type            int              `json:"type"`
	VideoIds        []string         `json:"video_ids"`
	DisableOffline  bool             `json:"disable_offline"`
}

type secrets struct {
	BotToken           string `json:"bot_token"`
	TwitchClientID     string `json:"twitch_client_id"`
	TwitchClientSecret string `json:"twitch_client_secret"`
	BaseUrl            string `json:"url"`
}

type cofiguration struct {
	Secrets secrets       `json:"secrets"`
	Streams []*streamInfo `json:"streams"`
}

type hub struct {
	Mode         string `json:"hub.mode"`
	Topic        string `json:"hub.topic"`
	Callback     string `json:"hub.callback"`
	LeaseSeconds int    `json:"hub.lease_seconds"`
}

type Handler func(http.ResponseWriter, *http.Request) error

const (
	twitchType  = 1
	youtubeType = 2
)
