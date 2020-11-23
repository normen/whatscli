package messages

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/Rhymen/go-whatsapp"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/qrcode"
)

// TODO: move message styling and ordering into UI, don't use strings
type UiMessageHandler interface {
	NewMessage(string, string)
	NewScreen(string, []string)
	SetContacts([]string)
	PrintError(error)
	PrintText(string)
	GetWriter() io.Writer
}

type Command struct {
	Name   string
	Params []string
}

type SendMsg struct {
	Wid  string
	Text string
}

const GROUPSUFFIX = "@g.us"
const CONTACTSUFFIX = "@s.whatsapp.net"

type SessionManager struct {
	db              MessageDatabase
	currentReceiver string // currently selected contact for message handling
	uiHandler       UiMessageHandler
	CommandChannel  chan Command
	TextChannel     chan whatsapp.TextMessage
	OtherChannel    chan interface{}
}

func (sm *SessionManager) Init(handler UiMessageHandler) {
	sm.db = MessageDatabase{}
	sm.db.Init()
	sm.uiHandler = handler
	//TODO: conflate to commandchannel
	sm.CommandChannel = make(chan Command, 10)
	sm.TextChannel = make(chan whatsapp.TextMessage, 10)
	sm.OtherChannel = make(chan interface{}, 10)
}

func (sm *SessionManager) SetCurrentReceiver(id string) {
	sm.currentReceiver = id
	screen, ids := sm.db.GetMessagesString(id)
	sm.uiHandler.NewScreen(screen, ids)
}

// gets an existing connection or creates one
func (sm *SessionManager) GetConnection() *whatsapp.Conn {
	var wac *whatsapp.Conn
	if connection == nil {
		wacc, err := whatsapp.NewConn(5 * time.Second)
		if err != nil {
			return nil
		}
		wac = wacc
		connection = wac
		//wac.SetClientVersion(2, 2021, 4)
	} else {
		wac = connection
	}
	return wac
}

// Login logs in the user. It ries to see if a session already exists. If not, tries to create a
// new one using qr scanned on the terminal.
func (sm *SessionManager) Login() error {
	return sm.LoginWithConnection(sm.GetConnection())
}

// LoginWithConnection logs in the user using a provided connection. It ries to see if a session already exists. If not, tries to create a
// new one using qr scanned on the terminal.
func (sm *SessionManager) LoginWithConnection(wac *whatsapp.Conn) error {
	if wac != nil && wac.GetConnected() {
		wac.Disconnect()
	}
	//load saved session
	session, err := readSession()
	if err == nil {
		//restore session
		session, err = wac.RestoreWithSession(session)
		if err != nil {
			return fmt.Errorf("restoring failed: %v\n", err)
		}
	} else {
		//no saved session -> regular login
		qr := make(chan string)
		go func() {
			terminal := qrcode.New()
			terminal.SetOutput(tview.ANSIWriter(sm.uiHandler.GetWriter()))
			terminal.Get(<-qr).Print()
		}()
		session, err = wac.Login(qr)
		if err != nil {
			return fmt.Errorf("error during login: %v\n", err)
		}
	}

	//save session
	err = writeSession(session)
	if err != nil {
		return fmt.Errorf("error saving session: %v\n", err)
	}
	//<-time.After(3 * time.Second)
	return nil
}

func (sm *SessionManager) Disconnect() error {
	wac := sm.GetConnection()
	if wac != nil && wac.GetConnected() {
		_, err := wac.Disconnect()
		return err
	}
	return nil
}

// Logout logs out the user.
func (ub *SessionManager) Logout() error {
	return removeSession()
}

func (sm *SessionManager) ExecCommand(command Command) {
	sndTxt := command.Name
	switch sndTxt {
	default:
	case "backlog":
		//command
		if sm.currentReceiver == "" {
			return
		}
		count := 10
		if currentMsgs, ok := sm.db.textMessages[sm.currentReceiver]; ok {
			if len(currentMsgs) > 0 {
				firstMsg := currentMsgs[0]
				go sm.GetConnection().LoadChatMessages(sm.currentReceiver, count, firstMsg.Info.Id, firstMsg.Info.FromMe, false, sm)
			}
		}
	//FullChatHistory(currentReceiver, 20, 100000, handler)
	//messages.GetConnection().LoadFullChatHistory(currentReceiver, 20, 100000, handler)
	case "login":
		//command
		sm.Login()
	case "disconnect":
		//TODO: output error
		sm.uiHandler.PrintError(sm.Disconnect())
	case "logout":
		//command
		//TODO: output error
		sm.uiHandler.PrintError(sm.Logout())
	case "send_message":
		sm.SendText(command.Params[0], command.Params[1])
	case "select_contact":
		sm.SetCurrentReceiver(command.Params[0])
	}
}

