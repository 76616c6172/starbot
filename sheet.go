package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

const SPREADSHEET_ID string = "1K-jV6-CUmjOSPW338MS8gXAYtYNW9qdMeB7XMEiQyn0"

type player_t struct {
	discordName string
	race        string
}

// error check as a func because it's annoying to write "if err != nil { .. .. }" over and over
func checkError(err error) {
	if err != nil {
		panic(err.Error())
	}
}

//call with map, get map back filled with data,
func parse_sheet(players map[string]player_t) map[string]player_t {
	// Read the secret file (google api key to access google sheets)
	data, err := ioutil.ReadFile("secret.json")
	checkError(err)
	// Create oAuth client for google sheets
	conf, err := google.JWTConfigFromJSON(data, sheets.SpreadsheetsScope)
	checkError(err)
	client := conf.Client(context.TODO())
	srv, err := sheets.New(client)
	checkError(err)

	// see: https://developers.google.com/sheets/api/guides/concepts
	// This gives all screen names

	// Read sheet name cells from spreadsheet
	target_screen_names := "Player List" + "!A2:A"
	screenNameResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_screen_names).Do()
	checkError(err)

	// Read discord name cells from spreadsheet
	target_discord_names := "Player List" + "!B2:B"
	discordNameResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_discord_names).Do()
	checkError(err)

	// Read race cells from spreadsheet
	target_ingame_race := "Player List" + "!E2:E"
	ingameRaceResp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_ingame_race).Do()
	checkError(err)

	//Extract the screen names of the players
	//for _, row := range resp.Values {
	// Loop over the data and add it to the players map
	for i := 0; i < len(screenNameResp.Values); i++ {
		//Extract the screen name
		a := fmt.Sprint(screenNameResp.Values[i])
		a = strings.TrimPrefix(a, "[")
		a = strings.TrimSuffix(a, "]")

		//Extract the discord name
		b := fmt.Sprint(discordNameResp.Values[i])
		b = strings.TrimPrefix(b, "[")
		b = strings.TrimSuffix(b, "]")

		//Extract ingame race
		c := fmt.Sprint(ingameRaceResp.Values[i])
		c = strings.TrimPrefix(c, "[")
		c = strings.TrimSuffix(c, "]")

		//add player data to the map
		players[a] = player_t{discordName: b,
			race: c}
	}

	/* usage goal:
	players["screen-name"].discordname = bob#1337
	players["screen-name"].race = zerg
	//and so on..
	*/

	return players
}

// Testinggg
func main() {

	//maps screenname to player_t (holds info such as discord username, starcraft race, etc..)
	players_m := make(map[string]player_t)
	players_m = parse_sheet(players_m)

	fmt.Println(players_m)
}
