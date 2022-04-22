//this package manages the messages
package messages

import "io"

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

type Backend interface {
	Start(chan interface{}) error
	Stop() error
	Command(string, []string) error
	Download(*Message, string) error
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
	Link         string
	MessageType  string
	MediaLink    string
	MediaData1   []byte
	MediaData2   []byte
	MediaData3   []byte
}

type Chat struct {
	Id          string
	IsGroup     bool
	Name        string
	Unread      int
	LastMessage uint64
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