// load data for message specified by message id TODO: support types
func (sm *SessionManager) LoadMessageData(wid string) ([]byte, error) {
	if msg, ok := sm.db.otherMessages[wid]; ok {
		switch v := (*msg).(type) {
		default:
		case whatsapp.ImageMessage:
			return v.Download()
		case whatsapp.DocumentMessage:
			//return v.Download()
		case whatsapp.AudioMessage:
			//return v.Download()
		case whatsapp.VideoMessage:
			//return v.Download()
		}
	}
	return []byte{}, errors.New("This is not an image message")
}

// attempts to download a messages attachments, returns path or error message
func (sm *SessionManager) DownloadMessage(wid string, preview bool) (string, error) {
	if msg, ok := sm.db.otherMessages[wid]; ok {
		var fileName string = ""
		if preview {
			fileName += config.GetSetting("download_path")
		} else {
			fileName += config.GetSetting("preview_path")
		}
		fileName += string(os.PathSeparator)
		switch v := (*msg).(type) {
		default:
		case whatsapp.ImageMessage:
			fileName += v.Info.Id + "." + strings.TrimPrefix(v.Type, "image/")
			if _, err := os.Stat(fileName); err == nil {
				return fileName, err
			} else if os.IsNotExist(err) {
				if data, err := v.Download(); err == nil {
					return saveAttachment(data, fileName)
				} else {
					return fileName, err
				}
			}
		case whatsapp.DocumentMessage:
			fileName += v.Info.Id + "." + strings.TrimPrefix(strings.TrimPrefix(v.Type, "application/"), "document/")
			if _, err := os.Stat(fileName); err == nil {
				return fileName, err
			} else if os.IsNotExist(err) {
				if data, err := v.Download(); err == nil {
					return saveAttachment(data, fileName)
				} else {
					return fileName, err
				}
			}
		case whatsapp.AudioMessage:
			fileName += v.Info.Id + "." + strings.TrimPrefix(v.Type, "audio/")
			if _, err := os.Stat(fileName); err == nil {
				return fileName, err
			} else if os.IsNotExist(err) {
				if data, err := v.Download(); err == nil {
					return saveAttachment(data, fileName)
				} else {
					return fileName, err
				}
			}
		case whatsapp.VideoMessage:
			fileName += v.Info.Id + "." + strings.TrimPrefix(v.Type, "video/")
			if _, err := os.Stat(fileName); err == nil {
				return fileName, err
			} else if os.IsNotExist(err) {
				if data, err := v.Download(); err == nil {
					return saveAttachment(data, fileName)
				} else {
					return fileName, err
				}
			}
		}
	}
	return "", errors.New("No attachments found")
}

// create a formatted string with regions based on message ID from a text message
// TODO: move message styling into UI
func GetTextMessageString(msg *whatsapp.TextMessage) string {
	colorMe := config.GetColorName("chat_me")
	colorContact := config.GetColorName("chat_contact")
	out := ""
	text := tview.Escape(msg.Text)
	tim := time.Unix(int64(msg.Info.Timestamp), 0)
	time := tim.Format("02-01-06 15:04:05")
	out += "[\""
	out += msg.Info.Id
	out += "\"]"
	if msg.Info.FromMe { //msg from me
		out += "[-::d](" + time + ") [" + colorMe + "::b]Me: [-::-]" + text
	} else if strings.Contains(msg.Info.RemoteJid, GROUPSUFFIX) { // group msg
		userId := msg.Info.SenderJid
		out += "[-::d](" + time + ") [" + colorContact + "::b]" + GetIdShort(userId) + ": [-::-]" + text
	} else { // message from others
		out += "[-::d](" + time + ") [" + colorContact + "::b]" + GetIdShort(msg.Info.RemoteJid) + ": [-::-]" + text
	}
	out += "[\"\"]"
	return out
}

// starts the receiver and message handling thread
// TODO: can't be stopped, can only be called once!
func (sm *SessionManager) StartTextReceiver() error {
	var wac = sm.GetConnection()
	err := sm.LoginWithConnection(wac)
	if err != nil {
		return fmt.Errorf("%v\n", err)
	}
	wac.AddHandler(sm)
	for {
		select {
		case msg := <-sm.TextChannel:
			didNew := sm.db.AddTextMessage(&msg)
			if msg.Info.RemoteJid == sm.currentReceiver {
				if didNew {
					sm.uiHandler.NewMessage(GetTextMessageString(&msg), msg.Info.Id)
				} else {
					screen, ids := sm.db.GetMessagesString(sm.currentReceiver)
					sm.uiHandler.NewScreen(screen, ids)
				}
			}
			sm.uiHandler.SetContacts(sm.db.GetContactIds())
		case other := <-sm.OtherChannel:
			sm.db.AddOtherMessage(&other)
		case command := <-sm.CommandChannel:
			sm.ExecCommand(command)
		}
	}
	fmt.Fprintln(sm.uiHandler.GetWriter(), "closing the receiver")
	wac.Disconnect()
	return nil
}

