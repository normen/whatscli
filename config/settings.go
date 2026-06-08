package config

import (
	"fmt"
	"os"
	"os/user"

	"github.com/adrg/xdg"
	"gopkg.in/ini.v1"
)

var configFilePath string
var cfg *ini.File

type IniFile struct {
	*General
	*Keymap
	*Ui
	*Colors
	*Bot
}

type General struct {
	DownloadPath        string
	PreviewPath         string
	CmdPrefix           string
	ShowCommand         string
	EnableNotifications bool
	UseTerminalBell     bool
	NotificationTimeout int64
	BacklogMsgQuantity  int
}

type Keymap struct {
	SwitchPanels    string
	FocusMessages   string
	FocusInput      string
	FocusChats      string
	FindChats       string
	Copyuser        string
	Pasteuser       string
	CommandBacklog  string
	CommandRead     string
	CommandConnect  string
	CommandQuit     string
	CommandHelp     string
	MessageDownload string
	MessageOpen     string
	MessageShow     string
	MessageUrl      string
	MessageInfo     string
	MessageRevoke   string
}

type Ui struct {
	ChatSidebarWidth int
}

type Colors struct {
	Background      string
	Text            string
	ForwardedText   string
	ListHeader      string
	ListContact     string
	ListGroup       string
	ChatContact     string
	ChatMe          string
	Borders         string
	InputBackground string
	InputText       string
	UnreadCount     string
	Positive        string
	Negative        string
}

// Bot holds the configuration for the optional AI auto-reply bot.
type Bot struct {
	// Enabled turns the auto-reply bot on or off.
	Enabled bool
	// ChatId is the only chat the bot will reply in (e.g. "5511999999999@s.whatsapp.net").
	// For a group, use the group JID (ends with "@g.us").
	ChatId string
	// TriggerPrefix, when non-empty, makes the bot reply ONLY to messages that
	// start with this prefix (e.g. "@"). The prefix is stripped before the text
	// is sent to the model. Leave empty to reply to every message in the chat.
	TriggerPrefix string
	// Model is the OpenAI chat model to use (e.g. "gpt-4o-mini").
	Model string
	// SystemPrompt is the instruction that defines the bot's behaviour/persona.
	SystemPrompt string
	// MemoryMessages is how many of the most recent messages of the chat are
	// sent to the model as conversation context. The model only "remembers"
	// this many messages; anything older is not considered.
	MemoryMessages int
	// BoostModel is the model used when a user supplies their own API key
	// (via the "/key" chat command), giving them a larger context window.
	BoostModel string
	// BoostMemoryMessages is the (larger) memory window used when a user
	// supplies their own API key.
	BoostMemoryMessages int
	// The default OpenAI API key is read from the OPENAI_API_KEY environment variable, not from here.
}

var Config = IniFile{
	&General{
		DownloadPath:        GetHomeDir() + "Downloads",
		PreviewPath:         GetHomeDir() + "Downloads",
		CmdPrefix:           "/",
		ShowCommand:         "jp2a --color",
		EnableNotifications: true,
		UseTerminalBell:     true,
		NotificationTimeout: 60,
		BacklogMsgQuantity:  10,
	},
	&Keymap{
		SwitchPanels:    "Tab",
		FocusMessages:   "Ctrl+w",
		FocusInput:      "Ctrl+Space",
		FocusChats:      "Ctrl+e",
		FindChats:       "Ctrl+f",
		CommandBacklog:  "Ctrl+b",
		CommandRead:     "Ctrl+n",
		Copyuser:        "Ctrl+y",
		Pasteuser:       "Ctrl+v",
		CommandConnect:  "Ctrl+r",
		CommandQuit:     "Ctrl+q",
		CommandHelp:     "Ctrl+?",
		MessageDownload: "d",
		MessageInfo:     "i",
		MessageOpen:     "o",
		MessageUrl:      "u",
		MessageRevoke:   "r",
		MessageShow:     "s",
	},
	&Ui{
		ChatSidebarWidth: 30,
	},
	&Colors{
		Background:      "black",
		Text:            "white",
		ForwardedText:   "purple",
		ListHeader:      "yellow",
		ListContact:     "green",
		ListGroup:       "blue",
		ChatContact:     "green",
		ChatMe:          "blue",
		Borders:         "white",
		InputBackground: "blue",
		InputText:       "white",
		UnreadCount:     "yellow",
		Positive:        "green",
		Negative:        "red",
	},
	&Bot{
		Enabled:             true,
		ChatId:              "120363426087525156@g.us",
		TriggerPrefix:       "@",
		Model:               "gpt-4o-mini",
		SystemPrompt:        "You are a helpful WhatsApp assistant. Reply concisely in the same language as the message.",
		MemoryMessages:      10,
		BoostModel:          "gpt-4o",
		BoostMemoryMessages: 50,
	},
}

func InitConfig() {
	// load a project-local .env (if present) before reading any config/env values
	LoadDotEnv()
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
			if section, err := cfg.GetSection("bot"); err == nil {
				section.MapTo(&Config.Bot)
			}
			//TODO: only save if changes
			//newCfg := ini.Empty()
			//if err = ini.ReflectFromWithMapper(newCfg, &Config, ini.TitleUnderscore); err == nil {
			//err = newCfg.SaveTo(configFilePath)
			//}
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
		fmt.Print(err.Error())
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

// gets the OS home dir with a path separator at the end
func GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
	}
	return usr.HomeDir + string(os.PathSeparator)
}
