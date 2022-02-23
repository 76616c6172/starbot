package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

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
var AUTHORIZED_USERS = map[string]bool{
	"96492516966174720": true, //valar
	"93204976779694080": true, //Pete aka Pusagi
}

/* WE ARE ON THE QA BRANCH:
// HARDCODED IDS link to roles on the Testserver and Test-Sheet */
const SPREADSHEET_ID string = "1K-jV6-CUmjOSPW338MS8gXAYtYNW9qdMeB7XMEiQyn0" // google sheet ID

//const SERVER_ID string = "856762567414382632"                  //TEST SERVER
//const MATCH_REPORTING_CHANNEL_ID string = "945364478973861898" //TEST CHANNEL
const SERVER_ID string = "426172214677602304"                  // CPL SERVER
const MATCH_REPORTING_CHANNEL_ID string = "945736138864349234" // CPL CHANNEL
const ZERG_ROLE_ID string = "941808009984737281"
const TERRAN_ROLE_ID string = "941808071817187389"
const PROTOSS_ROLE_ID string = "941808145993441331"
const TIER0_ROLE_ID string = "942081263358070794"
const TIER1_ROLE_ID string = "942081322325794839"
const TIER2_ROLE_ID string = "942081354353487872"
const TIER3_ROLE_ID string = "942081409500213308"
const COACH_ROLE_ID string = "942083540739317811"
const ASST_COACH_ROLE_ID string = "941808582410764288"
const PLAYER_ROLE_ID string = "942083605667131443"

//const PLAYER_ROLE_ID string = "" // no longer needed

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

// Colors for role creation (decimal values of hex color codes)
const NEON_GREEN int = 2358021

// help cmd text (DONT write longer lines than this, this is the maximum that still looks good on mobile)
const AVAILABLE_COMMANDS string = `
[ /help        - show commands                        ]
[ /test        - test command                         ]
[                                                     ]
[ /assignroles - create and assign roles              ]
[ /deleteroles - delete previously created roles      ]
`

const FORMAT_USAGE string = "`G2: player_name 1-0 player_two`"

//[ /unassignroles   - unassign previous batch of roles     ]

// Discord message formatting strings
const DIFF_MSG_START string = "```diff\n"
const DIFF_MSG_END string = "\n```"
const FIX_MSG_START string = "```fix\n"
const FIX_MSG_END string = "\n```"
const MATCH_ACCEPTED string = "```diff\n+ Message formatting passes the check\n```"

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

//var GROUP int = 0

//##### End of Hardcoded values

/* DATA STRUCTURES
##### */

type user_t struct {
	ign          string //ingame name
	discord_name string
	discord_id   string
	race         string
	tier         int //0, 1, 2, or 3
	group        int //Coach, Assistant Coach, or Player
	team         string
}

