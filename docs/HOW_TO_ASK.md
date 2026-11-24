# How to ask your assistant

Example questions for every tool and prompt of the `wow` server. You do not need to name the tool: the assistant picks it on its own.
The data comes from the last `/mcp reload` (or `/reload`, or logout), so save a dump before asking. Windows (trainer,
vendor, bank, mail, auction house, professions, social window and guild window) must be opened in the game beforehand. If you ask about another
character on the account, include its name.

You pick prompts from the `wow` server's prompt list in your MCP client (the "+" button or the attachments menu). Each
one optionally takes a character name.

## Status and account

| Tool or prompt | Example questions |
|---|---|
| `wow_status` | "Can you see my character, and how fresh is the data?" · "What needs my attention?" (mail, auctions, gear, bags) |
| `wow_character` | "What level am I and how much gold do I have?" · "Where does my Hearthstone take me, and am I resting in an inn?" · "How many honorable kills do I have and what is my PvP rank?" (`pvp`, since addon 0.7.0) |
| `wow_account` | "How much gold do I have in total across all characters?" · "Which alt is worth playing today?" |
| `wow_export` | "Make a character summary for Discord, without gold." · "Prepare a forum post in BBCode." |
| `wow_search` | "Which quest rewards a Linen Bandage?" · "Who sells a Light Quiver?" · "Where does Linen Cloth show up on this character?" · "Find everything related to Deadmines." |
| `wow_compare_characters` | "Compare my two characters." · "Which alt has better gear and more gold?" |
| `wow_alt_mail` | "What should I mail to my leatherworker?" · "Which materials from my main will be useful to an alt?" (check items flagged as weak evidence in the game) |
| `wow_account_crafting` | "What can I craft from the materials of all characters combined?" · "Can any alt make Embossed Leather Gloves from another character's materials?" |
| `account_overview` | "Review the whole account: gold, cooldowns, mail, old dumps." |
| `first_run` | "What do you see and what should I ask about?" (after installation or for a new character) |

## Quests

| Tool or prompt | Example questions |
|---|---|
| `wow_quests` | "What quests do I have in my log and which can I turn in?" · "Which quest has an unfinished objective?" |
| `wow_quest_lookup` | "Where do I turn in The Glowing Shard?" · "Where do I pick up the next quest of this chain?" · "Where are the mobs for this quest?" (needs Questie installed; for a quest outside your log type `/mcp quest <name or ID>` in game, then `/mcp reload`) |
| `wow_lookup` | "What does this item really do?" · "What does Arcane Shot cost and how long does it take to cast?" · "Is this trinket's Use effect the same as in Classic?" (for something the dump does not know, type `/mcp item <ID, link or name>` or `/mcp spell <ID, link or name>` in game, then `/mcp reload`; shift-click an item into the command to paste its link) |
| `wow_zone_progress` | "How many quests have I done in Darkshore and what is left?" · "Which quest chains did I leave unfinished here?" (needs Questie installed; covers the current zone and the zones of the quests in your log) |
| `wow_guide` | "What is my next step in the guide?" · "What do I have left in this TUGs part and where do I go?" (needs Guidelime with a guide selected) |
| `wow_quest_plan` | "Arrange my quests by zone." · "Which quests are too hard or gray?" |
| `wow_leftovers` | "Do I have quest items in my bags for quests I no longer have?" (these are only candidates, nothing is removed automatically) |
| `wow_quest_rewards` | "Which reward should I choose for this quest?" · "Evaluate all choice rewards in my log." · "Count the rewards with a weight of 1 on agility." (quests with an identical reward list get `suspect`: the assistant will not recommend a choice) |
| `session_plan` | "Plan my session: quests, trainer, bags." · "When should I go back to town?" |

## Gear and bags

| Tool or prompt | Example questions |
|---|---|
| `wow_bags` | "How many free slots do I have in my bags?" · "How much is my junk worth at the vendor?" |
| `wow_equipment` | "Which piece of gear is the most damaged and how much does repair cost?" · "What equipment sets do I have and which one is equipped now?" (`equipmentSets`) |
| `wow_bank` | "What can I sell from the bank?" · "What Leatherworking materials do I have in the bank?" |
| `wow_vendor` | "What does this vendor offer that is an upgrade over my gear?" · "What can I afford?" · "What did the vendor Laird offer?" (up to 10 NPCs are remembered) · "What did I just sell and can I buy it back?" (`buyback`; the buyback list is lost on logout) |
| `wow_compare_item` | "Is this green wand from my bags better than mine?" |
| `wow_find_item` | "Which alt has Runecloth?" · "Where is my Mooncloth?" |
| `wow_mail` | "What do I have in the mail and what is about to expire?" |
| `gear_upgrades` | "What from my bags, bank and the vendor is an upgrade?" |
| `what_to_sell` | "What should I sell and what should I keep for quests and professions?" |
| `wow_bag_plan` | "Bags full: what to keep, what to put in the bank, what to sell, what to auction?" (destroys nothing) |
| `wow_reagents` | "What am I about to run out of: arrows, food, bandages, reagents?" |
| `wow_shopping_list` | "What should I buy before leaving town?" · "What does the budget cover: trainer, repair, ammo, food?" · "When should I return to town?" |

## Gold and auctions

