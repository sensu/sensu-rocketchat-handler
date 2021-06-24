package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/templates"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
)

type LoginResponse struct {
	Status  string
	Error   string
	Message string
	Data    struct {
		AuthToken string
		UserId    string
		Me        struct {
			Username string
			Roles    []string
		}
	}
	Response string
}
type InfoResponse struct {
	User struct {
		Type  string
		Roles []string
	}
	Success bool
}

type PostMessageResponse struct {
	Success bool
	Error   string
}

type LogoutResponse struct {
	Status string
}

type MsgAttachment struct {
	Color       string            `json:"color,omitempty"`
	Text        string            `json:"text,omitempty"`
	Ts          string            `json:"ts,omitempty"`
	ThumbUrl    string            `json:"thumb_url,omitempty"`
	MessageLink string            `json:"message_link,omitempty"`
	AuthorName  string            `json:"author_name,omitempty"`
	AuthorLink  string            `json:"author_link,omitempty"`
	AuthorIcon  string            `json:"author_icon,omitempty"`
	Title       string            `json:"title,omitempty"`
	ImageUrl    string            `json:"image_url,omitempty"`
	Fields      []AttachmentField `json:"fields,omitempty"`
}

type AttachmentField struct {
	Short bool   `json:"short,omitempty"`
	Title string `json:"title"`
	Value string `json:"value"`
}

type RocketMsg struct {
	Channel     string          `json:"channel"`
	Text        string          `json:"text,omitempty"`
	Alias       string          `json:"alias,omitempty"`
	Emoji       string          `json:"emoji,omitempty"`
	Avatar      string          `json:"avatar,omitempty"`
	Attachments []MsgAttachment `json:"attachments,omitempty"`
}

// Config represents the handler plugin config.
type Config struct {
	sensu.PluginConfig
	Verbose             bool
	DryRun              bool
	Url                 string
	User                string
	Password            string
	Token               string
	UserID              string
	Channel             string
	DescriptionTemplate string
	Alias               string
	Avatar              string
}

const (
	defaultTemplate = "{{ .Check.Output }}"
)

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-rocketchat-handler",
			Short:    "Sensu Handler to send messages to rocketchat chat service",
			Keyspace: "sensu.io/plugins/rocketchat/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		&sensu.PluginConfigOption{
			Path:      "dry-run",
			Argument:  "dry-run",
			Shorthand: "n",
			Default:   false,
			Usage:     "Used for testing, do not communicate with RocketChat API, report only (implies --verbose)",
			Value:     &plugin.DryRun,
		},
		&sensu.PluginConfigOption{
			Path:      "verbose",
			Argument:  "verbose",
			Shorthand: "v",
			Default:   false,
			Usage:     "Verbose output",
			Value:     &plugin.Verbose,
		},
		&sensu.PluginConfigOption{
			Path:      "url",
			Env:       "ROCKETCHAT_URL",
			Argument:  "url",
			Shorthand: "u",
			Default:   "http://localhost:3000",
			Usage:     "RocketChat service URL",
			Value:     &plugin.Url,
		},
		&sensu.PluginConfigOption{
			Path:      "channel",
			Env:       "ROCKETCHAT_CHANNEL",
			Argument:  "channel",
			Shorthand: "c",
			Default:   "",
			Usage:     "RocketChat channel to send messages to. (Required)",
			Value:     &plugin.Channel,
		},
		&sensu.PluginConfigOption{
			Path:      "description-template",
			Env:       "ROCKETCHAT_DESCRIPTION_TEMPLATE",
			Argument:  "description-template",
			Shorthand: "t",
			Default:   defaultTemplate,
			Usage:     "The RocketChat notification output template, in Golang text/template format",
			Value:     &plugin.DescriptionTemplate,
		},

		&sensu.PluginConfigOption{
			Path:      "user",
			Env:       "ROCKETCHAT_USER",
			Argument:  "user",
			Shorthand: "U",
			Secret:    true,
			Default:   "",
			Usage:     "RocketChat User. Used with --password. Note for security using ROCKETCHAT_USER environment variable is preferred",
			Value:     &plugin.User,
		},
		&sensu.PluginConfigOption{
			Path:      "password",
			Env:       "ROCKETCHAT_PASSWORD",
			Argument:  "password",
			Shorthand: "P",
			Secret:    true,
			Default:   "",
			Usage:     "RocketChat User Password. Used with --user. Note for security using ROCKETCHAT_PASSWORD environment variable is preferred",
			Value:     &plugin.Password,
		},
		&sensu.PluginConfigOption{
			Path:      "token",
			Env:       "ROCKETCHAT_TOKEN",
			Argument:  "token",
			Shorthand: "T",
			Secret:    true,
			Default:   "",
			Usage:     "RocketChat Auth Token. Used with --userID Note for security using ROCKETCHAT_TOKEN environment variable is preferred",
			Value:     &plugin.Token,
		},
		&sensu.PluginConfigOption{
			Path:      "userID",
			Env:       "ROCKETCHAT_USERID",
			Argument:  "userID",
			Shorthand: "I",
			Secret:    true,
			Default:   "",
			Usage:     "RocketChat Auth UserID. Used with --token. Note for security using ROCKETCHAT_USERID environment variable is preferred",
			Value:     &plugin.UserID,
		},
		&sensu.PluginConfigOption{
			Path:     "alias",
			Env:      "ROCKETCHAT_ALIAS",
			Argument: "alias",
			Default:  "sensu",
			Usage:    "Name to use in the RocketChat msg. (Note: user must have bot role to take effect)",
			Value:    &plugin.Alias,
		},
		&sensu.PluginConfigOption{
			Path:     "avatar-url",
			Env:      "ROCKETCHAT_AVATAR_URL",
			Argument: "avatar-url",
			Default:  "https://www.sensu.io/img/sensu-logo.png",
			Usage:    "Avatar image url to use in RocketChat msg. (Note: user must have bot role to take effect)",
			Value:    &plugin.Avatar,
		},
	}
)

