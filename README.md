# Starbot
![alt text](https://github.com/76616c6172/starbot/blob/master/creepy_miku_attack.jpg)

Discord bot, written to help the [CPL](https://liquipedia.net/starcraft/Coach_Pupil_League) admin team by
automating away tedious administrative tasks.

## Completed features
1. `/assignroles`  
- Assign roles based on Google Sheets (assigns Terran/Zerg/Protoss and Coach/Player/Assistant
  roles, creates and assigns new team roles as needed).
  correct spreadsheet ID, and correct role ID strings.
1. `/deleteroles`  
- Delete previously created roles in batches (interactively prompts to select batch of roles to delete).
1. Semi-Automatic match reporting  
- Scans messages in preseason-reporting-week2 channel, performs data validation, and appends match reports to local log on disc.

## WIP/Roadmap/Planned features
1. Associate username with discord_ID and save to Google Sheets
1. Track user name changes and update Google Sheets with new name

## Usage:
```bash
./Starbot bot_authorization_token_here
```