// Struct to store information about a team-role
type team_t struct {
	name       string
	discord_id string
	members    []user_t
	color      int
	exists     bool
	//mentionable        bool
	//perms              int64
	//fully_created_role discordgo.Role
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
var NEW_BATCH_NAME string                     // Name of a batch of newly created roles
var newlyCreatedRoles []string                // Holds newly created discord role IDs
var newlyAssignedRoles [][2]string            //[roleid][userid]
var dangerousCommands dangerousCommands_t     // info about /update roles command while being used
var discordNameToID = map[string]string{}     // Used to lookup discordid from discord name
var discordIDExists = map[string]bool{}       // Used to check if the user exists on the server
var discordRoleExsits = map[string]bool{}     // Used to check if the user exists on the server
var batchCreatedRoles = map[string][]team_t{} //[batchName]{roleid, roleid, roleid, roleid}
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

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.ChannelID == MATCH_REPORTING_CHANNEL_ID {
		user_message := m.Content
		if strings.Contains(user_message, ": ") {
			return_message := parse_match_result(user_message)
			_, err := s.ChannelMessageSend(m.ChannelID, return_message)
			checkError(err)
		}
	}

	// Trigger on interactive command:w
	if dangerousCommands.isInUse && m.Author.ID == dangerousCommands.AuthorID && AUTHORIZED_USERS[m.Author.ID] {
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

	// Handle first use or non-interactive commands
	switch m.Content {
	case "/deleteroles":
		if !AUTHORIZED_USERS[m.Author.ID] { // Check for Authorization
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
			select_batch_to_delete(s, m)
		} else {
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /deleteroles ERROR: "+m.Author.Username+" DANGEROUS COMMAND IS IN USE"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
			}
			return
		}

	case "/get_server_id": // Prints the ID of the discord server
		_, err := s.ChannelMessageSend(m.ChannelID, m.GuildID)
		if err != nil {
			fmt.Println(err)
		}

	case "/unassignroles": //not implemented yet
		_, err := s.ChannelMessageSend(m.ChannelID, "/unassignroles is not implemented yet")
		checkError(err)

	case "/assignroles":
		if dangerousCommands.isInUse { //check if this command is in use first and disallow simultanious use
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /assignroles ERROR: EXECUTION IN PROGRESS"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		if AUTHORIZED_USERS[m.Author.ID] { // if the user is authorized, proceed with the operation
			_, err := s.ChannelMessageSend(m.ChannelID, FIX_MSG_START+"+ /assignroles ROLE UPDATE STARTED"+FIX_MSG_END)
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
			_, err = s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"+ /assignroles ROLE UPDATE COMPLETE"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
				return
			}

			return

		} else {
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /assignroles ERROR: "+m.Author.Username+" IS NOT AUTHORIZED"+DIFF_MSG_END)
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
}

// Test function executes with side effects and returns final message to be send
func test(s *discordgo.Session, m *discordgo.MessageCreate) string {
	message := fmt.Sprintln(m.ChannelID)
	message += "<-channel ID"
	return message
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
	var z string
	for a, b := range batchCreatedRoles {
		fmt.Println(a)

		x := fmt.Sprintf(a)
		y := fmt.Sprintln(b)
		z += x + y + "\n"
	}
	_, err := s.ChannelMessageSend(m.ChannelID, FIX_MSG_START+"+ /deleteroles STARTING\n\nPlease enter batchnumber of roles to be deleted from below:\n\n"+z+FIX_MSG_END)
	checkError(err)
}

// Delete all roles that were created by Starbot since the bot started running
func deleteroles(s *discordgo.Session, m *discordgo.MessageCreate, batchName string) {
	_, err := s.ChannelMessageSend(m.ChannelID, FIX_MSG_START+"+ /deleteroles DELETING ROLES\n"+FIX_MSG_END)
	checkError(err)

	//delete each role in the provided batch
	for _, b := range batchCreatedRoles[batchName] {
		cordMessage := fmt.Sprintf("> Deleting %s <@%s>\n", b.name, b.discord_id)
		_, err = s.ChannelMessageSend(m.ChannelID, cordMessage)
		checkError(err)
		err = s.GuildRoleDelete(m.GuildID, b.discord_id)
		checkError(err)
	}

	// Cleanup and finish
	delete(batchCreatedRoles, batchName)
	reset_dangerous_commands_status()

	_, err = s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"+ /deleteroles DONE"+DIFF_MSG_END)
	checkError(err)
}

