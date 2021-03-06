package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	//third party dependencies:
	"github.com/bwmarrin/discordgo"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

/* #####
IMPORTANT HARDCODED CONSTANTS
The values here all need to be set correctly for all functionality to work!
##### */

// Hardcode all IDs that are allowed to use potentially dangerous administrative actions, such as /assignroles
var IS_AUTHORIZED_AS_ADMIN = map[string]bool{
	"96492516966174720": true, //valar
}

// Users with extra priviliges bot nothing dangerous
var IS_PRIVILEGED_USER = map[string]bool{
	"105697010165747712": true, //dada
	"228586200741445642": true, //Snipe
	"533205511185629202": true, //Y2kid
	"93204976779694080":  true, //Pete aka Pusagi
}

// CPL SERVER VALUES: (MASTER BRANCH)
const SPREADSHEET_ID string = "1Xd0ohSMrYKsB-d0g3OgbovA3BV4NntQg_ZXjDJ7js8I" // CPL MASTER SPREADSHEET ID
const DISCORD_SERVER_ID string = "426172214677602304"                        // CPL SERVER
const MATCH_REPORTING_CHANNEL_ID string = "945736138864349234"               // CPL CHANNEL
const CPL_CLIPS_CHANNEL_ID string = "868530162852057139"                     // CPL CLIPS CHANNEL
const ZERG_ROLE_ID string = "426370952402698270"
const TERRAN_ROLE_ID string = "426371039241437184"
const PROTOSS_ROLE_ID string = "426371009982103555"
const TIER0_ROLE_ID string = "686335315492732963"
const TIER1_ROLE_ID string = "486932541396221962"
const TIER2_ROLE_ID string = "486932586724065285"
const TIER3_ROLE_ID string = "486932645519818752"
const COACH_ROLE_ID string = "426370872740413440"
const ASST_COACH_ROLE_ID string = "514179771295334420"
const TEAM1_ROLE_ID string = "952362058282836079"
const TEAM2_ROLE_ID string = "952363361360810015"
const TEAM3_ROLE_ID string = "952363465299853373"
const TEAM4_ROLE_ID string = "952363498233536533"
const TEAM5_ROLE_ID string = "952363548166750259"
const TEAM6_ROLE_ID string = "952363607616794624"

//// TEST SERVER VALUESL (TESTING BRANCH)
//const CPL_CLIPS_CHANNEL_ID string = "945364478973861898"                     // TEST SERVER CLIPS CHANNEL
//const DISCORD_SERVER_ID string = "856762567414382632"                        // TEST SERVER ID
//const MATCH_REPORTING_CHANNEL_ID string = "945364478973861898"               // TEST CHANNEL ID
//const SPREADSHEET_ID string = "1K-jV6-CUmjOSPW338MS8gXAYtYNW9qdMeB7XMEiQyn0" // TEST TEST SHEEET ID
//const ZERG_ROLE_ID string = "941808009984737281"
//const TERRAN_ROLE_ID string = "941808071817187389"
//const PROTOSS_ROLE_ID string = "941808145993441331"
//const TIER0_ROLE_ID string = "942081263358070794"
//const TIER1_ROLE_ID string = "942081322325794839"
//const TIER2_ROLE_ID string = "942081354353487872"
//const TIER3_ROLE_ID string = "942081409500213308"
//const COACH_ROLE_ID string = "942083540739317811"
//const ASST_COACH_ROLE_ID string = "941808582410764288"

// Constants for use on get_sheet_state logic
const STAFF int = -1
const COACHES int = -2
const ASSISTANTCOACH int = -3
const PLAYER_AND_ASSITANT_COACH int = -4
const PLAYER int = -5
const TIER0 int = 0
const TIER1 int = 1
const TIER2 int = 2
const TIER3 int = 3

// Discord colors for role creation (decimal values of hex color codes)
const NEON_GREEN int = 2358021

// help cmd text (DONT write longer lines than this, this is the maximum that still looks good on mobile)
const AVAILABLE_COMMANDS string = `
[ /help           - show commands                     ]
[ /test           - test command                      ]
[                                                     ]
[ /scan_users     - identify users based on web info  ]
[ /assignroles    - assign roles based on players json]
[ /webassignroles - create and assign roles from sheet]
[ /deleteroles    - delete previously created roles   ]
`

const MATCH_REPORT_FORMAT_HELP_TEXT string = "G2: player_name 1-0 player_two\n```"

// Discord message formatting strings
const DIFF_MSG_START string = "```diff\n"
const DIFF_MSG_END string = "\n```"
const FIX_MSG_START string = "```fix\n"
const FIX_MSG_END string = "\n```"
const MATCH_ACCEPTED string = "```diff\n+ ACCEPTED: Message formatting passes the check\n```"

// Used during parsing logic for "Teams" spreadsheet
var NOT_PLAYER_NAME = map[string]bool{
	/*"Staff":   true,
	"Coaches": true,
	"Tier 0":  true,
	"Tier 2":  true,
	"Tier 3":  true,
	*/
	"":   true,
	" ":  true,
	"\n": true,
	"\t": true,
}

//##### End of Hardcoded values

/* DATA STRUCTURES
##### */

type user_t struct {
	Ign          string //ingame name
	Discord_name string
	Discord_id   string
	Race         string
	Tier         int //0, 1, 2, or 3
	Group        int //Coach, Assistant Coach, or Player
	Team         string
}

// Struct to store information about a team-role
type team_t struct {
	Name       string
	Discord_id string
	Members    []user_t
	Color      int
	Exists     bool
	//mentionable        bool
	//perms              int64
	//fully_created_role discordgo.Role
}

// define data structure for unmarshalling
// Fields have to be Uppwercase so that unmarshaller can see them from the other package!
// We can include lowercase fields but then the unmarshaller won't use them
type web_player_t struct { // use this if the json data has different name than Web_player_t
	WebUserId             int    `json:"id"`
	WebName               string `json:"Name"`
	DiscordName           string `json:"Discord_account"`
	Mmr                   int
	Timezone              string
	Registration_feedback string
	Team                  string
	Tier                  int
	Cpl_edition           int
	Elo                   int
	Availability          []int
	Helper_role           []int
	League_role           []int
	Race                  int
	Activity              int
	In_waitlist           bool
	Wins                  int
	Losses                int
	Ties                  int
	Discord_id            string
}

// Struct to keep track of how /assignroles is being used (we want to disallow multiple simultanious use)
type dangerousCommands_t struct {
	isInUse   bool               //is set to true if command is in use
	AuthorID  string             //author who last initiated /assignroles
	ChannelID string             //channel where /assignroles was initiated from
	session   *discordgo.Session //the current session
	cmdName   string
}

//##### End of data structures

