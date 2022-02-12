package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"

	//third party dependencies:
	"github.com/bwmarrin/discordgo"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

/* #####
// IMPORTANT HARDCODED VALUES
// The values here all need to be set correctly for all functionality to work!
*/
// Hardcode all IDs that are allowed to use potentially dangerous administrative actions, such as /assignroles
var AUTHORIZED_USERS = map[string]bool{
	"96492516966174720": true, //valar
}

var newly_created_roles []string // Holds newly created discord role IDs
var updateRoles_s rolesCmd_t     // info about /update roles command while being used

const SPREADSHEET_ID string = "1K-jV6-CUmjOSPW338MS8gXAYtYNW9qdMeB7XMEiQyn0" // google sheet ID
const SERVER_ID string = "856762567414382632"                                // the discord server ID
const ZERG_ROLE_ID string = "941808009984737281"
const TERRAN_ROLE_ID string = "941808071817187389"
const PROTOSS_ROLE_ID string = "941808145993441331"
const TIER0_ROLE_ID string = "941808145993441331"
const TIER1_ROLE_ID string = "942081263358070794"
const TIER2_ROLE_ID string = "942081322325794839"
const TIER3_ROLE_ID string = "942081409500213308"
const COACH_ROLE_ID string = "942083540739317811"
const ASST_COACH_ROLE_ID string = "941808582410764288"
const PLAYER_ROLE_ID string = "942083605667131443"

// help cmd text
const AVAILABLE_COMMANDS string = `
[ /help        - show commands                        ]
[ /test        - test command                         ]
[ /delete      - delete roles                         ]
[ /updateroles - update roles from google spreadsheet ]
`

// Discord message formatting strings
const DIFF_MSG_START string = "```diff\n"
const DIFF_MSG_END string = "\n```"
const FIX_MSG_START string = "```fix\n"
const FIX_MSG_END string = "\n```"

//##### End of Hardcoded values

/* DATA STRUCTURES
##### */

type player_t struct {
	discord_Name string
	discord_ID   string
	tier         string
	race         string
	usergroup    string
}

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
type rolesCmd_t struct {
	isInUse   bool               //is set to true if command is in use
	AuthorID  string             //author who last initiated /assignroles
	ChannelID string             //channel where /assignroles was initiated from
	session   *discordgo.Session //the current session
}

//##### End of data structures

// error check as a func because it's annoying to write "if err != nil { .. .. }" over and over
func checkError(err error) {
	if err != nil {
		fmt.Println(err)
		//panic(err.Error())
	}
}

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
	case "/get_server_id": // Prints the ID of the discord server
		_, err := s.ChannelMessageSend(m.ChannelID, m.GuildID)
		if err != nil {
			fmt.Println(err)
		}
	case "/updateroles":
		if updateRoles_s.isInUse { //check if this command is in use first and disallow simultanious use
			_, err := s.ChannelMessageSend(m.ChannelID, "[:exclamation:] Error, `/updateroles` is currently in use. Unable to comply.\nIf you are trying to assign roles, please post a link to the spreadsheet you want me to use.")
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		if AUTHORIZED_USERS[m.Author.ID] { // if the user is authorized, proceed with the operation
			_, err := s.ChannelMessageSend(m.ChannelID, FIX_MSG_START+"+ /updateroles EXECUTION HAS STARTED"+FIX_MSG_END)
			if err != nil {
				fmt.Println(err)
				return
			}
			updateRoles_s.isInUse = true
			updateRoles_s.session = s
			updateRoles_s.AuthorID = m.Author.ID
			updateRoles_s.ChannelID = m.ChannelID
			update_roles(s, m)            //testing new command
			updateRoles_s.isInUse = false //reset the data so /updateroles can be used again
			_, err = s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"+ /updateroles EXECUTION HAS FINISHED"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
				return
			}

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
		_, err := s.ChannelMessageSend(m.ChannelID, "```ini\n"+AVAILABLE_COMMANDS+"\n```")
		if err != nil {
			fmt.Println(err)
		}
	}
	//fmt.Println(m.GuildIDO

	/* REMOVE THIS LATER, NO LONGER NEEDED?
	if updateRoles_s.isInUse && updateRoles_s.AuthorID == m.Author.ID {
		_, err := s.ChannelMessageSend(m.ChannelID, "[:sparkles:] Updating races! ")
		if err != nil {
			fmt.Println(err)
		}
		update_roles(s, m)              //testing
		updateRoles_s.isInUse = false //reset the data so /assignroles can be used again
	}
	*/
}