// Builds a map of the desired user state (discord roles) from google sheets
// see: https://developers.google.com/sheets/api/guides/concepts
//func get_sheet_state(players map[string]user_t, disRoles_m map[string]*discordgo.Role) map[string]user_t {
// Check google sheet and assign roles automatically (create new team roles as needed)
func update_roles(dg *discordgo.Session, m *discordgo.MessageCreate) {
	// 0. Get all the roles from the discord and make a map
	discordRoles, err := dg.GuildRoles(SERVER_ID)
	checkError(err)
	roles_m := make(map[string]*discordgo.Role)
	for _, b := range discordRoles {
		roles_m[b.Name] = b
	}

	// 1. Get all the users in the discord
	discordUsers, err := dg.GuildMembers(SERVER_ID, "", 1000)
	checkError(err)

	// 2. Create map of username#discriminator to discord_id
	// and also a map of discord_id -> bool to check if they exist
	//discordNameToID := make(map[string]string)
	//discordIDExists := make(map[string]bool)
	for _, u := range discordUsers {
		discordNameToID[u.User.String()] = u.User.ID
		discordIDExists[u.User.ID] = true
	}
	// used to check if a role by name already exists: if discordRoleExsits[rolename] {...}
	for _, b := range discordRoles {
		discordRoleExsits[b.Name] = true
	}

	/* Get sheet state
	#### */
	// 3. Get desired state of roles from google sheets

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
			discord_name: b,
			race:         c,
			group:        ug,
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
				exists: true,
				name:   x,
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
					entry.team = sheetsTeamList[teamIndex]
					entry.group = STAFF
					sheetPlayers[screenName] = entry
				case COACHES:
					entry.team = sheetsTeamList[teamIndex]
					entry.group = COACHES
					sheetPlayers[screenName] = entry
				case TIER0:
					entry.team = sheetsTeamList[teamIndex]
					entry.tier = TIER0
					sheetPlayers[screenName] = entry
				case TIER1:
					entry.team = sheetsTeamList[teamIndex]
					entry.tier = TIER1
					sheetPlayers[screenName] = entry
				case TIER2:
					entry.team = sheetsTeamList[teamIndex]
					entry.tier = TIER2
					sheetPlayers[screenName] = entry
				case TIER3:
					entry.team = sheetsTeamList[teamIndex]
					entry.tier = TIER3
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
		cordName := sheetPlayers[screen_name].discord_name
		cordUserid := discordNameToID[cordName]

		// Check if the user even exists on the server
		if !discordIDExists[cordUserid] {
			continue // skip if the user doesn't exist
		}

		// Assign Coach/Assistant Coach/ Player
		group_name := ""
		didAssignGroupRole := false // set to true if we assigned Coach, Assistant Coach, or Player role.
		wishGroup := sheetPlayers[screen_name].group
		switch wishGroup {
		case PLAYER:
			group_name = "Player"
			// don't try to assign player role since we don't have it anymore
			//err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, PLAYER_ROLE_ID)
			//checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		case COACHES:
			group_name = "Coach"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			//err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PLAYER_ROLE_ID) //no longer needed
			//checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		case ASSISTANTCOACH:
			group_name = "Assistant Coach"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			//err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PLAYER_ROLE_ID) //no longer needed
			//checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, COACH_ROLE_ID)
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
		wishRace := sheetPlayers[screen_name].race
		switch wishRace {
		case "Zerg":
			race_name = "Zerg"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, ZERG_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			didAssignRaceRole = true

		case "Terran":
			race_name = "Terran"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ZERG_ROLE_ID)
			checkError(err)
			didAssignRaceRole = true

		case "Protoss":
			race_name = "Protoss"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, PROTOSS_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TERRAN_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ZERG_ROLE_ID)
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
		wishTier := sheetPlayers[screen_name].tier
		switch wishTier {
		case TIER0:
			tier_name = "Tier 0"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, TIER0_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TIER1_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TIER3_ROLE_ID)
			checkError(err)
			didAssignTier = true
		case TIER1:
			tier_name = "Tier 1"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, TIER1_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TIER0_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TIER3_ROLE_ID)
			checkError(err)
			didAssignTier = true
		case TIER2:
			tier_name = "Tier 2"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, TIER2_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TIER1_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TIER3_ROLE_ID)
			checkError(err)
			didAssignTier = true
		case TIER3:
			tier_name = "Tier 3"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, TIER3_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TIER1_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, TIER2_ROLE_ID)
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
		if discordRoleExsits[n] {
			continue
		} else {
			if !created_new_role { // Creat a new batch for the teams we are about to create
				var newTeams []team_t
				NEW_BATCH_NAME = get_batch_name(batchCreatedRoles)
				batchCreatedRoles[NEW_BATCH_NAME] = newTeams
				created_new_role = true
			}
			entry := batchCreatedRoles[NEW_BATCH_NAME]
			var nTeam team_t
			// create the role
			new_role, err := dg.GuildRoleCreate(m.GuildID)
			checkError(err)
			nTeam.discord_id = new_role.ID
			nTeam.name = n
			nTeam.exists = true

			// name the role correctly
			new_role, err = dg.GuildRoleEdit(m.GuildID, new_role.ID, n, NEON_GREEN, false, 0, true)
			checkError(err)
			entry = append(entry, nTeam)

			//update the map of roles
			roles_m[n] = new_role
			batchCreatedRoles[NEW_BATCH_NAME] = entry // save the update to the map

			cordMessage3 := fmt.Sprintf("> Created <%s>\n", new_role.Mention())
			_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage3)
		}
	}

	// Copy user info into the sheetTeams map
	for _, usr := range sheetPlayers {
		team := usr.team
		if len(team) == 0 {
			continue //skip ahead if no team was found
		}
		// write the data to the sheetTeams map so we can use it in the next code block
		entry := sheetTeams[team]

		//make sure in the updated entry we have the userid, we need it in thext next loop
		usr.discord_id = discordNameToID[usr.discord_name]
		entry.members = append(entry.members, usr)

		sheetTeams[team] = entry //write the entry
	}

	// iterate over all teams and assign the teamrole to the members
	for _, t := range sheetTeams { //for each team in the spreadsheet
		for _, usr := range t.members { //for each user in the current team

			if usr.exists() == false { // Skip to the next user if the user is not on the server
				//unfound_members += "ERROR: " + usr.discord_name + "not found on the server\n"
				_, err = dg.ChannelMessageSend(m.ChannelID, "> "+usr.discord_name+" not found on the server")
				fmt.Println("ERROR:", usr.discord_name, "not found on the server.")
				continue
			}

			id := usr.discord_id
			team := usr.team
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
				cordUser, err := dg.GuildMember(SERVER_ID, id)
				checkError(err)
				cordMessage := fmt.Sprintf("> Assigned %s to %s\n", cordUser.Mention(), roles_m[team].Mention())
				_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage)
			}
		}

	}
}

