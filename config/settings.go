package config

import (
	"fmt"
	"os"
	"os/user"

	"github.com/adrg/xdg"
	"github.com/gdamore/tcell/v2"
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
	DownloadPath string
	PreviewPath  string
	CmdPrefix    string
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

func InitConfig() {
	defaultCfg := &IniFile{
		&General{
			DownloadPath: GetHomeDir() + "Downloads",
			PreviewPath:  GetHomeDir() + "Downloads",
			CmdPrefix:    "/",
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
	var err error
	if configFilePath, err = xdg.ConfigFile("whatscli/whatscli.config"); err == nil {
		// add any new values
		if err = ini.MapToWithMapper(*defaultCfg, ini.TitleUnderscore, configFilePath); err == nil {
			cfg = ini.Empty()
			if err = ini.ReflectFromWithMapper(cfg, defaultCfg, ini.TitleUnderscore); err == nil {
				err = cfg.SaveTo(configFilePath)
			}
		} else {
			cfg = ini.Empty()
			if err = ini.ReflectFromWithMapper(cfg, defaultCfg, ini.TitleUnderscore); err == nil {
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

func GetKey(name string) string {
	if sec, err := cfg.GetSection("keymap"); err == nil {
		if key, err := sec.GetKey(name); err == nil {
			return key.String()
		}
	}
	return ""
}

func GetColorName(key string) string {
	if sec, err := cfg.GetSection("colors"); err == nil {
		if key, err := sec.GetKey(key); err == nil {
			return key.String()
		}
	}
	return "white"
}

func GetColor(key string) tcell.Color {
	name := GetColorName(key)
	if color, ok := tcell.ColorNames[name]; ok {
		return color
	}
	return tcell.ColorWhite
}

func GetSetting(name string) string {
	if sec, err := cfg.GetSection("general"); err == nil {
		if key, err := sec.GetKey(name); err == nil {
			return key.String()
		}
	}
	return ""
}

func GetIntSetting(section string, name string) int {
	if sec, err := cfg.GetSection(section); err == nil {
		if key, err := sec.GetKey(name); err == nil {
			if val, err := key.Int(); err == nil {
				return val
			}
		}
	}
	return 0
}

// gets the OS home dir with a path separator at the end
func GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
	}
	return usr.HomeDir + string(os.PathSeparator)
}
