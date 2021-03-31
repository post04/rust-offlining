package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

type config struct {
	Token     string   `json:"token"`
	Players   []string `json:"players"`
	Prefix    string   `json:"prefix"`
	ChannelID string   `json:"channelID"`
	GuildID   string   `json:"guildID"`
	MessagID  string   `json:"messageID"`
}

var (
	c                  config
	baseURL            = "https://www.battlemetrics.com/players/"
	lastSeenRegex      = regexp.MustCompile(`<dt>Last Seen<\/dt><dd><time dateTime="[A-z0-9-:.]+" title="[A-z0-9:, ]+">[0-9A-z ]+<\/time><\/dd>`)
	currentServerRegex = regexp.MustCompile(`<dt>Current Server\(s\)<\/dt><dd>.+<\/dd>`)
	usernameRegex      = regexp.MustCompile(`<h3 class="css-8uhtka">.+<\/h3>`)
	rustServerRegex    = regexp.MustCompile(`<a href="\/servers\/rust\/[0-9]+">.+<\/a>`)
)

func getConfig() {
	f, err := ioutil.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(f, &c)
	if err != nil {
		panic(err)
	}
}

func updateConfig(c config) {
	out, _ := json.MarshalIndent(c, "", "	")
	ioutil.WriteFile("config.json", out, 0064)
}

func convert(thing string) int {
	kek, err := strconv.Atoi(thing)
	if err != nil {
		return 1
	}
	return kek

}

func checkValid(ID string) bool {
	resp, err := http.Get(baseURL + ID)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), "<h3 class=\"css-8uhtka\">")
}

func checkOnline(ID string) (bool, string, string) {
	resp, err := http.Get(baseURL + ID)
	if err != nil {
		return false, "", ""
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)

	accountNameList := usernameRegex.FindAllString(string(b), -1)
	if len(accountNameList) < 1 {
		return false, "", ""
	}
	username := strings.Replace(accountNameList[0], "<h3 class=\"css-8uhtka\">", "", 1)
	username = strings.Replace(username, "</h3>", "", 1)

	currentServerList := currentServerRegex.FindAllString(string(b), -1)
	if len(currentServerList) < 1 {
		return false, "", ""
	}
	currentServerChunk := currentServerList[0]
	if strings.HasPrefix(currentServerChunk, "<dt>Current Server(s)</dt><dd>Not online</dd>") {
		return false, "", username
	}
	currentServerChunk = currentServerRegex.FindAllString(currentServerChunk, -1)[0]
	rustServerList := rustServerRegex.FindAllString(currentServerChunk, -1)
	if len(rustServerList) < 0 {
		return false, "", username
	}

	rustServerChunk := strings.Split(rustServerList[0], "\">")[1]
	rustServer := strings.Split(rustServerChunk, "</a>")[0]

	return true, rustServer, username

}

func updatePlayers(session *discordgo.Session) {
	if c.MessagID == "" {
		msg, err := session.ChannelMessageSend(c.ChannelID, "filler")
		if err != nil {
			panic(err)
		}
		c.MessagID = msg.ID
	}
	final := "```\n"
	for _, player := range c.Players {
		online, server, username := checkOnline(player)
		if username == "" {
			if online {
				final += fmt.Sprintf("%s\n	Online: %t\n	Server: %s\n", player, online, server)
			} else {
				final += fmt.Sprintf("%s\n	Online: %t\n", player, online)
			}
		} else {
			if online {
				final += fmt.Sprintf("%s\n	Online: %t\n	Server: %s\n", username, online, server)
			} else {
				final += fmt.Sprintf("%s\n	Online: %t\n", username, online)
			}
		}
	}
	final += "```"
	session.ChannelMessageEdit(c.ChannelID, c.MessagID, final)
}

func getPosition(players []string, player string) int {
	for i, player1 := range players {
		if player1 == player {
			return i
		}
	}
	return -1
}