/* #####
Global vars
##### */
var TOKEN string                          //discord api token
var NEW_BATCH_NAME string                 // Name of a batch of newly created roles
var newlyCreatedRoles []string            // Holds newly created discord role IDs
var newlyAssignedRoles [][2]string        // [roleid][userid]
var dangerousCommands dangerousCommands_t // Info about /update roles command while being used
var discordUsers = []*discordgo.Member{}  // slice of all users from discord
// Maps
var mapDiscordNameToCordID = map[string]string{}     // Used to lookup discordid from discord name
var mapDiscordIdExists = map[string]bool{}           // Used to check if the user exists on the server
var mapExistingDiscordRoles = map[string]bool{}      // Used to check if the role exists on the server
var mapBatchesOfCreatedRoles = map[string][]team_t{} // [batchName]{roleid, roleid, roleid, roleid}
var mapWebUserNameToWebUserId = map[string]int{}     // map of WebApp username to numerical WebApp user ID
var mapWebUserIdToPlayer = map[int]web_player_t{}    // this is the main map I want to use for accessing player data

//##### End of global vars

// error check as a func because it's annoying to write "if err != nil { ... }" over and over
func checkError(err error) {
	if err != nil {
		fmt.Println(err)
		//panic(err.Error()rolesCmd_rolesCmd_ss)
	}
}

// Is called by AddHandler every time a new message is created - on ANY channel the bot has access to
func scan_message(s *discordgo.Session, m *discordgo.MessageCreate) {

	if m.Author.ID == s.State.User.ID { // Ignore all messages created by the bot itself
		return
	}

	switch m.ChannelID { // Monitor messages from certain channels
	case CPL_CLIPS_CHANNEL_ID:
		parse_message_in_clips_channel(s, m)
	}

	// Trigger on interactive command
	if dangerousCommands.isInUse && m.Author.ID == dangerousCommands.AuthorID && IS_AUTHORIZED_AS_ADMIN[m.Author.ID] {
		switch dangerousCommands.cmdName {

		case "/deleteroles":
			if deleteroles_check_input(m.Content) {
				deleteroles(s, m, m.Content) // run role deletion
			} else {
				reset_dangerous_commands_status()
				_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /deleteroles ERROR: INVALID SELECTION"+DIFF_MSG_END)
				checkError(err)
			}
		}
	}

	// Handle first use and non-interactive commands
	switch m.Content {
	case "/scan_users":
		if !IS_AUTHORIZED_AS_ADMIN[m.Author.ID] { // Check for Authorization
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /scan_missing ERROR: "+m.Author.Username+" IS NOT AUTHORIZED"+DIFF_MSG_END)
			checkError(err)
			return
		} else {
			if dangerousCommands.isInUse { // One at a time
				_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /scan_missing ERROR: Dangerous command is in use\n"+DIFF_MSG_END)
				checkError(err)
				return
			}
			dangerousCommands.isInUse = true
			dangerousCommands.cmdName = "/scan_missing"
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"+ /scan_missing SCAN STARTING"+DIFF_MSG_END)
			scan_web_players(s, m) //run the scan
			checkError(err)
		}

	case "/assignroles":
		if !IS_AUTHORIZED_AS_ADMIN[m.Author.ID] { // Check for Authorization
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /assignroles ERROR: "+m.Author.Username+" IS NOT AUTHORIZED"+DIFF_MSG_END)
			checkError(err)
			return
		}
		if dangerousCommands.isInUse {
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /assignroles ERROR: Dangerous command is in use\n"+DIFF_MSG_END)
			checkError(err)
			return
		}
		_, err := s.ChannelMessageSend(m.ChannelID, FIX_MSG_START+"+ /assignroles ROLE ASSIGNMENT STARTED"+FIX_MSG_END)
		checkError(err)
		assign_roles_from_json(s, m)

	case "/deleteroles":
		if !IS_AUTHORIZED_AS_ADMIN[m.Author.ID] { // Check for Authorization
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /deleteroles ERROR: "+m.Author.Username+" IS NOT AUTHORIZED"+DIFF_MSG_END)
			checkError(err)
			return
		}
		if !dangerousCommands.isInUse { // Check if dangerous commands are available (onely one at a time is allowed)
			dangerousCommands.isInUse = true
			dangerousCommands.session = s
			dangerousCommands.AuthorID = m.Author.ID
			dangerousCommands.ChannelID = m.ChannelID
			dangerousCommands.cmdName = "/deleteroles"
			select_batch_to_delete(s, m) // Show available batches and prompt user selection
		} else {
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /deleteroles ERROR: "+m.Author.Username+" DANGEROUS COMMAND IS IN USE"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
			}
			return
		}

	case "/get_discord_server_id": // Prints the ID of the discord server
		_, err := s.ChannelMessageSend(m.ChannelID, m.GuildID)
		if err != nil {
			fmt.Println(err)
		}

	case "/parse_past_messages":
		if IS_AUTHORIZED_AS_ADMIN[m.Author.ID] {
			parse_past_messages(s, m)
			_, err := s.ChannelMessageSend(m.ChannelID, "/parse_past_messages complete\n")
			checkError(err)
		}

	case "/unassignroles": //not implemented yet
		_, err := s.ChannelMessageSend(m.ChannelID, "/unassignroles is not implemented yet")
		checkError(err)

	case "/webassignroles":
		if dangerousCommands.isInUse { //check if this command is in use first and disallow simultanious use
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /webassignroles ERROR: EXECUTION IN PROGRESS"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		if IS_AUTHORIZED_AS_ADMIN[m.Author.ID] { // if the user is authorized, proceed with the operation
			_, err := s.ChannelMessageSend(m.ChannelID, FIX_MSG_START+"+ /webassignroles ROLE UPDATE STARTED"+FIX_MSG_END)
			if err != nil {
				fmt.Println(err)
				return
			}
			dangerousCommands.isInUse = true
			dangerousCommands.session = s
			dangerousCommands.AuthorID = m.Author.ID
			dangerousCommands.ChannelID = m.ChannelID
			dangerousCommands.cmdName = "/assignroles"
			update_roles(s, m)                //testing new command
			dangerousCommands.isInUse = false //reset the data so /assignroles can be used again
			_, err = s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"+ /webassignroles ROLE UPDATE COMPLETE"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
				return
			}

			return

		} else {
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /webassignroles ERROR: "+m.Author.Username+" IS NOT AUTHORIZED"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
			}
		}
	case "/test": // USE THIS COMMAND FOR TESTING
		test(s, m)

	case "/help":
		_, err := s.ChannelMessageSend(m.ChannelID, "```ini\n"+AVAILABLE_COMMANDS+"\n```")
		if err != nil {
			fmt.Println(err)
		}
	}

	/* REMOVE THIS LATER, NO LONGER NEEDED?
	if dangerousCommands.isInUse && dangerousCommands.AuthorID == m.Author.ID {
		_, err := s.ChannelMessageSend(m.ChannelID, "[:sparkles:] Updating races! ")
		if err != nil {
			fmt.Println(err)
		}
		update_roles(s, m)              //testing
		dangerousCommands.isInUse = false //reset the data so /assignroles can be used again
	}
	*/

	if IS_PRIVILEGED_USER[m.Author.ID] || IS_AUTHORIZED_AS_ADMIN[m.Author.ID] {
		// Lookup a player and show their information
		if strings.Contains(m.Content, "/show") {
			userInput := strings.TrimLeft(m.Content, "/show")
			userInput = strings.TrimPrefix(userInput, " ")
			userInput = strings.TrimSuffix(userInput, "\n")
			userID := mapWebUserNameToWebUserId[userInput]
			player := mapWebUserIdToPlayer[userID]

			if mapDiscordIdExists[player.Discord_id] {
				message := "**Web Name**: " + fmt.Sprintln(player.WebName)
				message += "**Discord: ** " + "<@" + fmt.Sprintf(player.Discord_id) + ">\n"
				//message += "**Known Discord Names: **" + fmt.Sprintln(player.DiscordName)
				message += "**SnowflakeID: ** " + fmt.Sprintln(player.Discord_id)
				_, err := s.ChannelMessageSend(m.ChannelID, message)
				checkError(err)
			} else {
				_, err := s.ChannelMessageSend(m.ChannelID, userInput+" not found")
				checkError(err)
			}
		}
	}

}