// sends text to whatsapp id
func (sm SessionManager) SendText(wid string, text string) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: wid,
			FromMe:    true,
			Timestamp: uint64(time.Now().Unix()),
		},
		Text: text,
	}

	_, err := sm.GetConnection().Send(msg)
	if err != nil {
		sm.uiHandler.PrintError(err)
	} else {
		sm.db.AddTextMessage(&msg)
		sm.uiHandler.NewMessage(GetTextMessageString(&msg), msg.Info.Id)
	}
}

// handler struct for whatsapp callbacks

// HandleError implements the error handler interface for go-whatsapp
func (sm *SessionManager) HandleError(err error) {
	sm.uiHandler.PrintText("[red]go-whatsapp reported an error:[-]")
	sm.uiHandler.PrintError(err)
	return
}

// HandleTextMessage implements the text message handler interface for go-whatsapp
func (sm *SessionManager) HandleTextMessage(msg whatsapp.TextMessage) {
	sm.TextChannel <- msg
}

// methods to convert messages to TextMessage
func (sm *SessionManager) HandleImageMessage(message whatsapp.ImageMessage) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			SenderJid: message.Info.SenderJid,
			FromMe:    message.Info.FromMe,
			Timestamp: message.Info.Timestamp,
			Id:        message.Info.Id,
		},
		Text: "[IMAGE] " + message.Caption,
	}
	sm.HandleTextMessage(msg)
	sm.OtherChannel <- message
}

func (sm *SessionManager) HandleDocumentMessage(message whatsapp.DocumentMessage) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			SenderJid: message.Info.SenderJid,
			FromMe:    message.Info.FromMe,
			Timestamp: message.Info.Timestamp,
			Id:        message.Info.Id,
		},
		Text: "[DOCUMENT] " + message.Title,
	}
	sm.HandleTextMessage(msg)
	sm.OtherChannel <- message
}

func (sm *SessionManager) HandleVideoMessage(message whatsapp.VideoMessage) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			SenderJid: message.Info.SenderJid,
			FromMe:    message.Info.FromMe,
			Timestamp: message.Info.Timestamp,
			Id:        message.Info.Id,
		},
		Text: "[VIDEO] " + message.Caption,
	}
	sm.HandleTextMessage(msg)
	sm.OtherChannel <- message
}

func (sm *SessionManager) HandleAudioMessage(message whatsapp.AudioMessage) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			SenderJid: message.Info.SenderJid,
			FromMe:    message.Info.FromMe,
			Timestamp: message.Info.Timestamp,
			Id:        message.Info.Id,
		},
		Text: "[AUDIO]",
	}
	sm.HandleTextMessage(msg)
	sm.OtherChannel <- message
}

// add contact info to database (not needed, internal db of connection is used)
func (sm *SessionManager) HandleNewContact(contact whatsapp.Contact) {
	// redundant, wac has contacts
	//contactChannel <- contact
}

// handle battery messages
//func (t textHandler) HandleBatteryMessage(msg whatsapp.BatteryMessage) {
//  app.QueueUpdate(func() {
//    infoBar.SetText("🔋: " + string(msg.Percentage) + "%")
//  })
//}

// helper to save an attachment and open it if specified
func saveAttachment(data []byte, path string) (string, error) {
	err := ioutil.WriteFile(path, data, 0644)
	return path, err
}

// reads the session file from disk
func readSession() (whatsapp.Session, error) {
	session := whatsapp.Session{}
	file, err := os.Open(config.GetSessionFilePath())
	if err != nil {
		// load old session file, delete if found
		file, err = os.Open(GetHomeDir() + ".whatscli.session")
		if err != nil {
			return session, err
		} else {
			os.Remove(GetHomeDir() + ".whatscli.session")
		}
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&session)
	if err != nil {
		return session, err
	}
	return session, nil
}

// saves the session file to disk
func writeSession(session whatsapp.Session) error {
	file, err := os.Create(config.GetSessionFilePath())
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(session)
	if err != nil {
		return err
	}
	return nil
}

// deletes the session file from disk
func removeSession() error {
	return os.Remove(config.GetSessionFilePath())
}
