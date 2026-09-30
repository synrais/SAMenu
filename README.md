<div align="center">

# SAMenu
### Super Attract Menu for MiSTer FPGA

A games menu, launcher and attract mode for the [MiSTer FPGA](https://github.com/MiSTer-devel), inspired by
[**MiSTer SAM** (Super Attract Mode) by mrchrisster](https://github.com/mrchrisster/MiSTer_SAM).

Browse and search your whole collection, launch games straight into their cores automatically,
and let the MiSTer show itself off with an attract mode that plays random games, music and videos.

</div>

---

## Contents

- [Features](#features)
- [Installing](#installing)
- [First run](#first-run)
- [Using the menu](#using-the-menu)
  - [Controls](#controls)
  - [Back to Menu](#back-to-menu)
  - [Systems and folders](#systems-and-folders)
  - [Search](#search)
  - [History and favourites](#history-and-favourites)
  - [Choosing a core](#choosing-a-core)
  - [The Options menu](#the-options-menu)
  - [Picture](#picture)
- [Genres and playlists](#genres-and-playlists)
  - [Genres](#genres)
  - [Attract playlists](#attract-playlists)
  - [Custom genres](#custom-genres)
- [Attract mode](#attract-mode)
  - [How games are picked](#how-games-are-picked)
  - [Attract mode controls](#attract-mode-controls)
  - [Input test](#input-test)
  - [Game lists](#game-lists)
  - [Static screen detector](#static-screen-detector)
- [BIOS skip](#bios-skip)
- [Music player](#music-player)
- [Video player](#video-player)
- [Startup and idle](#startup-and-idle)
- [Command line](#command-line)
- [Configuration: SAMenu.ini](#configuration-samenuini)
  - [Name tags](#name-tags)
- [Systems and groups](#systems-and-groups)
- [Files and folders](#files-and-folders)
  - [Easy on the SD card](#easy-on-the-sd-card)
- [Building from source](#building-from-source)
- [Credits](#credits)

---

## Features

- 🎮 **A fast games menu** for 114 systems (consoles, handhelds, computers and arcade), grouped and sorted the way you like, with a search that works from a controller.
- 🚀 **Launches games automatically**, with temporary MGL files generated in RAM, straight into the right core, including special cases like AmigaVision.
- 💾 **Easy on the SD card:** the busy, repetitive work happens in RAM, and memory use is capped (see [Easy on the SD card](#easy-on-the-sd-card)).
- 📺 **Attract mode** plays random games one after another, with smart picking (balanced across systems, no repeats, one version per title), a history to go back and forward through, and full control from a controller, keyboard, mouse or SSH.
- 🎲 **[Pick Random Game]** at the top of every list, and **virtual [Games A-Z] folders** for big genre-sorted sets.
- ⭐ **[Favourites]** you mark with one button, and **[History]** of the games you've played (and what attract mode played), newest first with the day.
- 🗂️ **[Genre Collection]** from your genre folders: browse every game by genre across all systems, and build **attract playlists** like "Fighters Night", including **custom genres** made from any word ("Mario").
- 🎛️ **Remappable controls** for the menu and attract mode, including controller buttons MiSTer doesn't pass to scripts (Start, Select, L2…), and an **Input test** screen that shows what every controller sends.
- 🔀 **Choose a different core** for any system from the core files on your SD card or USB drive.
- 🖥️ **A static screen detector** spots games stuck on a black or frozen screen, skips them, and learns from it.
- 🕹️ **BIOS skip** presses buttons on a virtual pad after a game loads, to get past BIOS screens.
- 🎵 **Background music**, and 🎬 **a video player**, including videos between attract games and streaming from a PC.
- ⏱️ **Startup options**: boot into SAMenu or attract mode, or start attract mode when the MiSTer's been left idle.
- 🔧 **One settings file**, `SAMenu.ini`, and almost everything in it can also be changed from the menu.
- 🔌 **Remote control** of everything over SSH, so it works with Zaparoo, Home Assistant or your own scripts.

---

## Installing

1. Copy **`SAMenu.sh`** to **`/media/fat/Scripts/`** on your MiSTer's SD card.
2. On the MiSTer, open the **Scripts** menu and run **SAMenu**.

That's it. The first time it runs, SAMenu creates its folder, `/media/fat/Scripts/.MiSTer_SAMenu/`, and its settings file, `SAMenu.ini`.

> [!NOTE]
> SAMenu is a single program with nothing else to install. MPlayer for videos, and everything else it needs, is built in.

---

## First run

The first time SAMenu opens, it **builds its games database** by scanning your games folders (`/media/fat/games`, USB drives, network shares and so on), counting each system as it goes. With a big collection this takes a few minutes. After that, SAMenu opens instantly.

Rebuild the database whenever you add or remove games: **Options → Game Database → Rebuild games database**, or `SAMenu.sh -rebuild` over SSH. Only one build runs at a time, and your old database stays usable until the new one's complete.

Ticking systems on or off in **Options → Game Database → Database systems** doesn't need a full rebuild: when you leave the screen, the systems you ticked off are dropped straight away, and only the ones you ticked on are scanned. The other systems stay as they were, so rebuild if you've also added or removed games elsewhere.

---

## Using the menu

### Controls

SAMenu works with a keyboard or a controller. MiSTer turns controller buttons into keys for scripts, so both work the same way.

| Controller | Keyboard | Does |
|:--|:--|:--|
| D-pad | Arrow keys | Move |
| **A** | Enter | Open, launch, choose |
| **B** | Esc | Back |
| **Y** | Tab | Search |
| **X** | Space | Options |
| **L** / **R** | Page Up / Page Down | Page up / down |
| (map L2 / R2) | `[` / `]` | Previous / next letter |

In a game list, **Y** (Tab) marks the highlighted game as a favourite, and in [Favourites] **X** (Space) removes one (see [History and favourites](#history-and-favourites)). Every list wraps around with Up and Down: Up at the top goes to the bottom, and Down at the bottom goes back to the first game (skipping **[Pick Random Game]**, so it's only ever chosen on purpose). **L** and **R** move a page at a time and stop at the ends: on the last page R goes to the last line, on the first page L goes to the first game (on a list shorter than a page, straight to the top or bottom).

**Previous letter** and **Next letter** jump any list by the first letter of its lines: next goes to the first title of the next letter, previous back to the start of the current letter (or, from there, the start of the one before). Numbers count as one letter, and a leading `* ` or `[System]` is skipped. They stop at the ends and never land on [Pick Random Game]. The keyboard has them on `[` and `]`; for a controller, map L2 and R2 (or any buttons) to them in **Options → Controls → Games Menu**.

**Menu Layout** can swap A and B, **Japanese** style: B confirms, A goes back.

All of these can be remapped in **Options → Controls → Games Menu**, each action for the place it applies: Search and Options on the systems list, Favourite in game lists, Remove in [Favourites], Previous and Next letter in every list. So one button can do different things in different places. Choose an action and press the key or button for it (a bound one removes it). Controller buttons MiSTer doesn't pass on as keys (Start, Select, L2, R2…) work too: SAMenu reads those itself.

### Back to Menu

The quickest way from a game back to SAMenu: turn on **Hotkey** in **Options → Controls → Back to Menu**, and in any game you launched from the games menu, hold **Start + Select** for 1.5 seconds, or press `` ` `` twice on a keyboard (as in attract mode). The game closes and the games menu opens again, about 3 seconds later. **Go back to** can instead take you to the **MiSTer menu**, or run a **Script**: choose it on the **Script** line, browsing the SD card from the Scripts folder. It runs on the TV the way the Scripts menu runs one, then "Press any key to continue" goes back to the MiSTer menu. **Buttons** takes one controller button, or two held together (hold them, then let go), **Hold time** is 0.5 to 3 seconds, and **Key (press twice)** is the keyboard key (pressing the one already set removes it). The game sees the presses too, so pick ones it won't mind. **Works in** says where it works. **Games started by SAMenu** *(default)*: only in games the games menu launched (once you leave that game some other way, it stops until the menu launches the next one). **Games**: in every game, however it was launched (MiSTer's own menu, Zaparoo, the command line), so it's also a quick way back to the stock **MiSTer menu**. **MiSTer menu**: only in the MiSTer menu, where it opens SAMenu (or runs the script; with Go back to MiSTer menu it does nothing there). **Anywhere**: every game and the MiSTer menu. Past Games started by SAMenu, SAMenu's background watcher runs from boot, the same one that starts attract mode when idle, reading the controllers ten times a second (about 1% of the CPU). It stays quiet while SAMenu is open, attract mode or a video plays, a core is still loading, or a script like update_all runs.

### Systems and folders

The first screen lists your systems. How they're grouped and sorted is up to you (**Options → Display & Sorting**): by category or manufacturer, as one list with headings or as folders you open, by release date or A-Z, and with the Arcade Cores pinned to the top if you like.

Open a system to see its games and folders. Along the way you'll find:

- **`[Pick Random Game]`** at the top of every list. It launches a random game from everything at that level and below, picked the same smart way as attract mode. (Options → Display & Sorting → [Pick Random Game].)
- **`[Games A-Z]`** at the top of the systems you choose. It lists every game from all of that system's folders once, A-Z, which is perfect for a PlayStation set sorted into genres, without keeping a second copy. (Options → Display & Sorting → Virtual A-Z Folders.)
- **Disc sets**, which can be grouped into one folder per game (Options → Display & Sorting → Game list sorting).
- **Hide games tagged…**, which leaves betas, prototypes, hacks, bad dumps and so on out of the lists entirely (see [Name tags](#name-tags)).
- **`[Favourites]`** and **`[History]`** on the systems list (see [History and favourites](#history-and-favourites)).
- **`[Genre Collection]`** on the systems list: every game by genre, across all systems (see [Genres](#genres)).

With **Remember position** on (in Menu list options), SAMenu opens where you left off.

### Search

Press **Y** (Tab) from the systems screen, then type with the on-screen keyboard:

| Controller | Keyboard | On the search keyboard |
|:--|:--|:--|
| **A** | Enter | Type the highlighted key, or press **Search** / **Back** |
| **B** | Esc | Leave search |
| **X** | Space | Type a space |
| **Y** | Tab / Backspace | Delete the last character |
| **L** / **R** | Page Up / Page Down | Move the text cursor |

Move down past the bottom row of keys to reach the **Search** button. On a keyboard, just type and press Enter.

From 3 letters on, the number of games your search will find shows under the text as you type (`1,284 matches`, `No matches`), so you know whether it's worth pressing Search. The same live count appears when typing a playlist's Skip words or a custom genre.

Results always read **`[System] Title.ext`**, with the systems A-Z and then the titles A-Z. **A** launches the highlighted game, and **B** goes back to the keyboard with your text kept.

**From SSH or a script:**

- `SAMenu.sh -find mario 3` plays every game matching the words, then carries on with attract mode as usual. Nothing to pick, and it starts attract mode if it isn't running. It's the same as the search key during attract mode.
- `SAMenu.sh -search` opens the search screen **on the TV** (for remotes like Zaparoo).
- `ssh -t root@<mister-ip> "/media/fat/Scripts/SAMenu.sh -search -here"` opens it **in your SSH window** instead. The games you pick still load on the MiSTer. The whole menu works that way too: `SAMenu.sh` on its own, or `-menu -here`.

### History and favourites

**`[History]`** on the systems list lists the games you launch from SAMenu, newest first, with the day each was played:

```
[History]
  Today      [SNES] Street Fighter II Turbo
  Yesterday  [Playstation] Tekken 3
  26 Sep     [NES] Duck Tales
```

- It records games launched from any folder, search, [Pick Random Game], [Genre Collection], [Favourites] and [History] itself, but not attract mode's. It's kept on the SD card, so it survives a restart.
- If attract mode has run since the MiSTer started, [History] has two folders: **Attract** (what attract mode played, newest first) and **Games Menu** (yours). Otherwise it goes straight into yours.
- **Options → Display & Sorting → [History]:** show it or not, **Played again: to the top** (on: a replayed game moves to the top; off: it's listed again), how many to **Keep** (25 to 250), where each game's system is shown, and **Clear history now**.

**Favourites:** in any game list, **Y** (the **Fav** button) marks the highlighted game with a `* ` in front of its name, and pressing it again unmarks it. The mark doesn't change the order. **`[Favourites]`** on the systems list lists them all by title, and **X** (**Remove**) takes one off. Attract mode can add the game on screen too, with its **Favourite** action.

- **Options → Display & Sorting → [Favourites]:** switch favourites on or off (off hides the button, the marks and the folder), and where each game's system is shown.

### Choosing a core

**Options → Game Database → Cores** picks which core each system uses (Arcade isn't listed: its MRA files name their own cores). The systems are listed under the folder their core is in: `_Console`, `_Computer` and `_Other` first, then any other folder in use, like `_Unstable` or one on a USB drive, with the systems A-Z in each (always, whatever the menu's own sorting). Choose a system, and browse the core files on every drive, like the games menu: the `_` folders at the root of the SD card and USB drives (except `_Arcade`) and their `_` subfolders, at any depth, merged across drives, with each file tagged by its drive, like `SNES_20260905.rbf (USB0)`. **Default** puts the system's own core back, and the core in use is marked `*`.

The exact file is saved, so that build is the one that loads. It's stored as a `set_core` line in `[Systems]`, which you can also write by hand. For one launch only, use `-launch <file> -core <core>`, which wins over `set_core`.

### The Options menu

Press **X** (Space) on the systems screen.

| Section | What's in it |
|:--|:--|
| **Game Database** | Rebuild the database (it asks first), choose which systems it includes (added or removed straight away, without a full rebuild), and which core each system uses |
| **Attract Mode** | Start attract mode, its settings, attract playlists, the detector and list settings, and the game lists (take games off them) |
| **Display & Sorting** | Menu list options (labels, remember position), menu list sorting, game list sorting (including hidden tags), Virtual A-Z Folders, [Pick Random Game], [Genre Collection], [Favourites] and [History] |
| **Screen** | **Text size** (Auto picks the biggest that still fits the menu) and **Keep text off edges** (Off, 1, 2 or 3 lines kept clear at the top and bottom, and the same share of the width at the sides, for TVs that cut the edges off). The preview shows the room the menu has, in characters. **Picture** is MiSTer's own picture settings (see [Picture](#picture)) |
| **Controls** | **Games Menu** (Menu Layout and the menu's mapping), **Attract Mode** (which inputs it watches, sticks, its mapping, and what other buttons do), **Back to Menu** (the hotkey from a game back to a menu or a script), **BIOS Skip** (when it runs, and each system's sequence) and the **Input test** |
| **Music Player** | Play and stop, next and previous track, the playback order and the playlist |
| **Video Player** | Browse and play videos, the playlist settings, videos in attract mode, and the sync options |
| **Startup** | What starts when the MiSTer boots |
| **Logs** | Read the logs worth checking when something didn't go as expected (see below) |

Changes are saved to `SAMenu.ini` as you make them.

**Logs** lists the logs there are right now, with how old and how big each is:

- **SAMenu logs**: attract mode (every game, skip and press, and why it stopped), the background watcher (when it counts idle time, and why not, and when it starts attract mode; where the Back to Menu hotkey works and what it did), startup (the wait for the MiSTer, the countdown, what it started), BIOS skip (each sequence and press), the music and video players (each track or video, pauses) and opening SAMenu on the TV.
- **MiSTer logs**: Linux's kernel messages (what `dmesg` shows: devices plugged in, SD card and USB errors), MiSTer's own program after SAMenu had to restart it, the system log if there is one, and every `.log` file the updaters keep under `/media/fat/Scripts/.config` (the downloader's, for one).

A log opens at its newest line. Scroll up with Up and PgUp; **Refresh** reads it again, for one that's still being written. Only the last 256 KB of a big log is read. SAMenu's logs are in `/tmp`, in RAM: they're gone after a reboot.

The options screens' lists of systems always look the same, whatever your Display & Sorting settings: under **Arcade**, **Consoles**, **Handhelds**, **Computers** and **Other**, A-Z in each. That's database systems, attract mode systems, Virtual A-Z Folders, leaving systems out (of [Genre Collection] or a playlist), a playlist's Per system, BIOS Skip's Add a system, and the Game lists. **Cores** is the one exception: it's grouped by the folder each core is in (see [Choosing a core](#choosing-a-core)).

In the lists you tick, each group's heading has its own box: **[x]** all on, **[ ]** none, **[-]** some. Choose it to turn the whole group on, or off when it already is. **All** and **None** do the whole list.

### Picture

**Options → Screen → Picture** changes the settings in MiSTer's own `MiSTer.ini` that decide whether your screen shows a picture at all, in plain words. The preview underneath says what they add up to (what the analog output sends, how games and the menu go out, the HDMI resolution).

| Setting | What it does | MiSTer.ini |
|:--|:--|:--|
| **My screen** | Pick what's plugged in and the rest is set to suit it: HDMI TV or monitor (which leaves the analog settings as they are), CRT TV over RGB (SCART), component, S-Video or composite, a PC monitor over VGA, or an HDMI to VGA adapter. **Custom** once you change something by hand so it matches none of them | several |
| **Analog signal** | What the analog port sends: RGB, Component (YPbPr), S-Video or Composite | `vga_mode` |
| **Colour system** | For S-Video and composite: NTSC, PAL-60 or PAL-M | `ntsc_mode` |
| **Sync** | For RGB: Separate (VGA), Combined (SCART) or On green | `composite_sync`, `vga_sog` |
| **Menu on analog** | **Scaled, fits any screen** sends the MiSTer menu (and SAMenu) out of the analog port at the Resolution below, so a TV or monitor that can't take the menu's own picture still shows it. Only the menu: games aren't slowed | `vga_scaler` in `[Menu]` |
| **Games on analog** | **Native, no lag**: each core's own picture (15 kHz for most, for CRT TVs). **Doubled, for PC monitors**: 15 kHz cores doubled to 31 kHz, still with no lag. **Scaled, small lag**: every core at the Resolution below, which almost any monitor takes | `forced_scandoubler`, `vga_scaler` |
| **Resolution** | The HDMI picture (and the scaled analog one): Auto from the screen, or 640x480 up to 2560x1440 | `video_mode` |
| **Top and bottom cut off** | A border for TVs that cut the top and bottom of the picture off | `vscale_border` |
| **HDMI colour range** | Full for monitors, limited for TVs, or the range for HDMI to VGA adapters | `hdmi_limited` |
| **HDMI to VGA adapter** | Direct video: each core's own timing over HDMI, for an adapter to a CRT | `direct_video` |

Nothing changes until you choose **Try these settings** (leaving the screen asks too). The new settings go in a copy of the file in RAM, used in place of the real one, and the MiSTer menu loads again with them; the screen goes dark for a few seconds. Then it asks **Can you see this?**, with a 15 second countdown: **Go back** (what it does by itself if you can't see anything, and what's highlighted), **Until reboot** (the copy stays in use until the MiSTer restarts, and the SD card isn't touched) or **Keep** (written into MiSTer.ini for good; the first time, the file as it was is kept as `.MiSTer_SAMenu/MiSTer.ini.bak`). **Undo the temporary settings** takes a copy off again. Games on the analog output can only be checked in a game: if one shows nothing, a reboot (or `-screen undo`) brings the old settings back.

If MiSTer uses one of its other settings files (`MiSTer_<name>.ini`, picked in its own menu), SAMenu finds out when it tries the settings, and switches to that file.

From a PC, `SAMenu.sh -screen status` shows the settings in use, `-screen undo` takes temporary ones off, and `-screen restore` also puts MiSTer.ini back as it was before SAMenu changed it for good.

---

## Genres and playlists

### Genres

A game's genres come from **the folders it's in**, worked out when the games database is built. `SNES/Genres/Fighting/…` makes a game **Fighting**, and many spellings are recognised (`_Fighter`, `[Sports]`, `Shoot 'em Ups`, `Puzzle - Logic` and so on). A folder only counts when its whole name is a genre, so a folder named after a game, like *Street Fighter II*, doesn't. Sub-genres count as their genre too: **Sports/Golf** is also **Sports**.

- **Browse by genre:** `[Genre Collection]` on the systems list shows every game by genre, across all systems. Each game's system can be shown before or after its name, and the games can be ordered by name or by system (Options → Display & Sorting → [Genre Collection]).
- **Leave systems out of [Genre Collection]:** in the same settings, so a system you don't want there never shows up in it (it still appears everywhere else).
- **Check what was found:** `SAMenu.sh -genres` prints the genres in your database, with counts per system.
- **Your own folder names:** add them in `SAMenu.ini`, under `[Genres]` for folders and `[Genres.Files]` for game names, then rebuild the database:

```ini
[Genres]
Fighting = My Fighters, VS Games
Sports/Golf = Golfing

[Genres.Files]
Fighting = Street Fighter*, Tekken*
```

### Attract playlists

A playlist decides **which games attract mode plays, by genre**, as a layer over your normal setup. Switch back to **Normal** and everything is as it was. Play time, the detector, the lists, orientation and skip tags all still apply.

Make them in **Options → Attract Mode → Playlists**:

- **Genres for all systems**, and **extra genres for one system** (Per system).
- **Systems with no picks:** leave them out, or play them as normal.
- **Skip:** words to never play. Type them with spaces between, like `mahjong pachinko shogi`, and anything with one of those words in its name or folder is skipped. The live count shows how many games that catches.
- **Leave out systems:** systems this playlist never plays, whatever genres are picked. Handy with genres for all systems: *Fighting from every system, except the computers*.
- **Attract mode plays:** which playlist is in use, or Normal.

Or pick one for a single run from the command line: `SAMenu.sh -attract -playlist "Fighters Night"`.

### Custom genres

Every genre-picking screen starts with **Create custom genre...**. Type a word or phrase, like `mario` (the live count shows how many games match as you type), and it becomes a genre called **Mario**: every game with that word in its **name or folder**, whatever genre it's filed under. That includes flat sets with no genre folders at all.

- It's added **ticked**, alongside the real genres, and a game plays if it's in any of the ticked ones, only ever once.
- If the name matches a real genre, it gets its own entry, **Puzzle - Made by you!**, next to the real **Puzzle**.
- Custom genres from your other playlists are listed at the top of every genre screen, so each only needs making once. Unticked everywhere, they disappear.

---

## Attract mode

Attract mode plays random games one after another, for **`PlayTime`** seconds each (45 to 60 by default), until you pick one up and play it.

Start it from **Options → Attract Mode → Start attract mode**, over SSH with `SAMenu.sh -attract`, or automatically at boot or when the MiSTer's idle (see [Startup and idle](#startup-and-idle)).

### How games are picked

**Selection** (in Options → Attract Mode → Attract mode settings, or `[Attract] Selection`):

| Selection | How it picks |
|:--|:--|
| **Balanced** *(default)* | Systems are weighted by the square root of their size, so big libraries still come up more often but don't take over |
| **Per game** | Every game is equally likely, so systems with huge libraries dominate |
| **Per system** | A random system, then a random game, giving every system equal airtime |
| **By category** | A random category (Console, Computer…), then a system, then a game |
| **By manufacturer** | The same, with manufacturers |
| **Round robin** | Each system in turn (A-Z), with a random game from each |
| **Time machine** | Each system in turn by release date, oldest first: a tour of gaming history |
| **In order** | Every game in turn, system by system, A-Z |

Then fine-tune it:

- **No repeats**: every game plays once before any repeats.
- **Mix systems**: never the same system twice in a row.
- **One version per title**: *Tetris (USA)*, *(Europe)* and *(Japan)* count as one game. It prefers USA, then World, then Europe, clean releases before betas and hacks, and disc 1 of a multi-disc game.
- **Skip games tagged**: never pick betas, prototypes, disc 2 and so on (see [Name tags](#name-tags)).
- **Orientation**: for arcade games, Both, Horizontal, Vertical, Vertical CW or Vertical CCW, read from each MRA's `<rotation>` line. Great for a vertical cabinet.
- **Include / Exclude**: systems or groups, for example `Include = Console` and `Exclude = Nintendo`.
- **`[Weights]`**: make systems or groups come up more or less often, for example `SNES = 3`, `Computer = 0.5` or `Arcade = 0`.

**[Pick Random Game]** and `-random` pick the same way (Selection, One version per title and Weights).

### Attract mode controls

What each input does is set in **Options → Controls → Attract Mode → Mapping**, or in the `[InputDetector.*]` sections of `SAMenu.ini`. The defaults are:

| Action | Controller | Keyboard | Mouse |
|:--|:--|:--|:--|
| **next**: next game | D-pad right, stick right | Right | Right button |
| **back**: previous game (the last 100 are kept) | D-pad left, stick left | Left | Left button |
| **play**: stop attract mode and keep playing this game | Start | Enter | |
| **stop**: stop attract mode and go back to the menu | Select | Esc | |
| **blacklist**: never play this game again, next game | | Delete | |
| **stay**: stay on this game (again to carry on) | | Space | |
| **search**: type a search whose results play first (twice quickly opens SAMenu) | | `` ` `` | |
| **menu**: close the game and open SAMenu on the TV | | | |
| **favourite**: add the game on screen to your favourites (again to take it off) | | | |
| **mute**: sound off / on | | | |
| **screenshot**: take a screenshot (in MiSTer's usual screenshots folder) | | | |

Favourite, mute and screenshot act without leaving the game.

Any other button does what **Other buttons** (`OtherInput`, in Options → Controls → Attract Mode) says: Ignore, **Play** *(default)* or Stop.

**Which inputs attract mode watches** is set in the same place: the mouse, keyboard and controllers each on or off, and **analog sticks**:

- **Sticks** on or off. Buttons and the d-pad always count.
- **Stick hold** (50 to 250 ms, 75 by default): how long a stick must stay pushed before it counts, so a glitch, like an empty port on a multi-port adapter jumping for an instant, doesn't stop attract mode.

Attract mode can also **mute** the MiSTer while it runs (`Mute = true`). The sound comes back when it stops or when you choose a game. If you'd muted the MiSTer yourself, it leaves your mute alone, and if the MiSTer restarts while attract mode had it muted, the sound is put back at the next boot.

### Input test

**Options → Controls → Input test** shows, live, what attract mode's input detectors see:

- **Controllers:** every keyboard, mouse and controller being watched, with how each controller's buttons were named.
- **Sticks:** bars for the last controller used, with `|` marking where a push starts to count, so drift and dead zones are easy to see.
- **Inputs:** every press, newest first, with the time, the device's port, the input's name (as used in `SAMenu.ini`) and what attract mode would do with it. Inputs the stick settings filtered out are shown dimmed, with why (`ignored: too short`).

Hold **Back** for 2 seconds to leave, then let go: the controller button you back out of the menu with (**B**, or **A** with the Japanese Menu Layout), a controller's Back/Select, or the keyboard's **Esc** (Enter with the Japanese layout). Tapping it just shows it, and every other button can be tried freely.

Over SSH, **`SAMenu.sh -inputs`** prints the same names, for example `joystick (8BitDo Pro 2): dpleft`.

Names are SDL's, as used in `SAMenu.ini`, and the face buttons say what they're called elsewhere, since MiSTer names them differently: `a (MiSTer B, Xbox A, PS X)`, `b (MiSTer A, Xbox B, PS O)`, `x (MiSTer Y, Xbox X, PS [])`, `y (MiSTer X, Xbox Y, PS /\)`.

### Game lists

Per-system lists live in `.MiSTer_SAMenu/Lists/`, one file per system, with one game title per line. The extension is optional, and lines starting with `#` or `;` are ignored.

| List | File | What it does |
|:--|:--|:--|
| **Blacklist** | `Lists/Blacklist/SNES_blacklist.txt` | Games attract mode never plays. The static detector adds black-screen games, and the **blacklist** action adds the current game |
| **Staticlist** | `Lists/Staticlist/SNES_staticlist.txt` | When each game's screen freezes, for example `<42> Game Name`. Attract mode moves on `SkipafterStatic` seconds after that point |
| **Whitelist** | `Lists/Whitelist/SNES_whitelist.txt` | A list you make yourself. For a system that has one, attract mode only plays the games on it |

Each list can be switched on or off, and limited to some systems, in `[List]`.

**Options → Attract Mode → Game lists** shows each list, system by system, to take games off it: **Remove** takes off the highlighted game straight away, **Remove all** (it asks first) the whole system's list. On a list's systems screen (grouped under Arcade, Consoles, Handhelds, Computers and Other, like every options screen's systems), the **Remove all** button clears that list for every system at once (it asks first too). A system's list goes once its last game does, so a whitelist never stays behind empty: an empty whitelist would play nothing for its system, while no whitelist plays everything. Comments in a list file are kept.

### Static screen detector

While attract mode plays a game, the detector watches the screen. After a **Grace** period, a screen that stays **black** or **unchanged** for too long can be added to the Blacklist or Staticlist, and skipped.

"Unchanged" is measured carefully. The screen is split into a 16×12 grid, and it only counts as moving if at least **MinSpread** cells changed in the last **SpreadTime** seconds. So a flashing cursor or a blinking *PRESS START* counts as still, while a small sprite moving around counts as moving.

To watch what it sees, live, from a PC:

```bash
ssh -t root@<mister-ip> "/media/fat/Scripts/SAMenu.sh -watch"
```

Every setting can be overridden per system or group, for example `[StaticDetector.PSX]` with a longer `Grace` for slow-loading CD games.

---

## BIOS skip

Some games stop at a BIOS or disk-system screen until a button's pressed. SAMenu plugs in a **virtual pad** (called *SAMenu*) before the game loads, presses a sequence of buttons, then unplugs it.

Sequences are set per system:

```ini
[BiosSkip.FDS]
Sequence = 10, a
```

| In a sequence | Means |
|:--|:--|
| A number | Wait that many seconds (start with one, to let the game load) |
| `a b x y l r select start home up down left right` | Virtual pad buttons (MiSTer's default map) |
| `key:enter`, `key:space`, `key:f1`… | Keyboard keys, for cores that read a keyboard |

Keys are written as plain names, the same ones **Add a press** saves when you press a key: a single letter, number or symbol (`key:a`, `key:1`, `key:!`), or `enter`, `esc`, `space`, `tab`, `backspace`, `f1`…`f12`, `up` `down` `left` `right`, `home`, `end`, `pageup`, `pagedown`, `insert`, `delete`, `capslock`, `numlock`, `scrolllock`, `printscreen`, `pause`, `numpad0`…`numpad9`, `numpadenter`, `numpad+` `numpad-` `numpad*` `numpad/` `numpad.`, and `leftshift` `leftctrl` `leftalt` `leftgui` (and their `right` versions). Capitals don't matter.

**Options → Controls → BIOS Skip** sets it all up from the menu:

- **Used for:** Off, Attract (games attract mode launches), Menu (games you launch) or Both (`[BiosSkip] Attract` and `Menu`).
- **Each system with a sequence**, and **Add a system...**. Choose one to edit its steps: **Add a press** (press the button or key on your controller or keyboard) or **Add a wait**, and choose a step to remove it.

A press is saved by the button's **position**, since the virtual pad's names are Nintendo style (`a` right, `b` bottom, `x` top, `y` left). So pressing your bottom button (Cross on a PlayStation pad) saves `b`, and each step shows it, like `Press b (bottom)`. A button with no virtual pad name gives you a list to pick from.

Test a sequence on a running game with `SAMenu.sh -press start`.

---

## Music player

The music player plays **MP3** and **OGG** files from **`/media/fat/music`** in the background. Each folder inside it is a **playlist**.

- Use **Options → Music Player**, or `SAMenu.sh -music start | stop | next | previous | status`.
- **Play music** starts it. While it plays, **Next track** skips on. With Playback In order, **Previous track** then shows too, and goes back one title in the folder.
- **Playback** is Random or In order. **Playlist** is the music folder itself, a folder inside it, or All.
- **Pause in games** is on by default. MiSTer mixes this music into every core's sound, so it pauses while a game plays and carries on in the menu. It always pauses while a video plays.

---

## Video player

The video player plays videos from **`/media/fat/video`** on the MiSTer's own screen, using the built-in MPlayer. Folders inside it are playlists.

- **Options → Video Player → Play videos…** lets you browse and play, and **Play playlist** plays the chosen playlist.
- **In attract mode**, a video can play between games every 3, 5, 10 or 20 games (`[Video] AttractEvery`).
- **Formats**: mp4, m4v, mov, mkv, webm, avi, mpg, mpeg, ts, wmv, asf, ogv and flv.

> [!IMPORTANT]
> Videos are decoded by the MiSTer's own ARM processor. **SD video at around 480p plays well**, but 720p and above, or HEVC/H.265, is too heavy. Videos play at 640×480, and MiSTer's scaler fills the TV.

**While a video plays** (a controller works too):

| Controller | Keyboard | Does |
|:--|:--|:--|
| D-pad left / right | Left / Right | Back / forward 10 seconds |
| D-pad up / down | Up / Down | Forward / back 1 minute |
| L / R | Page Up / Page Down | Forward / back 10 minutes |
| X | Space | Pause |
| A or B | Enter or Esc | Exit |
| | 9 / 0 | Volume down / up |
| | m | Mute |

**Over SSH**:

```bash
SAMenu.sh -video play                          # the playlist set in [Video]
SAMenu.sh -video play Trailers                 # a folder (All = every video)
SAMenu.sh -video play /media/fat/video/intro.mp4
SAMenu.sh -video next | stop | status
```

**You can also stream a video straight from your PC**, with nothing copied to the MiSTer:

```bash
cat movie.mkv | ssh root@<mister-ip> "/media/fat/Scripts/SAMenu.sh -video play -"
```

MKV, TS and WebM stream best. An MP4 needs its index at the front of the file, which `ffmpeg -i in.mp4 -c copy -movflags faststart out.mp4` sorts out once.

If the sound drifts out of step after skipping, try the **sync options** in Options → Video Player.

---

## Startup and idle

**Options → Startup** sets what happens when the MiSTer boots. SAMenu writes a marked block into `/media/fat/linux/user-startup.sh` and leaves the rest of that file alone.

| Setting | Choices |
|:--|:--|
| **Start** | Nothing, **SAMenu** (boot straight into the menu) or **Attract mode** |
| **Music** | Also start the music player |
| **Attract mode starts** | **Instantly**, **After a delay** (`AttractDelay` seconds, where a press can restart the countdown, cancel it, or be ignored for kiosks), or **When idle** |

**When idle** runs a small watcher that starts attract mode once nothing's been pressed for **IdleTime** minutes, counting in the MiSTer menu, in games, or both. With games included, attract mode also comes back after you pick one of its games and put the controller down. While it isn't counting (attract mode or a video is playing, or you're somewhere it doesn't count), it stops reading the controllers altogether.

> [!TIP]
> SAMenu never starts attract mode while another script is running, such as `update_all`, so a core never gets loaded over an update.

If mrchrisster's MiSTer SAM is also set to start at boot, SAMenu warns you, since the two would fight over the screen.

---

## Command line

Everything can be run over SSH or from any script:

```bash
/media/fat/Scripts/SAMenu.sh <option>
```

| Option | Does |
|:--|:--|
| *(none)* | Open SAMenu |
| **Attract mode** | |
| `-attract` | Start attract mode (its log on screen) |
| `-attract -bg` | Start it in the background (log in `/tmp/SAMenu_attract.log`) |
| `-attract SNES,Console` | Only these systems or groups, this time |
| `-status` | Is it running, and what's playing? |
| `-next` / `-back` | Next or previous game |
| `-play` | Stop attract mode and keep playing this game |
| `-stay` | Stay on this game (again to carry on) |
| `-blacklist` | Never play this game again, next game |
| `-stop` | Stop attract mode and go back to the MiSTer menu |
| `-attract -playlist "Fighters Night"` | Use this attract playlist, this run (Normal = the usual setup) |
| `-find mario 3` | Play the games matching all the words first, then carry on (starts attract mode if it isn't running) |
| **SAMenu on the TV, or here** | |
| `-menu` | Open SAMenu on the TV (closes the running game) |
| `-search` | …straight into its Search screen |
| `-menu -here`, `-search -here` | Open it in this terminal instead, e.g. over SSH with `ssh -t` |
| **Games** | |
| `-launch <file>` | Launch a game, for example `-launch /media/fat/games/NES/Tetris.nes` |
| `-launch <file> -system Atari2600` | …as this system (ID or name), instead of guessing from the file |
| `-launch <file> -core _Unstable/NES` | …with another core, loading the file the usual way (add `-system` if it can't be guessed) |
| `-random` | Launch a random game |
| `-random Nintendo` | …from these systems or groups |
| `-list` | Print every game in the database |
| `-rebuild` | Rebuild the games database |
| `-genres` | Print the genres found in the database, with counts per system |
| **Music and video** | |
| `-music start \| stop \| next \| previous \| status` | The music player |
| `-video play [file \| folder \| playlist \| -]` | The video player (`-` plays a video piped in) |
| `-video next \| stop \| status` | Control the video player |
| **Picture** | |
| `-screen status` | The picture settings in use ([Options → Screen → Picture](#picture)), and any temporary ones |
| `-screen undo` | Take off temporary picture settings, as a reboot would: the way back from a screen that shows nothing |
| `-screen restore` | Also put MiSTer.ini back as it was before SAMenu changed it for good |
| **Testing** | |
| `-inputs` | Print every key, click and controller press |
| `-press start` | Press buttons on the virtual pad |
| `-watch` | Show the static detector's live status |

`-launch`, `-random` and `-video play` end a running attract mode first. The other options never disturb a running menu or attract mode. Options that only work with another (`-bg`, `-playlist`, `-here`, `-system`, `-core`) say so if used on their own.

That makes SAMenu easy to drive from **Zaparoo**, **Home Assistant** or your own scripts.

---

## Configuration: SAMenu.ini

All settings live in **`/media/fat/Scripts/.MiSTer_SAMenu/SAMenu.ini`**. Every setting is explained in the file itself, and the top of the file has a command cheat sheet plus the full list of system IDs and groups. Most settings can be changed from the Options menu instead.

Settings are case-insensitive, and lists are comma separated. Anywhere a system is accepted, a **group** works too (see [Systems and groups](#systems-and-groups)).

<details>
<summary><b>[Systems]</b>: extra games folders and alternative cores</summary>

| Key | Example | Does |
|:--|:--|:--|
| `games_folder` | `/media/usb0/MoreGames` | An extra place laid out like a `games` folder, with a folder per system inside. One line each |
| `system_folder` | `SNES:/media/usb0/SNES Hacks` | An extra folder for one system, used as it is. One line each |
| `set_core` | `SNES:_Unstable/SNES_20260905` | Use a different core for a system (easiest from Options → Game Database → Cores). A full path is tidied, and a core on USB is `../usb0/_Cores/X`. With the build date, that exact build loads; without it, the newest one with that name in the folder |

</details>

<details>
<summary><b>[Database]</b> and <b>[Database.X]</b>: what goes into the games database</summary>

`[Database] Exclude` leaves whole systems or groups out, for example `Exclude = Computer, NESMusic`.

`[Database.X]` sections leave out only some games, where **X** is `ALL`, a system ID or a group:

| Key | Matches |
|:--|:--|
| `Folders` | Folder names inside a system |
| `Files` | File names (the extension is optional) |
| `Extensions` | Extensions, with or without the dot |
| `Paths` | A location and everything under it, for example `/media/usb1` |

Patterns ignore case: `hack` matches exactly, `hack*` starts with, `*hack` ends with, and `*hack*` contains.

```ini
[Database.ALL]
Files = *(Proto)*, *(Beta)*

[Database.SNES]
Folders    = Hacks
Extensions = bs
```

Rebuild the database after changing these.

</details>

<details>
<summary><b>[Attract]</b>, <b>[Weights]</b> and <b>[Attract.X]</b>: attract mode</summary>

| Key | Default | Does |
|:--|:--|:--|
| `PlayTime` | `45-60` | Seconds per game: a number, or a range to pick from |
| `Selection` | `Balanced` | How games are picked (see [How games are picked](#how-games-are-picked)) |
| `NoRepeats` | `true` | Every game once before any repeats |
| `MixSystems` | `true` | Never the same system twice in a row |
| `OneVersion` | `true` | One version per title |
| `SkipTags` | *(empty)* | [Name tags](#name-tags) never to pick |
| `Orientation` | `Both` | Arcade: Both, Horizontal, Vertical, Vertical CW or Vertical CCW |
| `Include` / `Exclude` | *(empty)* | Systems or groups (Exclude wins) |
| `UseStaticDetector` | `true` | Run the static screen detector |
| `OtherInput` | `Play` | What unbound buttons do: Ignore, Play or Stop |
| `Mute` | `false` | Mute the MiSTer while attract mode plays |
| `Playlist` | `Normal` | The attract playlist in use: Normal, or a playlist's name |

`[Weights]` takes `name = number`, for example `SNES = 3`, `Computer = 0.5`, or `Arcade = 0` for never.

`[Attract.X]` uses the same Folders / Files / Extensions / Paths rules as `[Database.X]`, but only for attract mode, with no rebuild needed.

</details>

<details>
<summary><b>[Genres]</b>, <b>[Genres.Files]</b> and <b>[Playlist.X]</b>: genres and attract playlists</summary>

`[Genres]` adds your own folder names for a genre, `Genre = names` or `Genre/Sub-genre = names` (wildcards allowed; a new name makes a new genre). `[Genres.Files]` does the same with game names. Rebuild the database after changing either.

Each attract playlist is a `[Playlist.X]` section:

```ini
[Playlist.Fighters Night]
Name   = Fighters Night
All    = Fighting, Beat 'em Up, *mario*   ; genres for every system
SNES   = Puzzle                           ; extra genres for one system (system ID)
Others = Leave out                        ; systems with no picks: Leave out or As normal
Skip   = *mahjong*, *pachinko*            ; never play games with these in their name or folder
SNES.Skip = *shogi*                       ; a Skip for one system only
Exclude = Computer, NES                   ; systems (or groups) never played
```

| Key | Does |
|:--|:--|
| `Name` | The playlist's name, as shown in the menu |
| `All` | Genres for every system. A custom genre is written as a pattern: `*mario*` |
| *system ID* | Extra genres for that system |
| `Others` | What systems with no picks do: `Leave out` or `As normal` |
| `Skip` | Patterns for games never to play (`*` wildcards, capitals ignored). Words typed in the menu are saved as `*word*` |
| *system ID*`.Skip` | A Skip for one system only (INI only) |
| `Exclude` | Systems or groups this playlist never plays, whatever genres are picked |

</details>

<details>
<summary><b>[List]</b>: Blacklist, Staticlist and Whitelist</summary>

| Key | Default | Does |
|:--|:--|:--|
| `UseBlacklist` | `true` | Never play blacklisted games |
| `UseStaticlist` | `true` | Leave games `SkipafterStatic` seconds after they freeze |
| `SkipafterStatic` | `10` | Seconds |
| `UseWhitelist` | `false` | Only play whitelisted games (for systems with a whitelist) |
| `…Include` / `…Exclude` | *(empty)* | Limit each list to some systems or groups |

</details>

<details>
<summary><b>[StaticDetector]</b>: the static screen detector</summary>

| Key | Default | Does |
|:--|:--|:--|
| `Grace` | `25` | Seconds before anything counts |
| `BlackThreshold` | `30` | Seconds of black screen before it acts |
| `StaticThreshold` | `30` | Seconds of still screen before it acts |
| `SkipBlack` / `SkipStatic` | `true` | Skip to the next game |
| `WriteBlackList` / `WriteStaticList` | `true` | Add the game to the list |
| `MinSpread` | `8` | Grid cells (out of 192) that must change to count as moving |
| `SpreadTime` | `5` | …within this many seconds |
| `DominantColour` | `false` | `-watch` also shows the screen's most common colour (off saves the detector some work every frame) |

Override any of these per system or group, for example `[StaticDetector.PSX]`. A system's own section wins over a group's.

</details>

<details>
<summary><b>[InputDetector]</b> and <b>[InputDetector.Keyboard / Mouse / Joystick]</b>: attract mode controls</summary>

`[InputDetector]` switches each kind of input on or off: `Mouse`, `Keyboard` and `Joystick`, plus the analog sticks: `Sticks` (true/false) and `StickHoldMs` (how long a stick must stay pushed to count, 75 by default).

The other sections take `input = action`, where the actions are `next`, `back`, `play`, `stop`, `blacklist`, `stay`, `menu`, `search`, `favourite`, `mute` and `screenshot`:

```ini
[InputDetector.Joystick]
dpleft  = back
dpright = next
start   = play
back    = stop
```

Use `SAMenu.sh -inputs` to find the names. Controllers the SDL controller database doesn't know use raw names like `btn0` and `axis0+`.

</details>

<details>
<summary><b>[BiosSkip]</b> and <b>[BiosSkip.X]</b>: BIOS skip</summary>

| Key | Default | Does |
|:--|:--|:--|
| `Attract` | `true` | After attract mode launches a game |
| `Menu` | `true` | After you launch a game from SAMenu |

`[BiosSkip.<system ID>]` takes `Sequence = ...`, see [BIOS skip](#bios-skip).

</details>

<details>
<summary><b>[Music]</b>, <b>[Video]</b> and <b>[Startup]</b></summary>

**[Music]**

| Key | Default | Does |
|:--|:--|:--|
| `Playback` | `Random` | Random or In order |
| `Playlist` | *(empty)* | Empty for the music folder itself, `All`, or a folder name |
| `PauseInGames` | `true` | Pause while a game plays |

**[Video]**

| Key | Default | Does |
|:--|:--|:--|
| `Playback` | `Random` | Random or In order |
| `Playlist` | *(empty)* | Empty for the video folder itself, `All`, or a folder name |
| `AttractEvery` | `0` | A video every this many attract games (0 = never) |
| `AutoSync` | `true` | Correct audio/video drift quickly |
| `CorrectPts` | `false` | MP4 and MKV: use the file's own timestamps |
| `Mp3Seek` | `false` | Files with MP3 audio: accurate seeking |
| `AviIndex` | `false` | AVIs without an index: build one first |

**[Startup]**

| Key | Default | Does |
|:--|:--|:--|
| `Start` | `Nothing` | Nothing, SAMenu or Attract mode |
| `Music` | `false` | Start the music player |
| `AttractWhen` | `Instantly` | Instantly, After a delay, or When idle |
| `AttractDelay` | `60` | Seconds, for After a delay |
| `AttractPress` | `Restarts the countdown` | Or Cancels it, or Is ignored |
| `IdleTime` | `5` | Minutes, for When idle |
| `IdleWhere` | `MiSTer menu` | MiSTer menu, Games, or Menu+Games |

After editing these by hand, save once from Options → Startup so the startup block gets updated.

</details>

<details>
<summary><b>[Menu]</b> and <b>[Controls.Menu]</b>: how the menu looks and sorts</summary>

**Labels**

| Key | Default | Does |
|:--|:--|:--|
| `LabelAlign` | `Left` | Left, Center or Right |
| `LabelDivider` | `Space` | Space, Dash, Colon, Pipe or Arrow |
| `LabelAlignDivider` | `false` | Line the dividers up in one column |
| `LabelEncapsulate` | `[ ]` | Around the manufacturer: None, `[ ]`, `( )`, `< >` or `{ }` (also spaced) |
| `ShowManufacturer` | `true` | Off shows just the system's full name |
| `TextSize` | `Auto` | Auto, Normal, Large, Extra large or Huge (Options → Screen) |
| `ScreenMargin` | `Off` | Keep text off edges: Off, 1 line, 2 lines or 3 lines (Options → Screen) |
| `RememberPosition` | `false` | Open where you left off |

**Systems list**

| Key | Default | Does |
|:--|:--|:--|
| `SystemGroup` | `Category` | Manufacturer, Category or None |
| `SystemGroupOrder` | `Custom` | A-Z, Oldest first, or Custom |
| `CategoryOrder` | `Arcade, Console, Handheld, Computer, Other` | The order used by Custom |
| `SystemSubgroup` | `Manufacturer` | A second level of groups |
| `GroupView` | `Folders` | List or Folders |
| `GroupHeaders` | `Off` | With List: Off, Line, Double, Brackets, Dots or Minimal |
| `SystemOrder` | `Release date` | Or A-Z |
| `ArcadeFirst` | `false` | Pin the Arcade Cores to the top |

**Game lists**

| Key | Default | Does |
|:--|:--|:--|
| `FolderPosition` | `First` | First, Last or Mixed |
| `GameOrder` | `Natural` | Alphabetical, or Natural ("Game 2" before "Game 10") |
| `IgnoreThe` | `false` | Sort "The Legend of Zelda" under L |
| `HideExtensions` | `false` | Hide file extensions |
| `GroupDiscs` | `false` | One folder per multi-disc game |
| `VirtualAZFolders` | *(empty)* | Systems that get a `[Games A-Z]` folder |
| `RandomEntry` | `true` | Show `[Pick Random Game]` |
| `HideTags` | *(empty)* | [Name tags](#name-tags) to hide from the lists |
| `SearchHidden` | `true` | Hide them from search too |
| `GenresEntry` | `true` | Show `[Genre Collection]` on the systems list |
| `GenresSystem` | `Before` | In `[Genre Collection]`, each game's system: Before, After or Off |
| `GenresOrder` | `Game name` | In `[Genre Collection]`, the order: Game name, or System |
| `GenresExclude` | *(empty)* | Systems or groups left out of `[Genre Collection]` |

**History and favourites**

| Key | Default | Does |
|:--|:--|:--|
| `HistoryEntry` | `true` | Keep and show `[History]` |
| `HistoryMoveToTop` | `true` | A game played again moves to the top, instead of being listed again |
| `HistoryKeep` | `100` | How many games `[History]` keeps |
| `HistorySystem` | `Before` | Each game's system: Before, After or Off |
| `Favourites` | `true` | The Fav button, the `* ` marks and `[Favourites]` |
| `FavouritesSystem` | `Before` | In `[Favourites]`, each game's system: Before, After or Off |

**[Controls.Menu]** has `Layout = Western` (A confirms) or `Japanese` (B confirms), and the menu's actions, each a list of inputs: keys (`tab`, `space`, a letter…) or `pad:<name>` for a controller button MiSTer doesn't pass on as a key (`pad:start`, `pad:back`…):

```ini
[Controls.Menu]
Layout    = Western
Search    = tab              ; systems list
Options   = space            ; systems list
Favourite = tab              ; game lists
Remove    = space, pad:back  ; [Favourites]
PrevLetter = [, pad:lefttrigger   ; any list
NextLetter = ], pad:righttrigger  ; any list
```

**[Controls.BackToMenu]** is the [Back to Menu](#back-to-menu) hotkey: `Enabled` (true/false, off by default), `Buttons` (one or two controller buttons, named as in [BIOS skip](#bios-skip): `start`, `select`, `a`…), `HoldTime` (seconds), `Key` (a keyboard key pressed twice, in quotes as `` ` `` means something in INI files; empty for none), `WorksIn` (`Games started by SAMenu`, `Games`, `MiSTer menu` or `Anywhere`, see [Back to Menu](#back-to-menu)), `Target` (`SAMenu`, `MiSTer menu` or `Script`) and `Script` (the script's full path, for `Target = Script`):

```ini
[Controls.BackToMenu]
Enabled  = true
Buttons  = start, select
HoldTime = 1.5
Key      = "`"
WorksIn  = Games started by SAMenu
Target   = SAMenu
Script   =
```

</details>

### Name tags

`SkipTags` (attract mode) and `HideTags` (the menu) take these tags, which you can also tick in a list in the menu:

| Tag | Matches |
|:--|:--|
| Beta | `(Beta)`, `(Beta 2)` |
| Proto | `(Proto)`, `(Prototype)` |
| Demo | `(Demo)`, `(Sample)`, `(Kiosk)`, `(Promo)`, `(Preview)` |
| Program | `(Program)`, `(Test Program)`, `[BIOS]` |
| Hack | `(Hack)`, `[h]`, `[h1]` |
| Translation | `[T+Eng]`, `[T-Eng]`, and `T+Eng` without brackets |
| Unlicensed | `(Unl)`, `(Pirate)`, `(Bootleg)` |
| Homebrew | `(Homebrew)`, `(Aftermarket)` |
| Cheat / Trainer / Fixed | `[c]`, `[t]`, `[f]` |
| Bad dump | `[b]`, `[b1]`, `[o]` |
| Alternate | `[a]`, `[a1]` |
| Disc 2+ | `(Disc 2)` and up, `(Side B)`, `(Tape 2)` |
| Re-releases | `(Virtual Console)`, `(Switch Online)`, `(Collection)`, `(Mini)` |

Tags only match inside `( )` or `[ ]`, as whole words, so **Demo** never catches *Demolition Man*.

---

## Systems and groups

SAMenu knows **114 systems**. Their IDs (like `SNES`, `MegaDrive`, `PSX` and `Arcade`) are listed at the top of `SAMenu.ini`.

Anywhere a system ID is accepted (Include, Exclude, Weights, the list and detector sections, `-attract` and `-random`), you can also use a **group**:

- **Categories**: `Console`, `Handheld`, `Computer`, `Arcade` and `Other`
- **Manufacturers**: `Nintendo`, `Sega`, `Sony`, `Atari`, `Commodore`, `SNK`, `NEC` and many more, all listed in `SAMenu.ini`

For example, `Include = Nintendo, Sega` with `Exclude = Gameboy2P` plays every Nintendo and Sega system except the two-player Game Boy core.

---

## Files and folders

| Where | What |
|:--|:--|
| `/media/fat/Scripts/SAMenu.sh` | The program |
| `/media/fat/Scripts/.MiSTer_SAMenu/SAMenu.ini` | The settings |
| `/media/fat/Scripts/.MiSTer_SAMenu/games.db` | The games database |
| `/media/fat/Scripts/.MiSTer_SAMenu/Lists/` | The Blacklist, Staticlist and Whitelist |
| `/media/fat/Scripts/.MiSTer_SAMenu/position.txt` | Where you left off (Remember position) |
| `/media/fat/Scripts/.MiSTer_SAMenu/history.json` | `[History]` |
| `/media/fat/Scripts/.MiSTer_SAMenu/favourites.json` | `[Favourites]` |
| `/media/fat/Scripts/.MiSTer_SAMenu/attract_muted` | Only while attract mode has the sound muted, so it can be undone after a restart |
| `/media/fat/music/` | Music (folders are playlists) |
| `/media/fat/video/` | Videos (folders are playlists) |
| `/tmp/SAMenu_attract.log` | Attract mode's log, when started in the background (menu, idle watcher, startup, `-bg`) |
| `/tmp/SAMenu_idle.log`, `SAMenu_boot.log`, `SAMenu_biosskip.log`, `SAMenu_backtomenu.log`, `SAMenu_music.log`, `SAMenu_video.log` | The background watcher's (idle time, and the Back to Menu hotkey when it works beyond games started by SAMenu), startup's, BIOS skip's, Back to Menu's (in games the menu launched) and the players' logs (each starts again at 512 KB). All of them are in **Options → Logs** |
| `/tmp/SAMenu_attract.status` | What attract mode is playing |
| `/tmp/SAMenu_attract_history.json` | What attract mode has played this session (`[History]` > Attract) |
| `/tmp/SAMenu_detector` | The static detector's live status, written only while `-watch` runs |

To use a different settings file, set the environment variable `SAMENU_CONFIG`.

### Easy on the SD card

SD cards wear out with repeated writing, so SAMenu keeps its busy work in **RAM** (`/tmp`, which vanishes at power-off):

- **Launching:** each game gets a small MGL file generated in RAM (`/tmp/.LASTLAUNCH.mgl`), not saved to the card.
- **Attract mode:** its log, status, history, command pipes and locks are all in RAM. The static detector's live status is written (up to 10 times a second, in RAM) only while `-watch` is showing it.
- **Music and video:** status and command files in RAM, and MPlayer is unpacked into RAM once per boot.
- **Searching and browsing:** the games database is read once and kept in memory, so browsing and searching never touch the card.

**What does get written to the SD card**, and only when it has to be:

- **`SAMenu.ini`**, when you change a setting.
- **`games.db`**, when you rebuild the database. It's written once, to a temporary file that's swapped in when complete.
- **The lists**, when the static detector or the blacklist action adds a game (one short line).
- **`position.txt`**, when you launch a game or leave the menu, with Remember position on.
- **`history.json`**, when you launch a game from SAMenu, and **`favourites.json`**, when you mark or remove a favourite.
- **`attract_muted`**, a tiny marker, only when attract mode mutes the sound (removed when it unmutes).

**Capped:** attract mode, which can run for hours, limits its memory to **128 MB** (of the MiSTer's roughly 500 MB for Linux), keeps a history of the last **100** games, and starts a **fresh log** each session.

**Light on the processor**, so the games run as they should:

- Attract mode keeps only the games it can play in memory, letting the rest of the database go once it's started.
- The static detector reads a fixed number of pixels per frame (up to about 5,000), whatever the core's resolution, and always rests between frames.
- Input detectors only run while something needs them: the menu's only for controller buttons mapped to its actions (and the Input test), and the background watcher's only while it's counting idle time or watching for the Back to Menu hotkey.

---

## Building from source

SAMenu is written in Go and uses ncurses, so it's built as a static ARM binary with a cross-compiler. From Linux or WSL:

```bash
# one-time setup
sudo apt install gcc-arm-linux-gnueabihf libncurses-dev:armhf

# build
CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc \
PKG_CONFIG_LIBDIR=/usr/lib/arm-linux-gnueabihf/pkgconfig \
go build -trimpath -tags netgo,osusergo \
  -ldflags '-s -w -linkmode external -extldflags "-static -lncurses -ltinfo"' \
  -o SAMenu.sh ./cmd/samenu
```

On Windows, `build_deploy_SAMenu.bat` runs the build in WSL and copies the result to the MiSTer. Set your MiSTer's IP at the top; it also needs `sshpass`. The other `.bat` files run things on the MiSTer over SSH: `start_attract.bat`, `watch_detector.bat`, `test_inputs.bat` and `test_press.bat`.

---

## Credits

- [**MiSTer SAM**](https://github.com/mrchrisster/MiSTer_SAM) by mrchrisster and contributors: the original Super Attract Mode, and the inspiration for SAMenu.
- The [**MiSTer FPGA project**](https://github.com/MiSTer-devel) and all its core developers.
- [**MPlayer**](http://www.mplayerhq.hu/) for video playback, and the [**SDL GameController DB**](https://github.com/mdqinc/SDL_GameControllerDB) for controller names.
