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
/undo - deletes the roles that were assigned the last time (FIXME: does not carry over on bot reboot)
/assignroles - automatically create and assign roles from a spreadsheet (special access privileges required)`

// Hardcode all IDs that are allowed to use potentially dangerous administrative actions, such as /assignroles
var AUTHORIZED_USERS = map[string]bool{
	"96492516966174720": true, //valar
}

var newly_created_roles []string

// Struct to store information about a team-role
type team_t struct {
	name               string
	members            []string
	color              int
	mentionable        bool
	perms              int64
	fully_created_role discordgo.Role
}

// Struct to keep track of how /assignroles is being used (we want to disallow multiple simultanious use)
type assignroles_t struct {
	isInUse   bool               //is set to true if command is in use
	AuthorID  string             //author who last initiated /assignroles
	ChannelID string             //channel where /assignroles was initiated from
	session   *discordgo.Session //the current session
}

var assignroles_s assignroles_t

// Is called by AddHandler every time a new message is created - on ANY channel the bot has access to
func scan_message(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Respond to specific messages
	switch m.Content {
	case "/undo":
		undo(s, m)
		_, err := s.ChannelMessageSend(m.ChannelID, "Deleted roles.")
		if err != nil {
			fmt.Println(err)
		}
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
	//fmt.Println(m.GuildIDO

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

	// 1. suppose we have a list of roles to create
	// TODO: We should get this data from parsing a file/spreadsheet/json etc..
	teams := make([]team_t, 2)

	teams[0].name = "Samyang Fire"
	teams[0].members = append(teams[0].members, "96492516966174720") //fixme: IDK how to go from tag to ID
	teams[0].color = 15548997                                        //red
	teams[0].perms = 0

	teams[1].name = "Teamliquid"
	teams[1].members = append(teams[0].members, "96492516966174720") //fixme: IDK how to go from tag to ID
	teams[1].color = 5793266                                         //blue
	teams[1].perms = 0

	// 2. Create some new roles
	roles_needed_amount := len(teams)
	for i := 0; i < roles_needed_amount; i++ {
		new_role, err := s.GuildRoleCreate(m.GuildID)
		teams[i].fully_created_role = *new_role
		if err != nil {
			fmt.Println(err)
		}
		//TODO: We should save a list of the newly created role IDs so we can simply remove a batch of incorrectly created roles
		newly_created_roles = append(newly_created_roles, new_role.ID)

	}

	// 3. Write the configuration of each role to the newly created roles
	for i := 0; i < roles_needed_amount; i++ {
		// Apply the specified configuration to the new role
		role_post_creation, err := s.GuildRoleEdit(m.GuildID, teams[i].fully_created_role.ID, teams[i].name, teams[i].color, false, teams[0].perms, true)
		if err != nil {
			fmt.Println(err)
			return "Error in test() execution 1"
		}
		fmt.Println("Created new role - NAME:", role_post_creation.Name, "ID:", role_post_creation.ID)
	}

	// 4. Add each team role to the correct users
	for i := 0; i < roles_needed_amount; i++ {
		for z := 0; z < len(teams[i].members); z++ {
			err := s.GuildMemberRoleAdd(m.GuildID, teams[i].members[z], teams[i].fully_created_role.ID)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	message := "Test function executed successfully"
	return message
}

func undo(s *discordgo.Session, m *discordgo.MessageCreate) {
	for _, v := range newly_created_roles {
		//s.State.RoleRemove(m.Member.GuildID, v)
		s.GuildRoleDelete(m.GuildID, v)
		fmt.Println(v)
	}
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
