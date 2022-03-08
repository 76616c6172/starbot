# Starbot
![alt text](https://github.com/76616c6172/starbot/blob/master/creepy_miku_attack.jpg)

Discord bot, written to help the [CPL](https://liquipedia.net/starcraft/Coach_Pupil_League) admin team by
automating away tedious administrative tasks.

## Completed features
1. `/assignroles`  
- Assign roles based on Google Sheets (assigns Terran/Zerg/Protoss and Coach/Player/Assistant
  roles, creates and assigns new team roles as needed).
- Requires Correct spreadsheet ID, valid Google API Token and correct discord role and server IDs .
2. `/deleteroles`  
- Delete previously created roles in batches (interactively prompts to select batch of roles to delete).
3. **Match report logging**
- Scans messages in preseason-reporting-week2 channel, performs data
  validation, and appends match reports to web viewable log.
4. **Twitch Clip logging**
- Scans messages in cpl-clips channel, and appends messages containing a
  twitch url to web viewable log.

## WIP/Roadmap/Planned features
1. Scan users from players.json and associate discord username#identifier with snowflake id (immutable)
1. Track user name changes and update internal data structures with past names 
1. Expose API for adding new user (scan and save snowflake id) from WebApp