// Executes with side effects and returns final message to be send
func test(s *discordgo.Session, m *discordgo.MessageCreate) string {
	//x := m.Author.Username //the username without nums
	y := m.Author.String()

	fmt.Println(y)
	return (y + "test completed\n")

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

// Delete roles that were assigned?
func undo(s *discordgo.Session, m *discordgo.MessageCreate) {
	for _, v := range newly_created_roles {
		//s.State.RoleRemove(m.Member.GuildID, v)
		s.GuildRoleDelete(m.GuildID, v)
		fmt.Println(v)
	}
}

// Build a map of he desired user state (discord roles) from google sheets
// see: https://developers.google.com/sheets/api/guides/concepts
func get_sheet_state(players map[string]player_t) map[string]player_t {

	// Read the secret file (google api key to access google sheets)
	data, err := ioutil.ReadFile("secret.json")
	checkError(err)
	// Create oAuth client for google sheets
	conf, err := google.JWTConfigFromJSON(data, sheets.SpreadsheetsScope)
	checkError(err)
	client := conf.Client(context.TODO())
	srv, err := sheets.New(client)
	checkError(err)

	// Read sheet name cells from spreadsheet
	target_screen_names := "Player List" + "!A1:A"
	screenNameResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_screen_names).Do()
	checkError(err)

	// Read discord name cells from spreadsheet
	target_discord_names := "Player List" + "!B1:B"
	discord_NameResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_discord_names).Do()
	checkError(err)

	// Read race cells from spreadsheet
	target_ingame_race := "Player List" + "!E1:E"
	ingameRaceResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_ingame_race).Do()
	checkError(err)

	// Read usergroup cells from spreadsheet
	target_usergroup := "Player List" + "!C1:C"
	usergroupResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_usergroup).Do()
	checkError(err)

	// Read player tier from.. teams sheet
	//target_usergroup := "Player List" + "!C1:C"
	//usergroupResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_usergroup).Do()
	//checkError(err)

	//Extract the screen names of the players
	//for _, row := range resp.Values {
	// Loop over the data and add it to the players map
	for i := 0; i < len(screenNameResp.Values); i++ {
		//Extract the screen name
		a := fmt.Sprint(screenNameResp.Values[i])
		a = strings.TrimPrefix(a, "[")
		a = strings.TrimSuffix(a, "]")

		//Extract the discord name
		b := fmt.Sprint(discord_NameResp.Values[i])
		b = strings.TrimPrefix(b, "[")
		b = strings.TrimSuffix(b, "]")

		//Extract ingame race
		c := fmt.Sprint(ingameRaceResp.Values[i])
		c = strings.TrimPrefix(c, "[")
		c = strings.TrimSuffix(c, "]")

		//Extract usergroup (coach/player/etc)
		d := fmt.Sprint(usergroupResp.Values[i])
		d = strings.TrimPrefix(d, "[")
		d = strings.TrimSuffix(d, "]")

		//add player data to the map
		players[a] = player_t{discord_Name: b,
			race:      c,
			usergroup: d}
	}

	return players
}

