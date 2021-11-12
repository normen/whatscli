package config

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	"github.com/gdamore/tcell/v2"
	"gopkg.in/ini.v1"
)

var configFilePath string
var cfg *ini.File

// IniFile is the raw mapping of an .ini file.
// All of the struct fields have primitive types supported by the .ini format (like int, bool, string, etc.).
type IniFile struct {
	*General
	*Keymap
	*Ui
	*IniColors
}

// AppConfig is the application-level configuration.
// Its struct values have richer types (tcell.Color rather than string, for example).
type AppConfig struct {
	*General
	*Keymap
	*Ui
	*Colors
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

// IniColors represents the raw color config values as strings in an .ini file.
type IniColors struct {
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

// Color is a wrapper around tcell.Color so we can add a HexCode() method.
type Color struct {
	TColor tcell.Color
}

// HexCode returns the color as a #rrggbb string.
func (c *Color) HexCode() string {
	return fmt.Sprintf("#%x", c.TColor.Hex())
}

// Colors represents colors to be used by whatscli.
type Colors struct {
	Background      Color
	Text            Color
	ForwardedText   Color
	ListHeader      Color
	ListContact     Color
	ListGroup       Color
	ChatContact     Color
	ChatMe          Color
	Borders         Color
	InputBackground Color
	InputText       Color
	UnreadCount     Color
	Positive        Color
	Negative        Color
}

// BadColorError is returned when a string cannot be converted to a tcell.Color.
type BadColorError struct {
	color string
}

func (e *BadColorError) Error() string {
	return fmt.Sprintf("Bad color string '%s'", e.color)
}

func parseColor(color string) (Color, error) {
	tColor := tcell.GetColor(color) // if this fails, it's tcell.ColorDefault

	if len(color) == 7 && color[0] == '#' {
		// hex
		tColor = tcell.GetColor(color)
	} else if len(color) == 8 && color[0:2] == "0x" {
		// hex but with 0x as a prefix
		tColor = tcell.GetColor("#" + color[2:])
	} else if len(strings.Split(color, ",")) == 3 {
		// R, G, B (where R is a 3-digit number)
		rgbStr := strings.Split(color, ",")
		rgb := make([]int32, 3)
		for i, v := range rgbStr {
			v, err := strconv.ParseInt(strings.TrimSpace(v), 10, 32)
			if err != nil {
				return Color{tColor}, &BadColorError{color}
			}
			rgb[i] = int32(v)
		}
		tColor = tcell.NewRGBColor(rgb[0], rgb[1], rgb[2])
	}

	if tColor == tcell.ColorDefault {
		return Color{tColor}, &BadColorError{color}
	}
	return Color{tColor}, nil
}

func (c *Colors) loadFrom(iniColors IniColors) error {
	// TODO: use reflect package to do this in a more future-proof way.
	type Failure struct {
		field string
		err   string
	}
	errors := []Failure{}
	var err error

	c.Background, err = parseColor(iniColors.Background)
	if err != nil {
		errors = append(errors, Failure{"background", err.Error()})
	}

	c.Text, err = parseColor(iniColors.Text)
	if err != nil {
		errors = append(errors, Failure{"text", err.Error()})
	}

	c.ForwardedText, err = parseColor(iniColors.ForwardedText)
	if err != nil {
		errors = append(errors, Failure{"forwarded_text", err.Error()})
	}

	c.ListHeader, err = parseColor(iniColors.ListHeader)
	if err != nil {
		errors = append(errors, Failure{"list_header", err.Error()})
	}

	c.ListContact, err = parseColor(iniColors.ListContact)
	if err != nil {
		errors = append(errors, Failure{"list_contact", err.Error()})
	}

	c.ListGroup, err = parseColor(iniColors.ListGroup)
	if err != nil {
		errors = append(errors, Failure{"list_group", err.Error()})
	}

	c.ChatContact, err = parseColor(iniColors.ChatContact)
	if err != nil {
		errors = append(errors, Failure{"chat_contact", err.Error()})
	}

	c.ChatMe, err = parseColor(iniColors.ChatMe)
	if err != nil {
		errors = append(errors, Failure{"chat_me", err.Error()})
	}

	c.Borders, err = parseColor(iniColors.Borders)
	if err != nil {
		errors = append(errors, Failure{"borders", err.Error()})
	}

	c.InputBackground, err = parseColor(iniColors.InputBackground)
	if err != nil {
		errors = append(errors, Failure{"input_background", err.Error()})
	}

	c.InputText, err = parseColor(iniColors.InputText)
	if err != nil {
		errors = append(errors, Failure{"input_text", err.Error()})
	}

	c.UnreadCount, err = parseColor(iniColors.UnreadCount)
	if err != nil {
		errors = append(errors, Failure{"unread_count", err.Error()})
	}

	c.Positive, err = parseColor(iniColors.Positive)
	if err != nil {
		errors = append(errors, Failure{"positive", err.Error()})
	}

	c.Negative, err = parseColor(iniColors.Negative)
	if err != nil {
		errors = append(errors, Failure{"negative", err.Error()})
	}

	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("Error parsing colors: %+v", errors)
}

// Config is the global object representing the user's configuration.
var Config = AppConfig{
	&General{
		DownloadPath:        GetHomeDir() + "Downloads",
		PreviewPath:         GetHomeDir() + "Downloads",
		CmdPrefix:           "/",
		ShowCommand:         "jp2a --color",
		EnableNotifications: false,
		UseTerminalBell:     false,
		NotificationTimeout: 60,
		BacklogMsgQuantity:  10,
	},
	&Keymap{
		SwitchPanels:    "Tab",
		FocusMessages:   "Ctrl+w",
		FocusInput:      "Ctrl+Space",
		FocusChats:      "Ctrl+e",
		CommandBacklog:  "Ctrl+b",
		CommandRead:     "Ctrl+n",
		Copyuser:        "Ctrl+c",
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
		Background:      Color{tcell.ColorBlack},
		Text:            Color{tcell.ColorWhite},
		ForwardedText:   Color{tcell.ColorPurple},
		ListHeader:      Color{tcell.ColorYellow},
		ListContact:     Color{tcell.ColorGreen},
		ListGroup:       Color{tcell.ColorBlue},
		ChatContact:     Color{tcell.ColorGreen},
		ChatMe:          Color{tcell.ColorBlue},
		Borders:         Color{tcell.ColorWhite},
		InputBackground: Color{tcell.ColorBlue},
		InputText:       Color{tcell.ColorWhite},
		UnreadCount:     Color{tcell.ColorYellow},
		Positive:        Color{tcell.ColorGreen},
		Negative:        Color{tcell.ColorRed},
	},
}

// InitConfig initializes the global UserConfig object.
func InitConfig() error {
	var err error
	configFilePath, err = xdg.ConfigFile("whatscli/whatscli.config")
	if err != nil {
		return err
	}

	// add any new values
	var cfg *ini.File
	cfg, err = ini.LoadSources(ini.LoadOptions{UnescapeValueDoubleQuotes: true}, configFilePath)
	if err != nil {
		// Couldn't load config file. Save the default config to the filepath
		cfg = ini.Empty()
		cfg.NameMapper = ini.TitleUnderscore
		cfg.ValueMapper = os.ExpandEnv
		err = ini.ReflectFromWithMapper(cfg, &Config, ini.TitleUnderscore)
		if err != nil {
			return err
		}

		return cfg.SaveTo(configFilePath)
	}

	cfg.NameMapper = ini.TitleUnderscore
	cfg.ValueMapper = os.ExpandEnv
	var section *ini.Section

	// Load values from .ini file into global variable Config.
	if section, err = cfg.GetSection("general"); err == nil {
		section.MapTo(&Config.General)
	}
	if section, err = cfg.GetSection("keymap"); err == nil {
		section.MapTo(&Config.Keymap)
	}
	if section, err = cfg.GetSection("ui"); err == nil {
		section.MapTo(&Config.Ui)
	}
	if section, err = cfg.GetSection("colors"); err == nil {
		rawColorConfig := IniColors{}
		section.MapTo(&rawColorConfig)
		err = Config.Colors.loadFrom(rawColorConfig)
	}
	//TODO: only save if changes
	//newCfg := ini.Empty()
	//if err = ini.ReflectFromWithMapper(newCfg, &Config, ini.TitleUnderscore); err == nil {
	//err = newCfg.SaveTo(configFilePath)
	//}

	return err
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

// GetHomeDir the OS home dir with a path separator at the end
func GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
	}
	return usr.HomeDir + string(os.PathSeparator)
}
