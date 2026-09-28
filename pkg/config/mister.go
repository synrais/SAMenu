package config

// TODO: should this be hardcoded? how common is usb0 setup?
const SdFolder = "/media/fat"
const CoreConfigFolder = SdFolder + "/config"

const TempFolder = "/tmp"
const LinuxFolder = SdFolder + "/linux"
const ScriptsFolder = SdFolder + "/Scripts"

const StartupFile = LinuxFolder + "/user-startup.sh"

const CoreNameFile = TempFolder + "/CORENAME"

const MenuCore = "MENU"

const CmdInterface = "/dev/MiSTer_cmd"

// TODO: not the order mister actually checks, it does games folders second, but this is simpler for checking prefix
var GamesFolders = []string{
	"/media/usb0/games",
	"/media/usb0",
	"/media/usb1/games",
	"/media/usb1",
	"/media/usb2/games",
	"/media/usb2",
	"/media/usb3/games",
	"/media/usb3",
	"/media/usb4/games",
	"/media/usb4",
	"/media/usb5/games",
	"/media/usb5",
	"/media/network/games",
	"/media/network",
	"/media/fat/cifs/games",
	"/media/fat/cifs",
	"/media/fat/games",
	"/media/fat",
}
