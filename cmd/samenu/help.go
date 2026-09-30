package main

import (
	"flag"
	"fmt"
)

// helpText is SAMenu.sh -h: the options, grouped as in the README. The
// internal ones (-musicd, -videod, -boot, -idlewatch, -openmenu,
// -autoinput, -backtomenu), which SAMenu uses to start its own background parts, are
// left out. Add a new option here and in the README's table too.
const helpText = `Usage: SAMenu.sh [option]

With no option, SAMenu opens.

Attract mode
  -attract                     Start attract mode (its log on screen)
  -attract -bg                 Start it in the background (log in /tmp/SAMenu_attract.log)
  -attract SNES,Console        Only these systems or groups, this time
  -attract -playlist "Name"    Use this attract playlist, this run (Normal = the usual setup)
  -find mario 3                Play the games matching all the words first, then carry on
                               (starts attract mode if it isn't running)
  -status                      Is it running, and what's playing?
  -next, -back                 Next or previous game
  -play                        Stop attract mode and keep playing this game
  -stay                        Stay on this game (again to carry on)
  -blacklist                   Never play this game again, next game
  -stop                        Stop attract mode and go back to the MiSTer menu

SAMenu on the TV, or here
  -menu                        Open SAMenu on the TV (closes the running game)
  -search                      ...straight into its Search screen
  -menu -here, -search -here   Open it in this terminal instead (e.g. over SSH with ssh -t)

Games
  -launch <file>               Launch a game, e.g. -launch /media/fat/games/NES/Tetris.nes
  -launch <file> -system ID    ...as this system (ID or name), instead of guessing from the file
  -launch <file> -core CORE    ...with another core, e.g. -core _Unstable/NES
  -random                      Launch a random game
  -random Nintendo             ...from these systems or groups
  -list                        Print every game in the database
  -rebuild                     Rebuild the games database
  -genres                      Print the genres found in the database, with counts per system

Music and video
  -music start|stop|next|previous|status
                               The music player
  -video play [file|folder|playlist|-]
                               The video player (- plays a video piped in)
  -video next|stop|status      Control the video player

Picture (Options -> Screen -> Picture)
  -screen status               The picture settings in use, and any temporary ones
  -screen undo                 Take off temporary picture settings (as a reboot would)
  -screen restore              Also put MiSTer.ini back as it was before SAMenu changed it

Testing
  -inputs                      Print every key, click and controller press
  -press start                 Press buttons on the virtual pad or keyboard
  -watch                       Show the static detector's live status

-launch, -random and -video play end a running attract mode first. The
other options never disturb a running menu or attract mode.
`

func init() {
	flag.Usage = func() { fmt.Fprint(flag.CommandLine.Output(), helpText) }
}