func messageCreate(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if msg.Author.Bot || !strings.HasPrefix(msg.Content, c.Prefix) || msg.GuildID != c.GuildID {
		return
	}
	parts := strings.Fields(msg.Content)
	if len(parts) <= 0 {
		return
	}
	command := parts[0][len(c.Prefix):]
	command = strings.ToLower(command)
	if command == "addplayer" {
		if len(parts) < 2 {
			session.ChannelMessageSend(msg.ChannelID, "Please provide a userID to add. You get this from the battlementrics link for a profile.\nExample: `https://www.battlemetrics.com/players/989425839` -> `989425839`")
			return
		}

		if converted := convert(parts[1]); converted == 1 {
			session.ChannelMessageSend(msg.ChannelID, "Please provide a **VALID** userID to add. You get this from the battlementrics link for a profile.\nExample: `https://www.battlemetrics.com/players/989425839` -> `989425839`")
			return
		}

		valid := checkValid(parts[1])
		if !valid {
			session.ChannelMessageSend(msg.ChannelID, "It appears that ID isn't valid according to battlementrics...")
			return
		}
		c.Players = append(c.Players, parts[1])
		updateConfig(c)
	}
	if command == "force" {
		updatePlayers(session)
		session.ChannelMessageSend(msg.ChannelID, "Force updated players... <#"+c.ChannelID+">")
	}
	if command == "check" {
		if len(parts) < 2 {
			session.ChannelMessageSend(msg.ChannelID, "Please provide a userID to add. You get this from the battlementrics link for a profile.\nExample: `https://www.battlemetrics.com/players/989425839` -> `989425839`")
			return
		}

		if converted := convert(parts[1]); converted == 1 {
			session.ChannelMessageSend(msg.ChannelID, "Please provide a **VALID** userID to add. You get this from the battlementrics link for a profile.\nExample: `https://www.battlemetrics.com/players/989425839` -> `989425839`")
			return
		}

		valid := checkValid(parts[1])
		if !valid {
			session.ChannelMessageSend(msg.ChannelID, "It appears that ID isn't valid according to battlementrics...")
			return
		}
		online, server, username := checkOnline(parts[1])
		session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("```\nOnline: %t\nServer: %s\nUsername: %s```", online, server, username))
	}
	if command == "removeplayer" {
		if len(parts) < 2 {
			session.ChannelMessageSend(msg.ChannelID, "Please provide a userID to remove. You get this from the battlementrics link for a profile.\nExample: `https://www.battlemetrics.com/players/989425839` -> `989425839`")
			return
		}

		if converted := convert(parts[1]); converted == 1 {
			session.ChannelMessageSend(msg.ChannelID, "Please provide a **VALID** userID to remove. You get this from the battlementrics link for a profile.\nExample: `https://www.battlemetrics.com/players/989425839` -> `989425839`")
			return
		}

		position := getPosition(c.Players, parts[1])
		if position == -1 {
			session.ChannelMessageSend(msg.ChannelID, "That userID isn't in the list! Use `"+c.Prefix+"list` to show all UserID's being tracked!")
			return
		}
		c.Players = append(c.Players[:position], c.Players[position+1:]...)
		updateConfig(c)
	}
	if command == "list" {
		session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("```\n%s\n```", strings.Join(c.Players, "\n")))
	}
	if command == "help" {
		session.ChannelMessageSendEmbed(msg.ChannelID, &discordgo.MessageEmbed{Description: fmt.Sprintf(`
		%saddplayer <id> - adds a user to the list to the checked
		%sremoveplayer <id> - removes a user from the list to be checked
		%slist - shows the list of users that will be being checked
		%sforce - forces a update and check in the <#%s> channel
		%scheck <id> - checks if a user id online`, c.Prefix, c.Prefix, c.Prefix, c.Prefix, c.ChannelID, c.Prefix)})
	}
}

func ready(session *discordgo.Session, readyEvt *discordgo.Ready) {
	fmt.Printf("Logged in under %s#%s\n", readyEvt.User.Username, readyEvt.User.Discriminator)
	for {
		updatePlayers(session)
		time.Sleep(1 * time.Minute)
	}
}

func main() {
	getConfig()
	bot, err := discordgo.New("Bot " + c.Token)
	if err != nil {
		fmt.Println("error creating Discord session", err)
		return
	}
	bot.AddHandler(messageCreate)
	bot.AddHandler(ready)
	err = bot.Open()
	if err != nil {
		fmt.Println("error opening connection", err)
		return
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	_ = bot.Close()
	updateConfig(c)
}
