package config

import (
	"fmt"
	"os"
	"os/user"

	"github.com/adrg/xdg"
	"gitlab.com/tslocum/cbind"
	"gopkg.in/ini.v1"
)

var configFilePath string
var keyConfig *cbind.Configuration
var cfg *ini.File

type IniFile struct {
	*General
	*Keymap
	*Ui
	*Colors
}

type General struct {
	DownloadPath        string
	PreviewPath         string
	CmdPrefix           string
	EnableNotifications bool
	NotificationTimeout int64
}

type Keymap struct {
	SwitchPanels    string
	FocusMessages   string
	FocusInput      string
	FocusContacts   string
	CommandBacklog  string
	CommandConnect  string
	CommandQuit     string
	CommandHelp     string
	MessageDownload string
	MessageOpen     string
	MessageShow     string
	MessageInfo     string
	MessageRevoke   string
}

type Ui struct {
	ContactSidebarWidth int
}

type Colors struct {
	Background      string
	Text            string
	ListHeader      string
	ListContact     string
	ListGroup       string
	ChatContact     string
	ChatMe          string
	Borders         string
	InputBackground string
	InputText       string
	Positive        string
	Negative        string
}

var Config = IniFile{
	&General{
		DownloadPath:        GetHomeDir() + "Downloads",
		PreviewPath:         GetHomeDir() + "Downloads",
		CmdPrefix:           "/",
		EnableNotifications: false,
		NotificationTimeout: 60,
	},
	&Keymap{
		SwitchPanels:    "Tab",
		FocusMessages:   "Ctrl+w",
		FocusInput:      "Ctrl+Space",
		FocusContacts:   "Ctrl+e",
		CommandBacklog:  "Ctrl+b",
		CommandConnect:  "Ctrl+r",
		CommandQuit:     "Ctrl+q",
		CommandHelp:     "Ctrl+?",
		MessageDownload: "d",
		MessageInfo:     "i",
		MessageOpen:     "o",
		MessageRevoke:   "r",
		MessageShow:     "s",
	},
	&Ui{
		ContactSidebarWidth: 30,
	},
	&Colors{
		Background:      "black",
		Text:            "white",
		ListHeader:      "yellow",
		ListContact:     "green",
		ListGroup:       "blue",
		ChatContact:     "green",
		ChatMe:          "blue",
		Borders:         "white",
		InputBackground: "blue",
		InputText:       "white",
		Positive:        "green",
		Negative:        "red",
	},
}

func InitConfig() {
	var err error
	if configFilePath, err = xdg.ConfigFile("whatscli/whatscli.config"); err == nil {
		// add any new values
		var cfg *ini.File
		if cfg, err = ini.Load(configFilePath); err == nil {
			cfg.NameMapper = ini.TitleUnderscore
			cfg.ValueMapper = os.ExpandEnv
			if section, err := cfg.GetSection("general"); err == nil {
				section.MapTo(&Config.General)
			}
			if section, err := cfg.GetSection("keymap"); err == nil {
				section.MapTo(&Config.Keymap)
			}
			if section, err := cfg.GetSection("ui"); err == nil {
				section.MapTo(&Config.Ui)
			}
			if section, err := cfg.GetSection("colors"); err == nil {
				section.MapTo(&Config.Colors)
			}
			newCfg := ini.Empty()
			if err = ini.ReflectFromWithMapper(newCfg, &Config, ini.TitleUnderscore); err == nil {
				//TODO: only save if changes
				err = newCfg.SaveTo(configFilePath)
			}
		} else {
			cfg = ini.Empty()
			cfg.NameMapper = ini.TitleUnderscore
			cfg.ValueMapper = os.ExpandEnv
			if err = ini.ReflectFromWithMapper(cfg, &Config, ini.TitleUnderscore); err == nil {
				err = cfg.SaveTo(configFilePath)
			}
		}
	}
	if err != nil {
		fmt.Printf(err.Error())
	}
}

func GetConfigFilePath() string {
	return configFilePath
}

func GetSessionFilePath() string {
	if sessionFilePath, err := xdg.ConfigFile("whatscli/session"); err == nil {
		return sessionFilePath
	}
	return GetHomeDir() + ".whatscli.session"
}

func GetContactsFilePath() string {
	if sessionFilePath, err := xdg.ConfigFile("whatscli/contacts"); err == nil {
		return sessionFilePath
	}
	return GetHomeDir() + ".whatscli.contacts"
}

// gets the OS home dir with a path separator at the end
func GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
	}
	return usr.HomeDir + string(os.PathSeparator)
}