// wrapper for sending message so we can do it concurrently
// Okay that didn't make it faster at all
func messageSendWrapper(s *discordgo.Session, m *discordgo.MessageCreate, c string) {
	_, err := s.ChannelMessageSend(m.ChannelID, c)
	checkError(err)
}

// Test function executes with side effects and returns final message to be send
func test(s *discordgo.Session, m *discordgo.MessageCreate) {
	a := mapWebUserIdToPlayer[42]
	s.ChannelMessageSend(m.ChannelID, a.WebName)
	b := mapWebUserNameToWebUserId["Neblime"]
	s.ChannelMessageSend(m.ChannelID, string(b))
}

/* //testfunc old
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
		newlyCreatedRoles = append(newlyCreatedRoles, new_role.ID)

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
*/

func select_batch_to_delete(s *discordgo.Session, m *discordgo.MessageCreate) {
	var batch string
	for n, b := range mapBatchesOfCreatedRoles {
		batch += n + ": "
		for _, v := range b {
			batch += fmt.Sprintf("\t<@%s> %s \n", v.Discord_id, v.Name)
		}
		batch += "\n"
	}
	_, err := s.ChannelMessageSend(m.ChannelID, FIX_MSG_START+"+ /deleteroles STARTING\n\nPlease enter batchnumber of roles to be deleted from below:\n\n"+FIX_MSG_END+batch)
	checkError(err)
}

// Delete all roles that were created by Starbot since the bot started running
func deleteroles(s *discordgo.Session, m *discordgo.MessageCreate, batchName string) {
	_, err := s.ChannelMessageSend(m.ChannelID, FIX_MSG_START+"+ /deleteroles DELETING ROLES\n"+FIX_MSG_END)
	checkError(err)

	//delete each role in the provided batch
	for _, b := range mapBatchesOfCreatedRoles[batchName] {
		cordMessage := fmt.Sprintf("> Deleting %s <@%s>\n", b.Name, b.Discord_id)
		_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
		checkError(err)
		err = s.GuildRoleDelete(m.GuildID, b.Discord_id)
		checkError(err)
	}

	// Cleanup and finish
	delete(mapBatchesOfCreatedRoles, batchName)
	reset_dangerous_commands_status()
	store_data(mapBatchesOfCreatedRoles, "mapBatchesOfCreatedRoles")

	_, err = s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"+ /deleteroles DONE"+DIFF_MSG_END)
	checkError(err)
}