func main() {
	handler := sensu.NewGoHandler(&plugin.PluginConfig, options, checkArgs, executeHandler)
	handler.Execute()

}

func checkArgs(_ *types.Event) error {
	if len(plugin.Channel) == 0 {
		return fmt.Errorf("channel is required")
	}
	if len(plugin.Token) > 0 || len(plugin.UserID) > 0 {
		if len(plugin.User) > 0 {
			return fmt.Errorf("--user conflicts with --token. Please use either user/password or token based auth")
		}
		if len(plugin.Password) > 0 {
			return fmt.Errorf("--password conflicts with --token. Please use either user/password or token based auth")
		}
		if len(plugin.Token) == 0 {
			return fmt.Errorf("token is required with userID. Security Note: use ROCKETCHAT_TOKEN environment variable instead of --token in production")
		}
		if len(plugin.UserID) == 0 {
			return fmt.Errorf("userID is required with token. Security Note: use ROCKETCHAT_USERID environment variable instead of --userID in production")
		}
	} else {
		if len(plugin.User) == 0 {
			return fmt.Errorf("user is required. Security Note: use ROCKETCHAT_USER environment variable instead of --user in production")
		}
		if len(plugin.Password) == 0 {
			return fmt.Errorf("password is required. Security Note: use ROCKETCHAT_PASSWORD environment variable instead of --password in production")
		}
	}
	if plugin.DryRun {
		plugin.Verbose = true
	}
	return nil
}

func executeHandler(event *types.Event) error {

	// Login if not using token auth
	loginData := LoginResponse{}
	if len(plugin.Token) != 0 {
		loginData.Status = "success"
	} else {
		loginData = login()
	}

	if loginData.Status == "success" {
		message, err := buildMsg(event)
		//Construct Msg from Event
		if err != nil {
			fmt.Printf("%s: Error processing template: %s", plugin.PluginConfig.Name, err)
		}
		if plugin.DryRun {
			log.Printf("DryRun Report:\n Channel: %v Server: %v\n Msg: %v",
				plugin.Channel, plugin.Url, message)
		}
		postMessageData := postMessage(message)
		if postMessageData.Success {
			if plugin.Verbose {
				log.Println("RocketChat Message sent to channel: " + plugin.Channel + " <" + plugin.Url + "> message: " + message + " [ok]")
			}
		} else {
			log.Println("RocketChat Message sent to channel: " + plugin.Channel + " <" + plugin.Url + "> message: " + message + " [error]")
			log.Fatal("Error posting message to RocketChat:", postMessageData.Error)
		}
		if len(plugin.User) > 0 {
			logout := logout()
			if logout.Status == "success" {
				if plugin.Verbose {
					log.Println("RocketChat log out [ok]")
				}
			} else {
				log.Println("RocketChat log out [failed]")
			}
		} else {
			if plugin.Verbose {
				log.Println("RocketChat log out [skipped]")
			}
		}
	} else {
		log.Println("RocketChat Login Failed with msg: " + loginData.Message)
		log.Fatal(loginData.Response)
	}
	return nil
}

