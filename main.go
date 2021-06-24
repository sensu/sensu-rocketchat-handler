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
)

type LoginResponse struct {
	Status  string
	Error   string
	Message string
	Data    struct {
		AuthToken string
		UserId    string
	}
	Response string
}

type PostMessageResponse struct {
	Success bool
	Error   string
}

type LogoutResponse struct {
	Status string
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
}

const (
	defaultTemplate = "{{ .Check.Output }}"
)

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-rocketchat-handler",
			Short:    "Sensu Handler to send messages to rocketchat chat service",
			Keyspace: "sensu.io/plugins/sensu-rocketchat-handler/config",
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
	//Construct Msg from Event
	message := "test msg"

	// Login if not using token auth
	loginData := LoginResponse{}
	if len(plugin.Token) != 0 {
		loginData.Status = "success"
	} else {
		loginData = login()
	}

	if loginData.Status == "success" {
		if plugin.DryRun {
			log.Printf("DryRun Report:\n Channel: %v Server: %v\n Msg: %v",
				plugin.Channel, plugin.Url, message)
		}
		postMessageData := postMessage(message)
		if postMessageData.Success {
			if plugin.Verbose {
				log.Println("RocketChat Message sent to channel: #" + plugin.Channel + " <" + plugin.Url + "> message: " + message + " [ok]")
			}
		} else {
			log.Println("RocketChat Message sent to channel: #" + plugin.Channel + " <" + plugin.Url + "> message: " + message + " [error]")
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
		}
	} else {
		log.Println("RocketChat Login Failed with msg: " + loginData.Message)
		log.Fatal(loginData.Response)
	}
	return nil
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
	if plugin.Verbose {
		log.Printf("Login Http Request: %+v", req)
	}
	loginResponse := LoginResponse{}
	if plugin.DryRun {
		loginResponse.Status = "success"
		loginResponse.Response = "Dry Run: No RocketChat login request made"
		loginResponse.Data.AuthToken = "dryrun"
		loginResponse.Data.UserId = "dryrun"
	} else {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(b, &loginResponse)
		loginResponse.Response = string(b)
	}
	plugin.Token = loginResponse.Data.AuthToken
	plugin.UserID = loginResponse.Data.UserId
	return loginResponse
}

func postMessage(message string) PostMessageResponse {
	u, err := url.Parse(plugin.Url)
	u.Path = path.Join(u.Path, "/api/v1/chat.postMessage")
	body := strings.NewReader(`channel=#` + plugin.Channel + `&text=` + message)
	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-Auth-Token", plugin.Token)
	req.Header.Set("X-User-Id", plugin.UserID)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if plugin.Verbose {
		log.Printf("PostMessage Http Request: %+v", req)
	}
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
	if plugin.Verbose {
		log.Printf("Logout Http Request: %+v", req)
	}
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