// Builds a map of the desired user state (discord roles) from google sheets
// see: https://developers.google.com/sheets/api/guides/concepts
//func get_sheet_state(players map[string]user_t, disRoles_m map[string]*discordgo.Role) map[string]user_t {
// Check google sheet and assign roles automatically (create new team roles as needed)
func update_roles(dg *discordgo.Session, m *discordgo.MessageCreate) {
	// 0. Get all the roles from the discord and make a map
	discordRoles, err := dg.GuildRoles(DISCORD_SERVER_ID)
	checkError(err)
	roles_m := make(map[string]*discordgo.Role)
	for _, b := range discordRoles {
		roles_m[b.Name] = b
	}

	// 1. Get all the users in the discord
	discordUsers, err := dg.GuildMembers(DISCORD_SERVER_ID, "", 1000)
	checkError(err)

	// 2. Create map of username#discriminator to discord_id
	// and also a map of discord_id -> bool to check if they exist
	//mapDiscordNameToCordID := make(map[string]string)
	//mapDiscordIdExists := make(map[string]bool)
	for _, u := range discordUsers {
		mapDiscordNameToCordID[u.User.String()] = u.User.ID
		mapDiscordIdExists[u.User.ID] = true
	}
	// used to check if a role by name already exists: if mapExistingDiscordRoles[rolename] {...}
	for _, b := range discordRoles {
		mapExistingDiscordRoles[b.Name] = true
	}

	/* Get sheet state
	#### */
	// 3. Get desired state of roles from google sheets

	// Read the secret file (google api key to access google sheets)
	data, err := ioutil.ReadFile("./keys/secret.json")
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
	discord_nameResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_discord_names).Do()
	checkError(err)

	// Read race cells from spreadsheet
	target_ingame_race := "Player List" + "!E1:E"
	ingameRaceResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_ingame_race).Do()
	checkError(err)

	// Read group cells from spreadsheet
	target_group := "Player List" + "!C1:C"
	groupResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_group).Do()
	checkError(err)

	//Read player tier from.. teams sheet
	//target_group := "Player List" + "!C1:C"
	//groupResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_group).Do()
	//checkError(err)

	// sheetPlayers[ign]user_t
	sheetPlayers := make(map[string]user_t)

	//Extract the screen names of the players
	//for _, row := range resp.Values {
	// Loop over the data and add it to the players map
	var ug int
	for i := 0; i < len(screenNameResp.Values); i++ {
		//Extract the screen name
		a := fmt.Sprint(screenNameResp.Values[i])
		a = strings.TrimPrefix(a, "[")
		a = strings.TrimSuffix(a, "]")

		//Extract the discord name
		b := fmt.Sprint(discord_nameResp.Values[i])
		b = strings.TrimPrefix(b, "[")
		b = strings.TrimSuffix(b, "]")

		//Extract ingame race
		c := fmt.Sprint(ingameRaceResp.Values[i])
		c = strings.TrimPrefix(c, "[")
		c = strings.TrimSuffix(c, "]")

		//Extract group (coach/player/etc)
		d := fmt.Sprint(groupResp.Values[i])
		d = strings.TrimPrefix(d, "[")
		d = strings.TrimSuffix(d, "]")

		switch d {
		case "Player":
			ug = PLAYER
		case "Coach":
			ug = COACHES
		case "Assistant Coach":
			ug = ASSISTANTCOACH
		default:
			ug = PLAYER_AND_ASSITANT_COACH
		}

		//add player data to the map
		sheetPlayers[a] = user_t{
			Discord_name: b,
			Race:         c,
			Group:        ug,
		}
	}

	/// ###########

	// 1. Get teams sheet data without crashing on empty cells etc
	targetTeams := "Teams" + "!A1:Z" // defines the sheet and range to be read
	resp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, targetTeams).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	// 1. Let's make a list of the teams
	teams_a := resp.Values[0]

	// [teamname].struct that holds various info such as the discord ID, the players
	sheetTeams := make(map[string]team_t)

	sheetsTeamList := make([]string, 0)
	// Extract the team names and put into the list
	for _, b := range teams_a {
		x := fmt.Sprint(b)
		if len(x) == 0 { //skip empty cells
			continue
		} else {
			sheetsTeamList = append(sheetsTeamList, x)
			sheetTeams[x] = team_t{
				Exists: true,
				Name:   x,
			}
		}
	}

	// 2. Let's associate the users with their team

	isFirstRow := true
	var GROUP int = -99
	for _, collum := range resp.Values {
		if isFirstRow { //skip the first row because it contains teamnames
			isFirstRow = false
			continue
		}

		var teamIndex int
		for x, y := range collum {
			screenName := fmt.Sprint(y)
			if x == 0 {
				teamIndex = 0
			} else {
				teamIndex = x / 2
			}
			switch screenName { //check which block we're on (coach or tier0 etc)
			case "Staff":
				GROUP = STAFF
				continue
			case "Coaches":
				GROUP = COACHES
				continue
			case "Tier 0":
				GROUP = TIER0
				continue
			case "Tier 1":
				GROUP = TIER1
				continue
			case "Tier 2":
				GROUP = TIER2
				continue
			case "Tier 3":
				GROUP = TIER3
				continue
			}
			if NOT_PLAYER_NAME[screenName] { // skip entry if not player
				continue
			}

			if entry, ok := sheetPlayers[screenName]; ok {
				switch GROUP {
				case STAFF:
					entry.Team = sheetsTeamList[teamIndex]
					entry.Group = STAFF
					sheetPlayers[screenName] = entry
				case COACHES:
					entry.Team = sheetsTeamList[teamIndex]
					entry.Group = COACHES
					sheetPlayers[screenName] = entry
				case TIER0:
					entry.Team = sheetsTeamList[teamIndex]
					entry.Tier = TIER0
					sheetPlayers[screenName] = entry
				case TIER1:
					entry.Team = sheetsTeamList[teamIndex]
					entry.Tier = TIER1
					sheetPlayers[screenName] = entry
				case TIER2:
					entry.Team = sheetsTeamList[teamIndex]
					entry.Tier = TIER2
					sheetPlayers[screenName] = entry
				case TIER3:
					entry.Team = sheetsTeamList[teamIndex]
					entry.Tier = TIER3
					sheetPlayers[screenName] = entry
				}
			}
		}

	}
	checkError(err)

	// 4. Check if the user from sheet have desired roles assigned
	// and if not -> assign it!

	for screen_name, _ := range sheetPlayers {

		//get the discordname and the discorduser id for the player we're on in the loop
		cordName := sheetPlayers[screen_name].Discord_name
		cordUserid := mapDiscordNameToCordID[cordName]

		// Check if the user even exists on the server
		if !mapDiscordIdExists[cordUserid] {
			continue // skip if the user doesn't exist
		}

		// Assign Coach/Assistant Coach/ Player
		group_name := ""
		didAssignGroupRole := false // set to true if we assigned Coach, Assistant Coach, or Player role.
		wishGroup := sheetPlayers[screen_name].Group
		switch wishGroup {
		case PLAYER:
			group_name = "Player"
			// don't try to assign player role since we don't have it anymore
			//err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, PLAYER_ROLE_ID)
			//checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		case COACHES:
			group_name = "Coach"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			//err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, PLAYER_ROLE_ID) //no longer needed
			//checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		case ASSISTANTCOACH:
			group_name = "Assistant Coach"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			//err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, PLAYER_ROLE_ID) //no longer needed
			//checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		}

		if didAssignGroupRole {
			cordMessage1 := fmt.Sprintf("> Assigned <@%s> %s to %s\n", cordUserid, screen_name, group_name)
			_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage1)
			checkError(err)
			didAssignGroupRole = false
		}

		// Assign Zerg/Terran/Protoss
		race_name := ""
		didAssignRaceRole := false // set to true if we assigned Zerg, Terran, or Protoss role.
		wishRace := sheetPlayers[screen_name].Race
		switch wishRace {
		case "Zerg":
			race_name = "Zerg"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, ZERG_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			didAssignRaceRole = true

		case "Terran":
			race_name = "Terran"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, ZERG_ROLE_ID)
			checkError(err)
			didAssignRaceRole = true

		case "Protoss":
			race_name = "Protoss"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, ZERG_ROLE_ID)
			checkError(err)
			didAssignRaceRole = true

		}
		if didAssignRaceRole {
			cordMessage2 := fmt.Sprintf("> Assigned <@%s> %s to %s\n", cordUserid, screen_name, race_name)
			_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage2)
			checkError(err)
			didAssignRaceRole = false
		}

		// Assign correct tier
		tier_name := "Tier 0"
		didAssignTier := false
		wishTier := sheetPlayers[screen_name].Tier
		switch wishTier {
		case TIER0:
			tier_name = "Tier 0"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, TIER0_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TIER1_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TIER3_ROLE_ID)
			checkError(err)
			didAssignTier = true
		case TIER1:
			tier_name = "Tier 1"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, TIER1_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TIER0_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TIER3_ROLE_ID)
			checkError(err)
			didAssignTier = true
		case TIER2:
			tier_name = "Tier 2"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, TIER2_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TIER1_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TIER3_ROLE_ID)
			checkError(err)
			didAssignTier = true
		case TIER3:
			tier_name = "Tier 3"
			err := dg.GuildMemberRoleAdd(DISCORD_SERVER_ID, cordUserid, TIER3_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TIER1_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(DISCORD_SERVER_ID, cordUserid, TIER2_ROLE_ID)
			checkError(err)
			didAssignTier = true
		}
		if didAssignTier {
			cordMessage2 := fmt.Sprintf("> Assigned <@%s> %s to %s\n", cordUserid, screen_name, tier_name)
			_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage2)
			checkError(err)
			didAssignRaceRole = false
		}
	}

	//FIXME: If we manually delete a role that was auto created by Starbot and then try to run more commands
	// suc as deleteroles, it crashes and it seems to happen here!
	// my guess is that it has to do with reading something from one of the global data structures and using that
	// with a call to a func from discordgo and that breaks since the entry is no langer valid due to the manual edit
	created_new_role := false
	for _, n := range sheetsTeamList {
		if mapExistingDiscordRoles[n] {
			continue
		} else {
			if !created_new_role { // Creat a new batch for the teams we are about to create
				var newTeams []team_t
				NEW_BATCH_NAME = get_batch_name(mapBatchesOfCreatedRoles)
				mapBatchesOfCreatedRoles[NEW_BATCH_NAME] = newTeams
				created_new_role = true
			}
			entry := mapBatchesOfCreatedRoles[NEW_BATCH_NAME]
			var nTeam team_t
			// create the role
			new_role, err := dg.GuildRoleCreate(m.GuildID)
			checkError(err)
			nTeam.Discord_id = new_role.ID
			nTeam.Name = n
			nTeam.Exists = true

			// name the role correctly
			new_role, err = dg.GuildRoleEdit(m.GuildID, new_role.ID, n, NEON_GREEN, false, 0, true)
			checkError(err)
			entry = append(entry, nTeam)

			//update the map of roles
			roles_m[n] = new_role
			mapBatchesOfCreatedRoles[NEW_BATCH_NAME] = entry // save the update to the map

			cordMessage3 := fmt.Sprintf("> Created %s\n", new_role.Mention())
			_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage3)
		}
	}
	// Persist the newly created roles on disc
	// TO DO
	if created_new_role {
		fmt.Println("TODO: persist data on disk")
		store_data(mapBatchesOfCreatedRoles, "mapBatchesOfCreatedRoles")
		//load_data(&mapBatchesOfCreatedRoles, "mapBatchesOfCreatedRoles")
	}

	// Copy user info into the sheetTeams map
	for _, usr := range sheetPlayers {
		team := usr.Team
		if len(team) == 0 {
			continue //skip ahead if no team was found
		}
		// write the data to the sheetTeams map so we can use it in the next code block
		entry := sheetTeams[team]

		//make sure in the updated entry we have the userid, we need it in thext next loop
		usr.Discord_id = mapDiscordNameToCordID[usr.Discord_name]
		entry.Members = append(entry.Members, usr)

		sheetTeams[team] = entry //write the entry
	}

	// iterate over all teams and assign the teamrole to the members
	for _, t := range sheetTeams { //for each team in the spreadsheet
		for _, usr := range t.Members { //for each user in the current team

			if usr.exists() == false { // Skip to the next user if the user is not on the server
				_, err = dg.ChannelMessageSend(m.ChannelID, "> "+usr.Discord_name+" not found on the server")
				fmt.Println("ERROR:", usr.Discord_name, "not found on the server.")
				continue
			}

			id := usr.Discord_id
			team := usr.Team
			team_id := roles_m[team]

			err = nil
			err = dg.GuildMemberRoleAdd(m.GuildID, id, team_id.ID)
			checkError(err)
			if err == nil { //if we did actually assign the role, save that we did that so we can unassign it later
				// add the role to current member
				var assignment [2]string
				assignment[0] = id
				assignment[1] = team_id.ID
				newlyAssignedRoles = append(newlyAssignedRoles, assignment)
				// send discord message about the role assignment
				cordUser, err := dg.GuildMember(DISCORD_SERVER_ID, id)
				checkError(err)
				cordMessage := fmt.Sprintf("> Assigned %s to %s\n", cordUser.Mention(), roles_m[team].Mention())
				_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage)
			}
		}

	}
}

