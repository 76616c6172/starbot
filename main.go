package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
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
IMPORTANT HARDCODED CONSTANTS
The values here all need to be set correctly for all functionality to work!
##### */

// Hardcode all IDs that are allowed to use potentially dangerous administrative actions, such as /assignroles
var AUTHORIZED_USERS = map[string]bool{
	"96492516966174720": true, //valar
}

const SPREADSHEET_ID string = "1K-jV6-CUmjOSPW338MS8gXAYtYNW9qdMeB7XMEiQyn0" // google sheet ID
const SERVER_ID string = "856762567414382632"                                // the discord server ID
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

// help cmd text
const AVAILABLE_COMMANDS string = `
[ /help        - show commands                        ]
[ /test        - test command                         ]
[ /deleteroles - delete bot created roles             ]
[ /updateroles - update roles from google spreadsheet ]
`

// Discord message formatting strings
const DIFF_MSG_START string = "```diff\n"
const DIFF_MSG_END string = "\n```"
const FIX_MSG_START string = "```fix\n"
const FIX_MSG_END string = "\n```"

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

type player_t struct {
	discord_Name string
	discord_ID   string
	tier         int //can be 0, 1, 2, or 3
	usergroup    int
	race         string
	team         string
}

// Struct to store information about a team-role
type team_t struct {
	name               string
	members            []string
	color              int
	mentionable        bool
	perms              int64
	exists             bool
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

/* #####
Global vars
##### */
var newly_created_roles []string // Holds newly created discord role IDs
var rolesCmd_s rolesCmd_t        // info about /update roles command while being used
//##### End of global vars

// error check as a func because it's annoying to write "if err != nil { ... }" over and over
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
	case "/deleteroles":
		deleteroles(s, m)
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
		if rolesCmd_s.isInUse { //check if this command is in use first and disallow simultanious use
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /updateroles ERROR: EXECUTION IN PROGRESS"+DIFF_MSG_END)
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
			rolesCmd_s.isInUse = true
			rolesCmd_s.session = s
			rolesCmd_s.AuthorID = m.Author.ID
			rolesCmd_s.ChannelID = m.ChannelID
			update_roles(s, m)         //testing new command
			rolesCmd_s.isInUse = false //reset the data so /updateroles can be used again
			_, err = s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"+ /updateroles EXECUTION HAS FINISHED"+DIFF_MSG_END)
			if err != nil {
				fmt.Println(err)
				return
			}

			return

		} else {
			_, err := s.ChannelMessageSend(m.ChannelID, DIFF_MSG_START+"- /updateroles ERROR: "+m.Author.Username+" IS NOT AUTHORIZED"+DIFF_MSG_END)
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
	if rolesCmd_s.isInUse && rolesCmd_s.AuthorID == m.Author.ID {
		_, err := s.ChannelMessageSend(m.ChannelID, "[:sparkles:] Updating races! ")
		if err != nil {
			fmt.Println(err)
		}
		update_roles(s, m)              //testing
		rolesCmd_s.isInUse = false //reset the data so /assignroles can be used again
	}
	*/
}

// Test function executes with side effects and returns final message to be send
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

// WIP: Delete roles that were assigned?
func deleteroles(s *discordgo.Session, m *discordgo.MessageCreate) {
	for _, v := range newly_created_roles {
		//s.State.RoleRemove(m.Member.GuildID, v)
		s.GuildRoleDelete(m.GuildID, v)
		fmt.Println(v)
	}
}

