package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var Logger *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := getTokenCacheFile()
	if err != nil {
		Logger.Error("cache get error", "detail", err)
	}
	token, err := getTokenFromFile(cacheFile)
	if err != nil {
		token = getTokenFromWeb(config)
		saveToken(cacheFile, token)
	}
	return config.Client(ctx, token)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("authorization code: %v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		Logger.Error("can not read authorization code", "detail", err)
	}

	token, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		Logger.Error("can not get authorization code", "detail", err)
	}

	return token
}

func getTokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".youtube")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir, url.QueryEscape("youtube-go.json")), err
}

func getTokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Save credential file to %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		Logger.Error("can not cache oauth token", "detail", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func handleError(err error, message string) {
	if message == "" {
		message = "Error making API Call"
	}
	if err != nil {
		Logger.Error(message, "Detail", err)
	}
}

func channelsListByUsername(service *youtube.Service, part []string, forUserName string) {
	call := service.Channels.List(part)
	call = call.ForUsername(forUserName)
	response, err := call.Do()
	handleError(err, "")
	fmt.Printf("ID is %s\nTitle is %s\nVideo Count is %d\n", response.Items[0].Id, response.Items[0].Snippet.Title, response.Items[0].Statistics.ViewCount)
}

func main() {
	ctx := context.Background()

	b, err := os.ReadFile("client_secret.json")
	if err != nil {
		Logger.Error("can not read client secret file", "Detail", err)
	}

	config, err := google.ConfigFromJSON(b, youtube.YoutubeReadonlyScope)
	if err != nil {
		Logger.Error("can not parse Client secret file to config", "Detail", err)
	}
	client := getClient(ctx, config)
	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))

	handleError(err, "cant Creating Youtube Client")

	channelsListByUsername(service, []string{"snippet", "contentDetails"}, "GoogleDevelopers")
}
