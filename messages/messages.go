//this package manages the messages
package messages

import "io"

// TODO: move these funcs/interface to channels
type UiMessageHandler interface {
	NewMessage(Message)
	NewScreen([]Message)
	SetContacts([]Contact)
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
	SourceId     string // the source of the message (group id or contact id)
	ContactId    string
	ContactName  string
	ContactShort string
	Timestamp    uint64
	FromMe       bool
	Text         string
}

// internal contact representation to abstract from message lib
type Contact struct {
	Id      string
	IsGroup bool
	Name    string
}

const GROUPSUFFIX = "@g.us"
const CONTACTSUFFIX = "@s.whatsapp.net"
const STATUSSUFFIX = "status@broadcast"