// Helper that returns true if the user is found on the discord server
func (user user_t) exists() bool {
	discordid := discordNameToID[user.discord_name]
	return discordIDExists[discordid]
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
	teamname := u.team
	// Todo: Lookup the team somewhere
	var team team_t
	team.name = teamname

	return team
}

// Returns correct batchnumber as string for the new batch
func get_batch_name(m map[string][]team_t) string {
	a := len(m)
	a++
	return strconv.Itoa(a)
}

func reset_dangerous_commands_status() {
	dangerousCommands.isInUse = false
	dangerousCommands.cmdName = ""
	dangerousCommands.AuthorID = ""
}

// Returns true if the user input can be mapped to a batch of auto created roles from batchCreatedRoles
func deleteroles_check_input(userMessage string) bool {
	num, err := strconv.Atoi(userMessage)
	if err != nil {
		reset_dangerous_commands_status()
		return false
	}
	//check if the input is within the bounds of existing batches
	if !(num <= len(batchCreatedRoles)) {
		reset_dangerous_commands_status()
		return false
	}
	return true
}

func parse_match_result(user_input string) string {
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
			s2 := s1[1][0:]    //this is the rest of the line
			dashes_count := strings.Count(s2, "-")
			if dashes_count > 1 { //check if there is more than 1 dash, (some usernames have dashes)
				error_message += "```diff\n- ERROR: MANY DASHES IN " + s + "\n\nReporting format:\n```"
				error_message += FORMAT_USAGE
				fmt.Println("ERRROR: " + s)
				return error_message
			}
			spaces_count := strings.Count(s2, " ")
			if spaces_count > 2 {
				error_message += "```diff\n- ERROR: MANY SPACES IN " + s + "\n\nReporting format:\n```"
				error_message += FORMAT_USAGE
				fmt.Println("ERRROR: " + s)
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
					//fmt.Println(s) //the whole thing //debug
					//fmt.Println(player_segment)
					//extract player 1 name and score:
					//fmt.Printf("%t, %s\n", player_segment, player_segment) //debug
					player_one_name := player_segment[0]
					//Trim and sanitize the player1 remaining string to extract the score
					//segment_without_space := strings.Replace(a, " ", "")
					//player_one_score_segment := strings.Join(player_segment[1], "")
					p1s := player_segment[1]
					p1s_no_spaces := strings.ReplaceAll(p1s, " ", "")
					player_one_score := string(p1s_no_spaces[len(p1s_no_spaces)-1]) //last character is player_one's score
					//p2s_i, err := strconv.Atoi(player_one_score)
					//if err != nil {
					//fmt.Println("Error in reading score", err)
					//}
					p2s_no_spaces := strings.ReplaceAll(p2, " ", "")
					player_two_score := string(p2s_no_spaces[0]) //first character here should be player_two's score
					//p1s_i, err := strconv.Atoi(player_one_score)
					//if err != nil {
					//fmt.Println("Error in reading score", err)
					//}
					player_two_name := p2s_no_spaces[1:]

					/*
						fmt.Println("Group:", group)
						fmt.Println(player_one_name, "score:"+player_one_score+":")
						fmt.Println(player_two_name, "score:"+player_two_score+":")
					*/

					//get numerical score
					p1s_i, err := strconv.Atoi(player_one_score)
					p2s_i, err := strconv.Atoi(player_two_score)
					if err != nil {
						error_message += "```diff\n- FORMATTING ERROR IN" + s + "\n\nReporting format:\n```"
						fmt.Println(err)
						fmt.Println("ERRROR: " + s)
						error_message += FORMAT_USAGE
						return error_message
					}

					// FIXME: BUG BUG BUG
					message := "GROUP **" + group + "**.)"
					if p2s_i < p1s_i {
						message += "\n" + player_one_name + "(" + player_one_score + ") WINNER\n"
						message += player_two_name + "(" + player_two_score + ") LOOSER\n"
						message += MATCH_ACCEPTED
						fmt.Println("REPORT: " + s)
						return message
					} else if p2s_i > p1s_i {
						message += "\n" + player_two_name + "(" + player_two_score + ") WINNER\n"
						message += player_one_name + "(" + player_one_score + ") LOOSER\n"
						message += MATCH_ACCEPTED
						fmt.Println("REPORT: " + s)
						return message
					} else {
						message += "\n" + player_two_name + "(" + player_two_score + ") TIE\n"
						message += "\n" + player_one_name + "(" + player_one_score + ") TIE\n"
						message += MATCH_ACCEPTED
						fmt.Println("REPORT: " + s)
						return message
					}
				}
			} else {
				error_message += "```diff\n- FORMATTING ERROR IN" + s + "\n\nReporting format:\n```"
				fmt.Println("ERRROR: " + s)
				error_message += FORMAT_USAGE
				return error_message
			}
		}
	}
	message += "```diff\n- UNEXPECTED ERROR IN " + s + "\n\nReporting format:\n```"
	message += error_message
	fmt.Println("ERRROR: " + s)
	error_message += FORMAT_USAGE
	return message
}

func main() {
	/* Startup procedures:
	#####	*/

	// Check to make sure a bot auth token was supplied on startup
	if len(os.Args) < 2 || len(os.Args) > 2 {
		fmt.Println("Error: You must supply EXACTLY one argument (the bot's authorization token) on startup.")
		os.Exit(1)
	}

	TOKEN := os.Args[1] // discord API Token

	// Returns dg of type *session points to the data structures of active session
	dg, err := discordgo.New("Bot " + TOKEN)
	if err != nil {
		fmt.Println("Error creating discord session", err)
	}

	// Register scan_message as a callback func for message events
	dg.AddHandler(scan_message)

	// Determines which types of events we will receive from discord
	//dg.Identify.Intents = discordgo.IntentsGuildMessages // Exclusively care about receiving message events
	dg.Identify.Intents = discordgo.IntentsAll
	//dg.Identify.Intents = discordgo.IntentsGuilds

	// Establish the discord session through discord bot api
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