// Helper that returns true if the user is found on the discord server
func (user user_t) exists() bool {
	discordid := mapDiscordNameToCordID[user.Discord_name]
	return mapDiscordIdExists[discordid]
}

/* Todo: I haven't decided what kind of maps I want to store that hold the information required
for this lookup yet. What separation should there be between the globally availables states of:
- discord server
- google sheets
- webapp
And how do I want to store this on disc?
*/
// Helper that returns the team the user belongs to
func (u user_t) get_team() team_t {
	teamname := u.Team
	// Todo: Lookup the team somewhere
	var team team_t
	team.Name = teamname

	return team
}

// Returns correct batchnumber as string for the new batch
func get_batch_name(m map[string][]team_t) string {

	for i := 0; i <= len(m); i++ {
		num := strconv.Itoa(i)
		if _, exists := m[num]; !exists {
			return num
		}
	}
	return "-1" //this should never happen
}

func reset_dangerous_commands_status() {
	dangerousCommands.isInUse = false
	dangerousCommands.cmdName = ""
	dangerousCommands.AuthorID = ""
}

// Returns true if the user input can be mapped to a batch of auto created roles from mapBatchesOfCreatedRoles
func deleteroles_check_input(userMessage string) bool {
	_, err := strconv.Atoi(userMessage)
	if err != nil { //Looks like it isn't a number
		reset_dangerous_commands_status()
		return false
	}
	//check if the input is within the bounds of existing batches
	if _, exists := mapBatchesOfCreatedRoles[userMessage]; exists {
		reset_dangerous_commands_status()
		return true
	}
	return false
}

