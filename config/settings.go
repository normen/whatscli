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

func InitConfig() {
	var err error
	if configFilePath, err = xdg.ConfigFile("whatscli/whatscli.config"); err == nil {
		if cfg, err = ini.Load(configFilePath); err == nil {
			//TODO: check config for new parameters
		} else {
			cfg = ini.Empty()
			cfg.NewSection("general")
			cfg.Section("general").NewKey("download_path", GetHomeDir()+"Downloads")
			cfg.Section("general").NewKey("preview_path", GetHomeDir()+"Downloads")
			cfg.NewSection("keymap")
			cfg.Section("keymap").NewKey("switch_panels", "Tab")
			cfg.Section("keymap").NewKey("focus_messages", "Ctrl+w")
			cfg.Section("keymap").NewKey("focus_input", "Ctrl+Space")
			cfg.Section("keymap").NewKey("focus_contacts", "Ctrl+e")
			cfg.Section("keymap").NewKey("command_connect", "Ctrl+r")
			cfg.Section("keymap").NewKey("command_quit", "Ctrl+q")
			cfg.Section("keymap").NewKey("command_help", "Ctrl+?")
			cfg.Section("keymap").NewKey("message_download", "d")
			cfg.Section("keymap").NewKey("message_open", "o")
			cfg.Section("keymap").NewKey("message_show", "s")
			cfg.Section("keymap").NewKey("message_info", "i")
			cfg.NewSection("ui")
			cfg.Section("ui").NewKey("contact_sidebar_width", "30")
			cfg.NewSection("colors")
			cfg.Section("colors").NewKey("background", "black")
			cfg.Section("colors").NewKey("text", "white")
			cfg.Section("colors").NewKey("list_header", "yellow")
			cfg.Section("colors").NewKey("list_contact", "green")
			cfg.Section("colors").NewKey("list_group", "blue")
			cfg.Section("colors").NewKey("chat_contact", "green")
			cfg.Section("colors").NewKey("chat_me", "blue")
			err = cfg.SaveTo(configFilePath)
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
	return GetHomeDir() + ".whatscli.session"
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
