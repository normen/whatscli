//this package manages the messages
package messages

import "io"

// TODO: move these funcs/interface to channels
type UiMessageHandler interface {
	NewMessage(Message)
	NewScreen([]Message)
	SetChats([]Chat)
	PrintError(error)
	PrintText(string)
	PrintFile(string)
	SetStatus(SessionStatus)
	OpenFile(string)
	GetWriter() io.Writer
}

// data struct for current session status
type SessionStatus struct {
	BatteryCharge    int
	BatteryLoading   bool
	BatteryPowersave bool
	Connected        bool
	LastSeen         string
}

// message struct for battery messages
type BatteryMsg struct {
	charge    int
	loading   bool
	powersave bool
}

// message struct for status messages
type StatusMsg struct {
	connected bool
	err       error
}

// message object for commands
type Command struct {
	Name   string
	Params []string
}

// internal message representation to abstract from message lib
type Message struct {
	Id           string
	ChatId       string // the source of the message (group id or contact id)
	ContactId    string
	ContactName  string
	ContactShort string
	Timestamp    uint64
	FromMe       bool
	Forwarded    bool
	Text         string
	Orig         interface{}
}

// internal contact representation to abstract from message lib
type Chat struct {
	Id      string
	IsGroup bool
	Name    string
	Unread  int
	//TODO: convert to uint64
	LastMessage int64
}

type Contact struct {
	Id    string
	Name  string
	Short string
}

//TODO: whatsapp-specific
const GROUPSUFFIX = "@g.us"

//TODO: whatsapp-specific
const CONTACTSUFFIX = "@s.whatsapp.net"

//TODO: whatsapp-specific
const STATUSSUFFIX = "status@broadcast"