func parse_match_result(user_input string, sess *discordgo.Session, m *discordgo.MessageCreate) string {
	var message string //this will be returned and sent to discord every time a users posts a report
	var error_message string
	s := user_input
	//Format: "G27: player_one 1-0 player_two",
	//Check that the string contains ": "
	//So we don't crash later
	if strings.Contains(s, ": ") { //Check that it has the format of: "G42: bla."
		s1 := strings.Split(s, ": ")
		//fmt.Println("GROUP:", s1[0], "RESULT:", s1[1]) //debug
		//fmt.Println(s1[0])
		//fmt.Println(group, "---", s2)
		if s1[0][0] == 'G' { //Extract the group
			group := s1[0][1:] //this is the group

			//check that group is a number
			if _, err := strconv.ParseInt(group, 10, 64); err != nil {
				error_message += "```diff\n- REJECTED: Group is not number\n\nYour input:\n" + s + "\n\nCorrect format:\n" + MATCH_REPORT_FORMAT_HELP_TEXT
				log_match_accepted(s, false) //log the match in logfile and print to stdout
				sess.ChannelMessageDelete(MATCH_REPORTING_CHANNEL_ID, m.ID)
				return error_message
			}

			s2 := s1[1][0:] //this is the rest of the line
			dashes_count := strings.Count(s2, "-")
			if dashes_count > 1 { //check if there is more than 1 dash, (some usernames have dashes)
				error_message += "```diff\n- REJECTED: Many dashes\n\nYour input:\n" + s + "\n\nCorrect format:\n" + MATCH_REPORT_FORMAT_HELP_TEXT
				log_match_accepted(s, false) //log the match in logfile and print to stdout
				sess.ChannelMessageDelete(MATCH_REPORTING_CHANNEL_ID, m.ID)
				return error_message
			}
			spaces_count := strings.Count(s2, " ")
			if spaces_count > 2 {
				error_message += "```diff\n- REJECTED: Many spaces\n\nYour input:\n" + s + "\n\nCorrect format:\n" + MATCH_REPORT_FORMAT_HELP_TEXT
				log_match_accepted(s, false) //log the match in logfile and print to stdout
				sess.ChannelMessageDelete(MATCH_REPORTING_CHANNEL_ID, m.ID)
				return error_message
			}
			if strings.Contains(s2, "-") { //check that there is a - in the middle? indicating "player_one 5-2 player_two"
				//TODO: check that there aren't multiple dashes in s2
				s3 := strings.Split(s2, "-") // Split in the middle, multiple dashes = problem
				//fmt.Println("Group:", group) //REPORT
				//fmt.Println(s3) //debug
				p1 := s3[0] //this contains player_one name and score
				p2 := s3[1] //this contains player_two name and score
				player_segment := strings.Split(p1, " ")
				if len(player_segment) > 1 { //check and extract the player segments
					//extract player 1 name and score:
					player_one_name := player_segment[0]
					//Trim and sanitize the player1 remaining string to extract the score
					p1s := player_segment[1]
					p1s_no_spaces := strings.ReplaceAll(p1s, " ", "")
					player_one_score := string(p1s_no_spaces[len(p1s_no_spaces)-1]) //last character is player_one's score
					p2s_no_spaces := strings.ReplaceAll(p2, " ", "")
					player_two_score := string(p2s_no_spaces[0]) //first character here should be player_two's score
					player_two_name := p2s_no_spaces[1:]

					//get numerical score
					p1s_i, err := strconv.Atoi(player_one_score)
					p2s_i, err := strconv.Atoi(player_two_score)
					if err != nil {
						error_message += "```diff\n- REJECTED: Formatting error\n\nYour input:\n" + s + "\n\nCorrect format:\n" + MATCH_REPORT_FORMAT_HELP_TEXT
						fmt.Println(err)
						log_match_accepted(s, false) //log the match in logfile and print to stdout
						sess.ChannelMessageDelete(MATCH_REPORTING_CHANNEL_ID, m.ID)
						return error_message
					}

					// send discord messagge and log as accepted
					message := "GROUP **" + group + "**.)"
					if p2s_i < p1s_i {
						message += "\n" + player_one_name + "(" + player_one_score + ") WINNER\n"
						message += player_two_name + "(" + player_two_score + ") LOSER\n"
						message += MATCH_ACCEPTED
						log_match_accepted(s, true) //log the match in logfile and print to stdout
						return message
					} else if p2s_i > p1s_i {
						message += "\n" + player_two_name + "(" + player_two_score + ") WINNER\n"
						message += player_one_name + "(" + player_one_score + ") LOSER\n"
						message += MATCH_ACCEPTED
						log_match_accepted(s, true) //log the match in logfile and print to stdout
						return message
					} else {
						message += "\n" + player_two_name + "(" + player_two_score + ") TIE\n"
						message += "\n" + player_one_name + "(" + player_one_score + ") TIE\n"
						message += MATCH_ACCEPTED
						log_match_accepted(s, true) //log the match in logfile and print to stdout
						return message
					}
				}

			} else { // send error message and log as rejected
				error_message += "```diff\n- REJECTED: Formatting error\n\nYour input:\n" + s + "\n\nCorrect format:\n" + MATCH_REPORT_FORMAT_HELP_TEXT
				log_match_accepted(s, false) //log the match in logfile and print to stdout
				sess.ChannelMessageDelete(MATCH_REPORTING_CHANNEL_ID, m.ID)
				return error_message
			}
		}
	}
	error_message += "```diff\n- REJECTED: Unexpected formatting error\n\nYour input:\n" + s + "\n\nCorrect format:\n" + MATCH_REPORT_FORMAT_HELP_TEXT
	message += error_message
	log_match_accepted(s, false) //log the match in logfile and print to stdout
	sess.ChannelMessageDelete(MATCH_REPORTING_CHANNEL_ID, m.ID)
	return message
}

// Log everything
func log_message(s string) {
	log.Println("[log-all]       " + s + "<br>\n")
}

// Log match to index.html and stdout
// call with True to log accepted, and False to log rejected
func log_match_accepted(s string, accepted bool) {
	if accepted {
		log.Println("[ACCEPTED] " + s + "<br>\n")
		fmt.Println("[ACCEPTED] " + s)
	} else {
		log.Println("[REJECTED] " + s + "<br>\n")
		fmt.Println("[REJECTED] " + s)
	}
}

// Load persistent data into memory
func load_persistent_internal_data_structures() {
	/*
		var filenames = [7]string{"discordUsers",
			"mapWebUserNameToWebUserId",
			"mapDiscordNameToCordID",
			"mapDiscordIdExists",
			"mapWebUserIdToPlayer", //this is the main important one
			"mapBatchesOfCreatedRoles"}

			for _, name := range filenames {
				if _, err := os.Stat("./data/" + name); err == nil {
					load_data(&name, name)
					fmt.Println("LOADING", name)
				} else {
					checkError(err) // file may or may not exist. See err for details.
				}
			}
	*/
	load_data(&discordUsers, "discordUsers")
	load_data(&mapWebUserNameToWebUserId, "mapWebUserNameToWebUserId")
	load_data(&mapWebUserIdToPlayer, "mapWebUserIdToPlayer")
	load_data(&mapDiscordNameToCordID, "mapDiscordNameToCordId")
	load_data(&mapDiscordIdExists, "mapDiscordIdExists")
	load_data(&mapBatchesOfCreatedRoles, "mapBatchesOfCreatedRoles")
}

// persist data structures on disc in ./data (data folder must be present in directory)
func store_data(data interface{}, filename string) {
	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)
	err := encoder.Encode(data)
	checkError(err)
	err = ioutil.WriteFile("./data/"+filename, buffer.Bytes(), 0600)
	checkError(err)
}

// load data that was stored on disc in ./data (data folder must be present in directory)
func load_data(data interface{}, filename string) {
	raw, err := ioutil.ReadFile("./data/" + filename)
	checkError(err)
	buffer := bytes.NewBuffer(raw)
	dec := gob.NewDecoder(buffer)
	err = dec.Decode(data)
	checkError(err)
}