/* WIP: Assign roles based on desired
##### */
func update_roles(dg *discordgo.Session, m *discordgo.MessageCreate) {
	// 0. Get all the roles from the discord and make a map
	// roles_m["role_name"].*discordgo.Role
	allDiscordRoles, err := dg.GuildRoles(SERVER_ID)
	checkError(err)
	roles_m := make(map[string]*discordgo.Role)
	for _, b := range allDiscordRoles {
		roles_m[b.Name] = b
	}

	// 1. Get all the users in the discord
	allDiscordUsers_s, err := dg.GuildMembers(SERVER_ID, "", 1000)
	checkError(err)

	// 2. Create map of username#discriminator to discord_id
	// and also a map of discord_id -> bool to check if they exist
	discord_name_to_id_m := make(map[string]string)
	discord_id_exists := make(map[string]bool)
	for _, u := range allDiscordUsers_s {
		discord_name_to_id_m[u.User.String()] = u.User.ID
		discord_id_exists[u.User.ID] = true
	}

	// 3. Get desired state of roles from google sheets
	sheetUsrState := make(map[string]player_t)
	sheetUsrState = get_sheet_state(sheetUsrState)

	// 4. Check if the user from sheet have desired roles assigned
	// and if not -> assign it!
	for screen_name, _ := range sheetUsrState {
		a := sheetUsrState[screen_name].discord_Name
		cordUserid := discord_name_to_id_m[a]

		// Check if the user even exists on the server
		if !discord_id_exists[cordUserid] {
			fmt.Println("Error: couldn't find the user", screen_name, "on discord")
			continue // skip if the user doesn't exist
		}
		// Assign Coach/Assistant Coach/ Player
		didAssignGroupRole := false // set to true if we assigned Coach, Assistant Coach, or Player role.
		wishGroup := sheetUsrState[screen_name].usergroup
		switch wishGroup {
		case "Player":
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, PLAYER_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		case "Coach":
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PLAYER_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		case "Assistant Coach":
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PLAYER_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true

		}
		if didAssignGroupRole {
			cordMessage1 := fmt.Sprintf("> Assigned <@%s> %s to %s\n", cordUserid, screen_name, wishGroup)
			_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage1)
			checkError(err)
			didAssignGroupRole = false
		}

		// Assign Zerg/Terran/Protoss
		didAssignRaceRole := false // set to true if we assigned Zerg, Terran, or Protoss role.
		wishRace := sheetUsrState[screen_name].race
		switch wishRace {
		case "Zerg":
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, ZERG_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			didAssignRaceRole = true

		case "Terran":
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ZERG_ROLE_ID)
			checkError(err)
			didAssignRaceRole = true

		case "Protoss":
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ZERG_ROLE_ID)
			checkError(err)
			didAssignRaceRole = true

		}
		if didAssignRaceRole {
			cordMessage2 := fmt.Sprintf("> Assigned <@%s> %s to %s\n", cordUserid, screen_name, wishRace)
			_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage2)
			checkError(err)
			didAssignRaceRole = false
		}
	}
	//##### End of testing
}

func main() {

	/* Startup procedures:
	#####	*/

	// Check to make sure a bot auth token was supplied on startup
	if len(os.Args) < 2 || len(os.Args) > 2 {
		fmt.Println("Error: You must supply EXACTLY one argument (the bot's authorization token) on startup.")
		os.Exit(1)
	}

	// This is the auth token that allows us to interact with the discord api through a registered bot
	TOKEN := os.Args[1]

	// Returns dg which is of type session!
	dg, err := discordgo.New("Bot " + TOKEN)
	if err != nil {
		fmt.Println("Error creating discord session", err)
	}

	// Register scan_message as a callback func for message events
	dg.AddHandler(scan_message)

	// Exclusively care about receiving message events
	//dg.Identify.Intents = discordgo.IntentsGuildMessages
	dg.Identify.Intents = discordgo.IntentsAll
	//dg.Identify.Intents = discordgo.IntentsGuilds

	// Establish the discord session through discord bot api
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection", err)
		os.Exit(1)
	}
	//##### End of startup procedures

	/* Shutdown procedures
	##### */

	// Keep running until exit signal is received..
	fmt.Println("Bot is running..")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Gracefully close down the Discord session on exit
	dg.Close()
	fmt.Printf("\nBot exited gracefully.\n")

	//##### end of shutdown procedures
}
