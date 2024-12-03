package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var Logger *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	Logger.Info("ctx context.Context", "detail", ctx)
	Logger.Info("auth config", "detail", config)

	cacheFile, err := getTokenCacheFile()
	if err != nil {
		Logger.Error("cache get error", "detail", err)
	}
	token, err := getTokenFromFile(cacheFile)
	if err != nil {
		authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		token = getTokenFromWeb(config, authURL)
		if token == nil {
			Logger.Error("token is invalid", "", token)
			os.Exit(1)
		}
		saveToken(cacheFile, token)
	}
	return config.Client(ctx, token)
}

func startWebServer() (codeCh chan string, err error) {
	listener, err := net.Listen("tcp", "localhost:8090")
	if err != nil {
		return nil, err
	}
	codeCh = make(chan string)

	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		codeCh <- code
		listener.Close()
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Received code: %v\r\nYou can now safely close this browser window.", code)
	}))
	return codeCh, nil
}

func openURL(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost:4001/").Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("Cannot open URL %s on this platform", url)
	}
	return err
}

func getTokenFromWeb(config *oauth2.Config, authURL string) *oauth2.Token {
	codeCh, err := startWebServer()
	if err != nil {
		Logger.Error("can not start server", "detail", err)
		os.Exit(1)
	}
	err = openURL(authURL)
	if err != nil {
		Logger.Error("cant open browser", "detail", err)
		os.Exit(1)
	}
	code := <-codeCh

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		Logger.Error("can not get authorization code", "detail", err)
		os.Exit(1)
	}
	Logger.Info("getTokenFromWeb", "code", code)
	Logger.Info("getTokenFromWeb", "token", token)

	return token
}

func getTokenFromPrompt(config *oauth2.Config, authURL string) *oauth2.Token {
	var code string
	var token *oauth2.Token
	fmt.Printf("Go to the following link in your browser. After completing the authorization flow, enter the authorization code on the command line: \n%v\n", authURL)

	if _, err := fmt.Scan(&code); err != nil {
		Logger.Error("Can not read authorization code on getTokenFromPrompt", "detail ", err)
	}
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		Logger.Error("can not get Authorization Token", "detail", err)
	}
	Logger.Info("getTokenFromPrompt", "Code", code)
	Logger.Info("getTokenFromPrompt", "Token", token)
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
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
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
	fmt.Printf("ID is %s\nTitle is %s\n", response.Items[0].Id, response.Items[0].Snippet.Title)
}

func main() {
	ctx := context.Background()

	b, err := os.ReadFile("client_credentials.json")
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
