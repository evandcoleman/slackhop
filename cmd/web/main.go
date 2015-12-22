package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/edc1591/slackhop/models"
	"github.com/nlopes/slack"
)

func main() {
	// 1 year ago
	// doWork(1, "#tizzimehop_1yr_ago", "C03S13MA4")

	configFile, err := ioutil.ReadFile("./config.json")
	if err != nil {
		fmt.Println("Failed to read config.json file: %v", err)
		os.Exit(1)
	}

	var config models.Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		fmt.Println("Failed to parse config.json file: %v", err)
		os.Exit(1)
	}

	for _, channel := range config.Channels {
		go func(channel models.Channel) {
			t := time.Now().AddDate(-channel.YearsAgo, 0, 0) // start with one year ago today
			for {
				fmt.Println("Replaying messages for", t)
				replayMessages(t, channel)
				t = t.AddDate(0, 0, 1) // increment to the next day

				if t.After(time.Now()) {
					break
				}
			}
		}(channel)
	}

	http.HandleFunc("/health", Handler)
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		log.Fatal("Failed to listen and serve", "error", err)
	}
}

func Handler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
}

func replayMessages(t time.Time, channel models.Channel) {
	// Setup the HTTP client
	client := &http.Client{}

	token := os.Getenv("SLACK_API_TOKEN")
	if len(token) == 0 {
		fmt.Println("Missing Slack API token")
		return
	}

	startTime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	endTime := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
	count := 500

	api := slack.New(token)
	users, err := api.GetUsers()
	if err != nil {
		fmt.Println("Error fetching users:", err)
		return
	}

	var messages []slack.Message
	oldest := fmt.Sprintf("%v", startTime.Unix())
	for {
		params := slack.HistoryParameters{
			Count:  count,
			Oldest: oldest,
			Latest: fmt.Sprintf("%v", endTime.Unix()),
		}
		history, err := api.GetChannelHistory(channel.SourceChannelId, params)
		if err != nil {
			fmt.Println("Error fetching slack history:", err)
			break
		}

		messages = append(messages, history.Messages...)
		oldest = history.Latest

		if !history.HasMore {
			break
		}
	}

	for _, message := range messages {
		ts, _ := strconv.ParseFloat(message.Timestamp, 10)
		messageDate := time.Unix(int64(ts), 0)
		yearsAgo := time.Now().Year() - messageDate.Year()
		newDate := messageDate.AddDate(channel.YearsAgo, 0, 0)
		lastMessageSecondsAgo := time.Now().Unix() - newDate.Unix()
		if (newDate.After(time.Now()) || lastMessageSecondsAgo < 2) && yearsAgo == channel.YearsAgo {
			sd := newDate.Sub(time.Now())
			fmt.Printf("Next %v year(s) ago message in %v at %v\n", yearsAgo, sd, newDate.Local())
			time.Sleep(sd)

			for _, user := range users {
				if user.ID == message.User {
					slackMessage := makeReplayMessage(message, user, users, channel.TargetChannelId, channel.SuppressNotifications)
					// Post the message to Slack
					postToSlack(client, slackMessage)

					break
				}
			}
		}
	}
}

func postToSlack(client *http.Client, message models.SlackMessage) {
	b, err := json.Marshal(message)
	if err != nil {
		fmt.Println(err)
		return
	}

	url := os.Getenv("SLACK_WEBHOOK_URL")
	if len(url) > 0 {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
		}
		defer resp.Body.Close()
	} else {
		fmt.Println(string(b))
	}
}

func makeReplayMessage(m slack.Message, user slack.User, users []slack.User, channelId string, suppressNotifications bool) models.SlackMessage {
	text := m.Text

	if suppressNotifications {
		re := regexp.MustCompile("<@U([A-Z0-9]{8}(\\|[\\S]+)?)>")
		matches := re.FindAllString(text, -1)
		for _, match := range matches {
			parts := strings.Split(match, "|")
			part := parts[0]
			var userId string
			if len(parts) > 1 {
				userId = part[2:len(part)]
			} else {
				userId = part[2 : len(part)-1]
			}
			for _, u := range users {
				if u.ID == userId {
					text = strings.Replace(text, match, "@"+u.Name, 1)
				}
			}
		}

		// lol naming
		otherRe := regexp.MustCompile("<@([\\S]+)>")
		otherMatches := otherRe.FindAllString(text, -1)
		for _, match := range otherMatches {
			text = strings.Replace(text, match, match[2:len(match)-1], 1)
		}

		text = strings.Replace(text, "<!everyone>", "@everyone", 1)
		text = strings.Replace(text, "<!channel>", "@channel", 1)
		text = strings.Replace(text, "<!here>", "@here", 1)
	}

	sm := models.SlackMessage{
		Text:      text,
		Channel:   channelId,
		Name:      user.Profile.FirstName + " " + user.Profile.LastName,
		AvatarUrl: user.Profile.Image192,
	}
	if len(user.Profile.FirstName) == 0 {
		sm.Name = user.Name
	}

	if m.File != nil && strings.HasPrefix(m.File.Mimetype, "image") {
		attachment := slack.Attachment{
			ImageURL: m.File.URL,
		}
		sm.Attachments = []slack.Attachment{attachment}
	}

	return sm
}