| Tool or prompt | Example questions |
|---|---|
| `wow_gold_history` | "How much gold did I earn in the last week?" · "What were my biggest expenses?" |
| `wow_make_or_buy` | "Is it cheaper to craft or to buy at the auction house?" · "Which Leatherworking recipes save the most?" (assumes one item from one craft; a missing price means "unknown", not zero) |
| `wow_auctions` | "What are the most recently seen auction house prices?" · "Which of my auctions are still listed?" |
| `what_to_auction` | "What is worth listing and what should go to the vendor?" (first search for those items in the auction house) |
| `trainer_shopping` | "What should I buy from the trainer with my gold and what should I put off?" |
| `wow_trainer` | "What does the trainer I opened last offer, and what can I afford?" · "What can I buy from the trainer Astaia?" (the list of remembered trainers is in `known`) |

## Progress

| Tool or prompt | Example questions |
|---|---|
| `wow_progress` | "How much XP per hour am I making and when is the next level?" · "How many times did I die this session?" · "Where does my XP come from this session: kills or quests?" (`xpSources`, since addon 0.7.0) · "How much honor and how many honorable kills did I get this session?" (`honorGained`, `honorableKills`; only when the dumps have `pvp`) |
| `wow_timeline` | "Show my character's timeline for the last 7 days." · "When did I last level up and what changed afterwards?" · "Show only deaths." |
| `wow_session_rating` | "Is this session going faster than usual?" · "How do my XP and gold per hour compare with previous sessions?" |
| `week_in_game` | "Summarize my week in the game: levels, gold, play time and the best session." |
| `wow_journal` | "Describe my recent sessions." · "Give me a session journal in Markdown to paste." (with loot and records) |
| `wow_goals` | "How are my goals going and when will I reach them?" |
| `wow_goal_set` | "Add a goal: level 25." · "Goal: 10 gold." · "Remove goal g2." (writes only the server's goals file) |
| `wow_changes` | "What changed since the last dump?" · "What loot did I get since the last save?" · "Summarize the whole session today (last 4 hours)." · "How much honor did I earn since the last dump?" |
| `wow_rested` | "How much rested XP do I have and when will it be full if I log out in an inn?" |
| `wow_reputation` | "How much do I need to reach Honored with Darnassus?" |
| `wow_spells` | "Which abilities do I already know?" |
| `wow_legacy` | "Where should I spend my Legacy Points?" · "Which Legacy challenges are still open?" |
| `wow_achievements` | "How many achievement points do I have?" · "Which achievements am I closest to finishing, and what is missing?" · "When did I earn my exploration achievements?" (`view: timeline`, `category: Exploration`) |
| `wow_statistics` | "How many times have I died in total?" · "How much gold have I looted?" |
| `wow_collections` | "Which titles do I have and which one is shown?" · "Which mounts and companion pets does this character know?" |
| `wow_talents` | "What can I spend a talent point on right now?" · "Show the talent descriptions from the Beast Mastery tree." |
| `wow_recipes` | "What can I craft right now to level up Leatherworking?" · "What is missing for this recipe?" |
| `wow_cooldowns` | "Does any alt have a profession cooldown ready?" |
| `session_summary` | "What did the last session give me: XP, gold, quests?" |
| `before_logout` | "What should I do before I log out?" (inn, mail, auctions, repair, bags) |
| `profession_progress` | "What is the cheapest way to level a profession from what I have?" |
| `talent_planning` | "Where should I spend the next points and what build should I aim for?" |

## Pet

| Tool or prompt | Example questions |
|---|---|
| `wow_buffs` | "What buffs do I have and am I missing any?" (time remaining is shown when the dump comes from addon 0.7.0, which records `expiresAt`) |
| `wow_flights` | "In which quest zones do I not yet have a flight point?" |
| `wow_quest_route` | "In what order should I visit the zones with my quests?" · "Where do I have the most quests to turn in?" (a suggestion based on counts, not the shortest route) |
| `wow_pet` | "What level is my pet and how many free training points does it have?" · "Is my pet happy?" · "Which ranks am I missing and what fits in the free points?" · "Which stabled pet is better than the active one?" |
| `pet_care` | "Take care of my pet: loyalty, food from my bags, Beast Training." |
| `wow_ammo` | "How many hours will my arrows last and how many should I buy for 3 hours of play?" |

## Group and dungeons

| Tool or prompt | Example questions |
|---|---|
| `wow_group` | "Who is missing from the group: tank, healer?" · "Show the gear of the player I inspected." (after clicking Inspect) |
| `wow_dungeons` | "Which dungeons fit my level?" · "Which quests can I do in Deadmines?" |
| `dungeon_finder` | "Where should I go for a dungeon and how should I prepare?" |
| `wow_social` | "Which of my friends are online now?" · "Who from the guild is online and in which zone?" · "Show the whole guild, including offline members." · "Which classes and levels are online in my guild?" · "Who from the guild or my friends could I group with now?" (`groupCandidates`, `levelRange`, default 5; roles are what a class can do, not what the player picks) (first open the social or guild window in the game and do `/mcp reload`; the addon does not ask the game server for these lists on its own) |

## Diagnostics

| Tool or prompt | Example questions |
|---|---|
| `wow_section` | "Show the `probe` section." · "What do I have on my action bars?" · "Show `completedQuests`, I want to check prerequisite quests." |
| `wow_status` | "Did the addon report any section errors?" · "How long does a dump take and which sections are disabled?" |

You can set a note for the assistant in the game: `/mcp note I want Deadmines today`. `session_plan` treats it as the session goal, and `before_logout` reminds you about it.