// Builds a map of the desired user state (discord roles) from google sheets
// see: https://developers.google.com/sheets/api/guides/concepts
//func get_sheet_state(players map[string]player_t, disRoles_m map[string]*discordgo.Role) map[string]player_t {
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

	// #########
	//######### end of testing

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
	var ug int
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
		players[a] = player_t{
			discord_Name: b,
			race:         c,
			usergroup:    ug,
		}
	}

	/// ###########
	// WIP - DESIRED FUNCTIONALITY:
	// 1. Get teams sheet data without crashing on empty cells etc

	targetTeams := "Teams" + "!A1:Z" // defines the sheet and range to be read
	resp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, targetTeams).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	// 1. Let's make a list of the teams
	teams_a := resp.Values[0]

	// [teamname].struct that holds various info such as the discord ID, the players
	teamsFromSheet := make(map[string]team_t)

	sheetsTeamList := make([]string, 0)
	// Extract the team names and put into the list
	for _, b := range teams_a {
		x := fmt.Sprint(b)
		if len(x) == 0 { //skip empty cells
			continue
		} else {
			sheetsTeamList = append(sheetsTeamList, x)
			teamsFromSheet[x] = team_t{
				exists: true,
				name:   x,
			}
		}
	}

	// 2. Let's associate the users with their team

	//fmt.Println(sheetsTeamList) // debug

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
			//associate the user with the right team
			switch GROUP {
			case STAFF:
				//fmt.Println("STAFF:", teamIndex, x, screenName) //debug
				players[screenName] = player_t{
					team:      sheetsTeamList[teamIndex],
					usergroup: STAFF,
				}
			case COACHES:
				//				fmt.Println("COACHES:", teamIndex, x, screenName) //debug
				players[screenName] = player_t{
					team:      sheetsTeamList[teamIndex],
					usergroup: COACHES,
				}
			case TIER0:
				//			fmt.Println("TIER 0:", teamIndex, x, screenName) //debug
				players[screenName] = player_t{
					tier: 0,
					team: sheetsTeamList[teamIndex],
				}
			case TIER1:
				//		fmt.Println("TIER 1:", teamIndex, x, screenName) //debug
				players[screenName] = player_t{
					tier: 1,
					team: sheetsTeamList[teamIndex],
				}
			case TIER2:
				//	fmt.Println("TIER 2:", teamIndex, x, screenName) //debug
				players[screenName] = player_t{
					tier: 2,
					team: sheetsTeamList[teamIndex],
				}
			case TIER3:
				fmt.Println("TIER 3:", teamIndex, x, screenName) //debug
				fmt.Println("\n\n\n")
				fmt.Println("ALERT ALERT")
				fmt.Println("#######################")
				fmt.Println("\n\n\n")
				players[screenName] = player_t{
					tier: 3,
					team: sheetsTeamList[teamIndex],
				}
			}
		}
	}

	checkError(err)

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

	/* BIG FUNCTION CALL BIG
	#### */
	// 3. Get desired state of roles from google sheets
	sheetUsrState := make(map[string]player_t)
	//sheetUsrState = get_sheet_state(sheetUsrState, roles_m)
	sheetUsrState = get_sheet_state(sheetUsrState)

	// 4. Check if the user from sheet have desired roles assigned
	// and if not -> assign it!
	/* DEBUG CHECK
	 */
	fmt.Println("discord id:", sheetUsrState["SF)valar"].discord_ID)
	fmt.Println("discord name:", sheetUsrState["SF)valar"].discord_Name)
	fmt.Println("race:", sheetUsrState["SF)valar"].race)
	fmt.Println("tier:", sheetUsrState["SF)valar"].tier)
	fmt.Println("team:", sheetUsrState["SF)valar"].team)
	fmt.Println("usergoup: ", sheetUsrState["SF)valar"].usergroup)
	for screen_name, _ := range sheetUsrState {
		a := sheetUsrState[screen_name].discord_Name
		cordUserid := discord_name_to_id_m[a]

		// Check if the user even exists on the server
		if !discord_id_exists[cordUserid] {
			fmt.Println("Error: couldn't find the user", screen_name, "on discord")
			continue // skip if the user doesn't exist
		}

		// Assign Coach/Assistant Coach/ Player
		group_name := ""
		didAssignGroupRole := false // set to true if we assigned Coach, Assistant Coach, or Player role.
		wishGroup := sheetUsrState[screen_name].usergroup
		switch wishGroup {
		case PLAYER:
			group_name = "Player"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, PLAYER_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		case COACHES:
			group_name = "Coach"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, COACH_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PLAYER_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			didAssignGroupRole = true
		case ASSISTANTCOACH:
			group_name = "Assistant Coach"
			err := dg.GuildMemberRoleAdd(SERVER_ID, cordUserid, ASST_COACH_ROLE_ID)
			checkError(err)
			err = dg.GuildMemberRoleRemove(SERVER_ID, cordUserid, PLAYER_ROLE_ID)
			checkError(err)
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
		wishRace := sheetUsrState[screen_name].race
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
		wishTier := sheetUsrState[screen_name].tier
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
			if didAssignTier {
				cordMessage2 := fmt.Sprintf("> Assigned <@%s> %s to %s\n", cordUserid, screen_name, tier_name)
				_, err = dg.ChannelMessageSend(m.ChannelID, cordMessage2)
				checkError(err)
				didAssignRaceRole = false
			}

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

	TOKEN := os.Args[1] // discord API Token

	// Returns dg of type *session points to the data structures of active session
	dg, err := discordgo.New("Bot " + TOKEN)
	if err != nil {
		fmt.Println("Error creating discord session", err)
	}

	// Register scan_message as a callback func for message events
	dg.AddHandler(scan_message)

	// Determines which types of events we will receive from discord
	dg.Identify.Intents = discordgo.IntentsGuildMessages // Exclusively care about receiving message events
	//dg.Identify.Intents = discordgo.IntentsAll
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

	//Get desired state of roles from google sheets
	sheetUsrState := make(map[string]player_t)
	sheetUsrState = get_sheet_state(sheetUsrState)
	//dg.Close()
	//fmt.Printf("\nBot exited gracefully.\n")
	//os.Exit(0)
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
