# Starbot
This is a discord bot, written to help the [CPL](https://liquipedia.net/starcraft/Coach_Pupil_League) admin team by
automating away tedious administrative tasks.

## Completed features
1. `/updateroles`  
- Automatically assigns Terran/Zerg/Protoss and Coach/Player/Assistant
  roles based on google sheet
  (requires hardcoded AUTHORIZED_USERS, valid google API Token,
  correct spreadsheet ID, and correct role ID strings.

## WIP/Roadmap/Planned features
1. Automatically create/track assign teams based on google-sheets
1. Track user name changes and update google-sheets

## Usage:
```bash
./Starbot bot_authorization_token_here
```