// Get unique discord IDs for all players on web and save them -> output if we can't find players
func scan_web_players(s *discordgo.Session, m *discordgo.MessageCreate) {
	var found int
	var missing int
	var misspelled int
	// 1. Get all the users in the discord
	discordUsers1, err := s.GuildMembers(DISCORD_SERVER_ID, "", 1000)
	checkError(err)

	discordUsers = append(discordUsers, discordUsers1...)
	// TODO: we should check to make sure we automatically always send the right requests
	// Since the server has more than 1k members we have to request 2 batches of 1000 each
	if len(discordUsers) > 999 { //only request another batch if there are a lot of users
		lasti := len(discordUsers) - 1 //index of last mmember in slice
		discordUsers2, err := s.GuildMembers(DISCORD_SERVER_ID, discordUsers[lasti].User.ID, 1000)
		checkError(err)
		//combine both into mega slice
		discordUsers = append(discordUsers, discordUsers2...)
	}

	// 2. Create map of username#discriminator to discord_id
	for _, u := range discordUsers {
		mapDiscordNameToCordID[u.User.String()] = u.User.ID
		mapDiscordIdExists[u.User.ID] = true
	}

	// ioutil deprecated but still works (io wrappers)
	data_from_players_file, err := ioutil.ReadFile("./data/players.json")
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	// put json data into slice of players
	playersUnmarsh := make([]web_player_t, len(data_from_players_file))
	err = json.Unmarshal(data_from_players_file, &playersUnmarsh)
	if err != nil {
		fmt.Println("error: ", err)

	}

	// create map of WebUserId -> web_player_t
	for _, b := range playersUnmarsh {
		mapWebUserIdToPlayer[b.WebUserId] = b
	}
	// create map of playername to webID
	for _, b := range playersUnmarsh {
		mapWebUserNameToWebUserId[b.WebName] = b.WebUserId
	}

	// Find immuatable discord snowflake ID of all players from players.json and save to internal data structures
	var alternateName string
	for webId, player := range mapWebUserIdToPlayer {
		id := mapDiscordNameToCordID[player.DiscordName]
		if mapDiscordIdExists[id] { //store the id
			found++
			player.Discord_id = id
			mapWebUserIdToPlayer[webId] = player //write the new data to the map
			//_, err := s.ChannelMessageSend(m.ChannelID, "> Found user: "+player.DiscordName+" with snowflake id:"+id)
			checkError(err)
		} else {
			//Check forcommon capitalization mistake on first letter
			if unicode.IsLower(rune(player.DiscordName[0])) {
				alternateName = strings.ToUpper(string(player.DiscordName[0])) + player.DiscordName[1:]
			} else if unicode.IsUpper(rune(player.DiscordName[0])) {
				alternateName = strings.ToLower(string(player.DiscordName[0])) + player.DiscordName[1:]
			}
			id := mapDiscordNameToCordID[alternateName]
			if mapDiscordIdExists[id] { //store the id
				found++
				player.Discord_id = id
				mapWebUserIdToPlayer[webId] = player //write the new data to the map
				misspelled++
				_, err := s.ChannelMessageSend(m.ChannelID, "> Found misspelled user: "+player.DiscordName+" with snowflake id:"+id)
				checkError(err)
			} else {
				missing++
				fmt.Println("Missing user:", player.DiscordName)
				_, err := s.ChannelMessageSend(m.ChannelID, "[ERROR] cant find user: "+player.DiscordName)
				checkError(err)
			}
		}
		reset_dangerous_commands_status()
	}

	// find Dada
	//dada_id := mapWebUserNameToWebUserId["dada78641"]
	//dada := mapWebUserIdToPlayer[dada_id]
	//fmt.Println(dada)
	message := DIFF_MSG_START
	message += "+ /scan_missing USER SCAN COMPLETE\n"
	message += fmt.Sprintf("**Found:** %d\n**Found Typo'd user:** %d\n**Missing:** %d", found, misspelled, missing)
	message += DIFF_MSG_END
	_, err = s.ChannelMessageSend(m.ChannelID, message)
	checkError(err)

	// store updated maps and discordusers
	store_data(discordUsers, "discordUsers")
	store_data(mapWebUserNameToWebUserId, "mapWebUserNameToWebUserId")
	store_data(mapWebUserNameToWebUserId, "mapWebUserNameToWebUserId")
	store_data(mapDiscordNameToCordID, "mapDiscordNameToCordID")
	store_data(mapDiscordIdExists, "mapDiscordIdExists")
	store_data(mapWebUserIdToPlayer, "mapWebUserIdToPlayer")
}

// Parse past messages from channel this func is called
func parse_past_messages(s *discordgo.Session, m *discordgo.MessageCreate) {
	messagesFromChannel, err := s.ChannelMessages(m.ChannelID, 100, "", "", "")
	checkError(err)

	for _, message := range messagesFromChannel {
		if strings.Contains(message.Content, "twitch.tv") {
			log.Println("[CPL-CLIPS] " + message.Content + " <br>")
		}
	}

}

// Log messages from the clips channel that contain "twitch.tv"
func parse_message_in_clips_channel(s *discordgo.Session, m *discordgo.MessageCreate) {
	if strings.Contains(m.Content, "twitch.tv") {
		log.Println("[CPL-CLIPS] " + m.Content + " <br>")
	}
}