func buildMsg(event *corev2.Event) (string, error) {
	msg := RocketMsg{}
	msg.Channel = plugin.Channel
	if isBot() {
		msg.Alias = plugin.Alias
		msg.Avatar = plugin.Avatar
	}
	msg.Attachments = []MsgAttachment{
		messageAttachment(event),
	}
	m, err := json.Marshal(msg)
	return string(m), err
}

func isBot() bool {

	u, err := url.Parse(plugin.Url)
	u.Path = path.Join(u.Path, "/api/v1/users.info")
	log.Printf("User Info for User: %v", plugin.User)
	req, err := http.NewRequest("GET", u.String()+"?username="+plugin.User, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-Auth-Token", plugin.Token)
	req.Header.Set("X-User-Id", plugin.UserID)
	req.Header.Set("Content-Type", "application/json")

	infoResponse := InfoResponse{}
	if plugin.DryRun {
		infoResponse.Success = true
		infoResponse.User.Type = "dry-run"
	} else {

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(b, &infoResponse)
		if plugin.Verbose {
			log.Printf("User %s has bot role: %v\n", plugin.User, contains(infoResponse.User.Roles, "bot"))
		}
	}
	if infoResponse.Success {
		return contains(infoResponse.User.Roles, "bot")
	} else {
		return false
	}

}
func messageAttachment(event *corev2.Event) MsgAttachment {
	description, err := templates.EvalTemplate("description", plugin.DescriptionTemplate, event)
	if err != nil {
		log.Printf("%s: Error processing template: %s", plugin.PluginConfig.Name, err)
	}
	description = strings.Replace(description, `\n`, "\n", -1)

	a := MsgAttachment{
		Title: "Description",
		Color: messageColor(event),
	}
	fields := []AttachmentField{
		{
			Title: "Status",
			Value: messageStatus(event),
			Short: false,
		},
		{
			Title: "Entity",
			Value: event.Entity.Name,
			Short: true,
		},
		{
			Title: "Check",
			Value: event.Check.Name,
			Short: true,
		},
	}
	a.Fields = fields
	return a
}

func messageStatus(event *corev2.Event) string {
	switch event.Check.Status {
	case 0:
		return "Resolved"
	case 2:
		return "Critical"
	default:
		return "Warning"
	}
}

func messageColor(event *corev2.Event) string {
	switch event.Check.Status {
	case 0:
		return "green"
	case 2:
		return "red"
	default:
		return "orange"
	}
}

func login() LoginResponse {
	u, err := url.Parse(plugin.Url)
	u.Path = path.Join(u.Path, "/api/v1/login")

	body := strings.NewReader(`user=` + plugin.User + `&password=` + plugin.Password)
	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResponse := LoginResponse{}
	if plugin.DryRun {
		loginResponse.Status = "success"
		loginResponse.Response = "Dry Run: No RocketChat login request made"
		loginResponse.Data.AuthToken = "dryrun"
		loginResponse.Data.UserId = "dryrun"
		loginResponse.Data.Me.Username = "dryrun"
	} else {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(b, &loginResponse)
		loginResponse.Response = string(b)
		if plugin.Verbose {
			log.Printf("Login Data: %+v", loginResponse.Data.Me)
		}
	}
	plugin.Token = loginResponse.Data.AuthToken
	plugin.UserID = loginResponse.Data.UserId
	plugin.User = loginResponse.Data.Me.Username
	return loginResponse
}

func postMessage(message string) PostMessageResponse {
	u, err := url.Parse(plugin.Url)
	u.Path = path.Join(u.Path, "/api/v1/chat.postMessage")
	body := strings.NewReader(message)
	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-Auth-Token", plugin.Token)
	req.Header.Set("X-User-Id", plugin.UserID)
	req.Header.Set("Content-Type", "application/json")
	postMessageResponse := PostMessageResponse{}
	if plugin.DryRun {
		postMessageResponse.Success = true
	} else {

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(b, &postMessageResponse)
	}
	return postMessageResponse
}

func logout() LogoutResponse {
	u, err := url.Parse(plugin.Url)
	u.Path = path.Join(u.Path, "/api/v1/logout")
	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-Auth-Token", plugin.Token)
	req.Header.Set("X-User-Id", plugin.UserID)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	logoutResponse := LogoutResponse{}
	if plugin.DryRun {
		logoutResponse.Status = "success"
	} else {

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		b, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(b, &logoutResponse)
	}
	return logoutResponse
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
