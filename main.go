package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Flags
var (
	BotToken    = flag.String("token", "", "Bot token")
	ServerIP    = flag.String("serverIP", "", "Server IP to check")
	Channel     = flag.String("channel", "", "Discord channel to broadcast")
	done        chan bool
	channelIDs  []string
	serverAlive bool

	alive = "The server is alive"
	dead  = "The server is dead"
)

func init() {
	flag.Parse()
}

func main() {
	s, _ := discordgo.New("Bot " + *BotToken)
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Bot is ready")
	})

	s.Identify.Intents |= discordgo.IntentMessageContent

	s.AddHandler(ready)
	s.AddHandler(messageCreate)
	s.AddHandler(guildCreate)

	err := s.Open()
	if err != nil {
		log.Printf("cannot open the session: %v", err)
		os.Exit(1)
	}
	defer s.Close()

	serverAlive = checkServer()

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Press CTRL-C to exit")
	sc := make(chan os.Signal, 1)
	done = make(chan bool)

	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Println("Exiting")
	done <- true

	// remove all registered commands
	registeredCommands, err := s.ApplicationCommands(s.State.User.ID, "")
	if err != nil {
		log.Printf("Could not fetch registered commands: %v\n", err)
	}
	for _, v := range registeredCommands {
		err := s.ApplicationCommandDelete(s.State.User.ID, "", v.ID)
		if err != nil {
			log.Printf("Cannot delete '%v' command: %v\n", v.Name, err)
		}
	}
}

func ready(s *discordgo.Session, event *discordgo.Ready) {

	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				if t.Second()%30 == 0 {
					// fmt.Println("Tick at", t)
					isAliveNow := checkServer()
					for _, v := range channelIDs {
						if isAliveNow && !serverAlive {
							_, _ = s.ChannelMessageSend(v, alive)

							serverAlive = true
						}
						if !isAliveNow && serverAlive {
							_, _ = s.ChannelMessageSend(v, dead)
							serverAlive = false
						}
					}
				}
			}
		}
	}()
}

func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {

	if event.Guild.Unavailable {
		return
	}

	log.Printf("adding guild: %s (%s)\n", event.Guild.Name, event.Guild.ID)

	for _, channel := range event.Guild.Channels {
		if strings.Contains(channel.Name, *Channel) && channel.Type == discordgo.ChannelTypeGuildText {
			channelIDs = append(channelIDs, channel.ID)
			log.Printf("found text channel on guild: %s\n", *Channel)
			return
		}
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.HasPrefix(m.Content, "!server") {

		if serverAlive {
			_, _ = s.ChannelMessageSend(m.ChannelID, alive)
		} else {
			_, _ = s.ChannelMessageSend(m.ChannelID, dead)
		}
	}
}

func checkServer() bool {
	servers := ServerList{}
	err := GetServersAtAddress(*ServerIP, &servers)
	if err != nil {
		log.Printf("error getting servers: %v", err)
	}
	if !servers.Response.Success {
		log.Printf("error getting server list from steam: %v", err)
	}

	if len(servers.Response.Servers) != 0 {
		// fmt.Println("found server!")
		return true
	} else {
		// fmt.Println("no servers found")
		return false
	}
}

type ServerList struct {
	Response Response `json:"response"`
}

type Response struct {
	Success bool     `json:"success"`
	Servers []Server `json:"servers"`
}

type Server struct {
	Address string `json:"addr"`
	AppID   uint32 `json:"appid"`
	GameDir string `json:"gamedir"`
}

func GetServersAtAddress(ip string, target interface{}) error {
	r, err := http.Get(fmt.Sprintf("https://api.steampowered.com/ISteamApps/GetServersAtAddress/v1/?addr=%s", ip))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}
