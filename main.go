package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	//third party dependencies:
	"github.com/bwmarrin/discordgo"
)

// Just a string of all available commands for use by the the /help command
const AVAILABLE_COMMANDS string = `Available Commands:
/help
/test
/assignroles - automatically create and assign roles from a spreadsheet (special access privileges required)`

// Hardcode all IDs that are allowed to use potentially dangerous administrative actions, such as /assignroles
var AUTHORIZED_USERS = map[string]bool{
	"96492516966174720": true, //valar
}

// Struct to keep track of how /assignroles is being used (we want to disallow multiple simultanious use)
type assignroles_t struct {
	isInUse   bool               //is set to true if command is in use
	AuthorID  string             //author who last initiated /assignroles
	ChannelID string             //channel where /assignroles was initiated from
	session   *discordgo.Session //the current session
}

var assignroles_s assignroles_t

// Is called by AddHandler time a new message is created - on ANY channel the bot has access to
func scan_message(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Respond to specific messages
	switch m.Content {
	case "/assignroles":
		if assignroles_s.isInUse { //check if this command is in use first and disallow simultanious use
			_, err := s.ChannelMessageSend(m.ChannelID, "[:exclamation:] Error, `/assignroles` is currently in use. Unable to comply.\nIf you are trying to assign roles, please post a link to the spreadsheet you want me to use.")
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		if AUTHORIZED_USERS[m.Author.ID] { // if the user is authorized, proceed with the operation
			_, err := s.ChannelMessageSend(m.ChannelID,
				"You are "+m.Author.Username+" and your authorization has been granted!\n\n**Post the link to the spreadsheet you want me to use.**\n\n[:sparkles:] Awaiting input from "+m.Author.Mention()+"...")
			if err != nil {
				fmt.Println(err)
				return
			}
			assignroles_s.isInUse = true
			assignroles_s.session = s
			assignroles_s.AuthorID = m.Author.ID
			assignroles_s.ChannelID = m.ChannelID
			return

		} else {
			_, err := s.ChannelMessageSend(m.ChannelID, "[☆] You are "+m.Author.Username+"\n[☆] Authorization: denied."+m.Author.ID)
			if err != nil {
				fmt.Println(err)
			}
		}
		// =====================================================================
		// USE THIS COMMAND FOR TESTING
		// =====================================================================
	case "/test":
		message := test(s, m)

		_, err := s.ChannelMessageSend(m.ChannelID, message)
		if err != nil {
			fmt.Println(err)
			return
		}
		// =====================================================================
		// =====================================================================

	case "/help":
		_, err := s.ChannelMessageSend(m.ChannelID, AVAILABLE_COMMANDS)
		if err != nil {
			fmt.Println(err)
		}
	}

	if assignroles_s.isInUse && assignroles_s.AuthorID == m.Author.ID {
		_, err := s.ChannelMessageSend(m.ChannelID, "[:sparkles:] Trying to fetch spreadsheet from URL: \""+m.Content+"\"")
		if err != nil {
			fmt.Println(err)
		}
		assignroles_s.isInUse = false //reset the data so /assignroles can be used again
	}

}

// Executes with side effects and returns final message to be send
func test(s *discordgo.Session, m *discordgo.MessageCreate) string {

	// Create a new role on the server
	new_role_id, err := s.GuildRoleCreate(m.GuildID)

	// Specify the configuration of the new role (name, color, perms, etc)
	role_name := "MEEP"
	role_color := 42
	hoist := false
	mentionable := true
	var perm int64 = 0

	// Apply the specified configuration to the new role
	role_post_creation, err := s.GuildRoleEdit(m.GuildID, new_role_id.ID, role_name, role_color, hoist, perm, mentionable)
	if err != nil {
		fmt.Println(err)
		return "Error in test() execution 1"
	}
	fmt.Println("Created new role - NAME:", role_post_creation.Name, "ID:", role_post_creation.ID)

	// Add the role to the author of test message
	err = s.GuildMemberRoleAdd(m.GuildID, m.Author.ID, new_role_id.ID)
	if err != nil {
		fmt.Println(err)
		return "Error in test() execution 2"
	}

	message := "Test function executed successfully"
	return message
}

func main() {
	// Check to make sure a bot auth token was supplied on startup
	if len(os.Args) < 2 || len(os.Args) > 2 {
		fmt.Println("Error: You must supply EXACTLY one argument (the bot's authorization token) on startup.")
		os.Exit(1)
	}

	// this is the auth token that allows us to interact with the discord api through a registered bot
	TOKEN := os.Args[1]

	// Returns dg which is of type session!
	dg, err := discordgo.New("Bot " + TOKEN)
	if err != nil {
		fmt.Println("Error creating discord session", err)
		os.Exit(1)
	}

	// Register scan_message as a callback func for message events
	dg.AddHandler(scan_message)

	// Exclusively care about receiving message events
	//dg.Identify.Intents = discordgo.IntentsGuildMessages
	dg.Identify.Intents = discordgo.IntentsAll
	//dg.Identify.Intents = discordgo.IntentsGuilds

	// Establish the discord sessin through discord bot api
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection", err)
		os.Exit(1)
	}

	// Keep running until exit signal is received..
	fmt.Println("Bot is running..")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Gracefully close down the Discord session on exit
	dg.Close()
	fmt.Printf("\nBot exited gracefully.\n")
}
