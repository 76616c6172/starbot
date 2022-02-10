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
	target_sheet := "Player List" + "!A2:A"
	// Read sheet based on spreadsheet ID
	resp, err := srv.Spreadsheets.Values.Get(SPREADSHEET_ID, target_sheet).Do()
	checkError(err)

	//how the fuck does this work
	//	xx, err := resp.MarshalJSON()
	//checkError(err)
	//fmt.Println(xx)

	//Extract the screen names of the players
	//for _, row := range resp.Values {
	for i := 0; i < len(resp.Values); i++ {
		a := fmt.Sprint(resp.Values[i]...)
		a = strings.TrimPrefix(a, "[")
		a = strings.TrimSuffix(a, "]")

		//fmt.Printf("%T %v\n", row, row)
		fmt.Printf("%T, %v\n", a, a)
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