// Assigns/creates roles based on entry on web
func assign_roles_from_json(s *discordgo.Session, m *discordgo.MessageCreate) {
	dangerousCommands.isInUse = true
	dangerousCommands.cmdName = "/assign_roles_from_json"
	/* key = meaning */
	const PROTOSS int = 6
	const ZERG int = 7
	const TERRAN int = 8
	const DECLARED_WEEKLY int = 9
	const RACE_PICKER int = 10
	const TEAM_ADMIN int = 7
	const CASTER int = 8
	const LIQUIPEDIA_UPDATES int = 9
	const GRAPHICS int = 10
	const DEV int = 11
	const OTHER int = 12
	//helper roles
	const PLAYER int = 4
	const COACH int = 5
	const ASSISTANT_COACH int = 6

	/*
			assign:
		Team
		Tier
		Race"
	*/

	//stats, count every time a role was assigned
	var team1Count int
	var team2Count int
	var team3Count int
	var team4Count int
	var team5Count int
	var team6Count int
	var tier0Count int
	var tier1Count int
	var tier2Count int
	var tier3Count int
	var raceCount int
	var coachCount int
	var assisCoachCount int
	var totalRoleAssignments int

	for _, usr := range mapWebUserIdToPlayer {

		switch usr.Team {
		case "Team 1":
			fmt.Println("assign", usr.DiscordName, "to Team 1")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TEAM1_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			team1Count++

			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "Team 1")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				fmt.Println(cordMessage)
				checkError(err)
			}

		case "Team 2":
			fmt.Println("assign", usr.DiscordName, "to Team 2")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TEAM2_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			team2Count++

			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "Team 2")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				fmt.Println(cordMessage)
				checkError(err)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign team to"+usr.WebName+usr.DiscordName)
				checkError(err)

			}

		case "Team 3":
			fmt.Println("assign", usr.DiscordName, "to Team 3")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TEAM3_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			team3Count++

			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "Team 3")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				checkError(err)
				fmt.Println(cordMessage)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign team to"+usr.WebName+usr.DiscordName)
				checkError(err)
			}

		case "Team 4":
			fmt.Println("assign", usr.DiscordName, "to Team 4")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TEAM4_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			team4Count++

			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "Team 4")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				fmt.Println(cordMessage)
				checkError(err)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign team to"+usr.WebName+usr.DiscordName)
				checkError(err)
			}

		case "Team 5":
			fmt.Println("assign", usr.DiscordName, "to Team 5")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TEAM5_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			team5Count++

			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "Team 5")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				checkError(err)
				fmt.Println(cordMessage)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign team to"+usr.WebName+usr.DiscordName)
				checkError(err)
			}

		case "Team 6":
			fmt.Println("assign", usr.DiscordName, "to Team 6")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TEAM6_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			team6Count++

			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "Team 6")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				//checkError(err)
				fmt.Println(cordMessage)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign team to"+usr.WebName+usr.DiscordName)
				checkError(err)
			}

		default:
			fmt.Println("error", usr.DiscordName, "- no team found")

		}

		switch usr.Race { // Assign Race
		case PROTOSS:
			fmt.Println("assign", usr.DiscordName, "to Protoss")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, PROTOSS_ROLE_ID)
			checkError(err)
			err = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TERRAN_ROLE_ID)
			checkError(err)
			err = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, ZERG_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			raceCount++

		case ZERG:
			fmt.Println("assign", usr.DiscordName, "to Zerg")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, ZERG_ROLE_ID)
			checkError(err)
			err = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TERRAN_ROLE_ID)
			checkError(err)
			err = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, PROTOSS_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			raceCount++

		case TERRAN:
			fmt.Println("assign", usr.DiscordName, "to Terran")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TERRAN_ROLE_ID)
			checkError(err)
			err = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, ZERG_ROLE_ID)
			checkError(err)
			err = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, PROTOSS_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			raceCount++

		case DECLARED_WEEKLY:
			// idk what to do here, maybe nothing?

		case RACE_PICKER:

		default:
			fmt.Println("error", usr.DiscordName, "- no race found")
		}

		switch usr.Tier { // Assign Tier
		case 999:
			fmt.Println("removed all tiers from", usr.DiscordName)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER0_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER1_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER2_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER3_ROLE_ID)
			cordMessage := fmt.Sprintf("> Removed all tiers from <@%s> %s\n", usr.Discord_id, usr.WebName)
			_, err := s.ChannelMessageSend(m.ChannelID, cordMessage)
			checkError(err)

		case 0:
			fmt.Println("assign", usr.DiscordName, "to Tier0")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TIER0_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER1_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER2_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER3_ROLE_ID)
			totalRoleAssignments++
			tier0Count++
			checkError(err)
			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "TIER 0")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				//checkError(err)
				fmt.Println(cordMessage)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign tier to"+usr.WebName+usr.DiscordName)
				checkError(err)
			}

		case 1:
			fmt.Println("assign", usr.DiscordName, "to Tier1")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TIER1_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER0_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER2_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER3_ROLE_ID)
			totalRoleAssignments++
			tier1Count++
			checkError(err)
			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "TIER 1")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				//checkError(err)
				fmt.Println(cordMessage)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign tier to"+usr.WebName+usr.DiscordName)
				checkError(err)
			}

		case 2:
			fmt.Println("assign", usr.DiscordName, "to Tier2")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TIER2_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER0_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER1_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER3_ROLE_ID)
			totalRoleAssignments++
			tier2Count++
			checkError(err)
			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "TIER 2")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				//checkError(err)
				fmt.Println(cordMessage)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign tier to"+usr.WebName+usr.DiscordName)
				checkError(err)
			}

		case 3:
			fmt.Println("assign", usr.DiscordName, "to Tier3")
			err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, TIER3_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER2_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER1_ROLE_ID)
			_ = s.GuildMemberRoleRemove(DISCORD_SERVER_ID, usr.Discord_id, TIER0_ROLE_ID)
			checkError(err)
			totalRoleAssignments++
			tier3Count++
			if err == nil {
				cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "TIER 3")
				//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
				//checkError(err)
				fmt.Println(cordMessage)
			} else {
				_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign tier to"+usr.WebName+usr.DiscordName)
				checkError(err)
			}

		default:
			fmt.Println("error", usr.DiscordName, "- no Tier found")
		}

		for _, role := range usr.Helper_role {
			switch role { // Assign Coach/Assistant Coach/Player
			case PLAYER:
				fmt.Println("assign", usr.DiscordName, "to Player")
			case COACH:
				fmt.Println("assign", usr.DiscordName, "to Coach")
				totalRoleAssignments++
				coachCount++
				err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, COACH_ROLE_ID)
				checkError(err)
				if err == nil {
					cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "COACH")
					//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
					//checkError(err)
					fmt.Println(cordMessage)
				} else {
					_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign COACH to"+usr.WebName+usr.DiscordName)
					checkError(err)
				}

			case ASSISTANT_COACH:
				fmt.Println("assign", usr.DiscordName, "to Assistant Coach")
				err := s.GuildMemberRoleAdd(DISCORD_SERVER_ID, usr.Discord_id, ASST_COACH_ROLE_ID)
				totalRoleAssignments++
				assisCoachCount++
				checkError(err)
				if err == nil {
					cordMessage := fmt.Sprintf("> Assigned <@%s> %s to %s\n", usr.Discord_id, usr.WebName, "ASSISTANT COACH")
					//_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
					//checkError(err)
					fmt.Println(cordMessage)
				} else {
					_, err = s.ChannelMessageSend(m.ChannelID, "Couldn't assign ASSISTANT COACH to"+usr.WebName+usr.DiscordName)
					checkError(err)
				}
			default:
				fmt.Println("error", usr.DiscordName, "- no helper role ")
			}
		}
	}
	message := DIFF_MSG_START
	message += "+ /assignroles ROLE ASSIGNMENT COMPLETE\n"
	totalUsrInTeams := team1Count + team2Count + team3Count + team4Count + team5Count + team6Count
	message += fmt.Sprintf("**Users assigned to teams:** %d\n**Team 1:** %d\n**Team 2:** %d\n**Team 3:** %d\n**Team 4:** %d\n**Team 5:** %d\n**Team 6:** %d\n", totalUsrInTeams, team1Count, team2Count, team3Count, team4Count, team5Count, team6Count)
	message += fmt.Sprintf("**Coaches assigned:** %d\n**Assistant Coaches assigned:** %d\n\n**Total roles assigned**: %d\n", coachCount, assisCoachCount, totalRoleAssignments)
	message += DIFF_MSG_END
	_, err := s.ChannelMessageSend(m.ChannelID, message)
	checkError(err)
}

func main() {
	/* Startup procedures:
	#####	*/
	// Check to make sure a bot auth token was supplied on startup
	if len(os.Args) < 2 || len(os.Args) > 2 {
		fmt.Println("Error: You must supply EXACTLY one argument (the bot's authorization token) on startup.")
		os.Exit(1)
	}

	TOKEN = os.Args[1] // discord API Token

	// Open file for match report logging
	logfile, err := os.OpenFile("log.html", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		checkError(err)
	}
	defer logfile.Close() //close file when main exits
	log.SetOutput(logfile)

	// Load persistent data into memory
	load_persistent_internal_data_structures()

	// Initiate discord session through the discord API
	dg, err := discordgo.New("Bot " + TOKEN)
	if err != nil {
		fmt.Println("Error creating discord session", err)
	}

	// Register scan_message as a callback func for message events
	dg.AddHandler(scan_message)

	// Receive all events on the server
	dg.Identify.Intents = discordgo.IntentsAll

	// Establish the discord session
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection", err)
		os.Exit(1)
	}
	//##### End of startup procedures

	/* TESTING WIP:
	##### */
	// .. ..
	//##### End of Testing

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
