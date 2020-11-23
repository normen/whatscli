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
// move these funcs/interface to channels
type UiMessageHandler interface {
	NewMessage(string, string)
	NewScreen(string, []string)
	SetContacts([]string)
	PrintError(error)
	PrintText(string)
	PrintFile(string)
	OpenFile(string)
	GetWriter() io.Writer
}

type Command struct {
	Name   string
	Params []string
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

func (sm *SessionManager) setCurrentReceiver(id string) {
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

// login logs in the user. It ries to see if a session already exists. If not, tries to create a
// new one using qr scanned on the terminal.
func (sm *SessionManager) login() error {
	return sm.loginWithConnection(sm.GetConnection())
}

// loginWithConnection logs in the user using a provided connection. It ries to see if a session already exists. If not, tries to create a
// new one using qr scanned on the terminal.
func (sm *SessionManager) loginWithConnection(wac *whatsapp.Conn) error {
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

func (sm *SessionManager) disconnect() error {
	wac := sm.GetConnection()
	if wac != nil && wac.GetConnected() {
		_, err := wac.Disconnect()
		return err
	}
	return nil
}

// logout logs out the user.
func (ub *SessionManager) logout() error {
	return removeSession()
}

func (sm *SessionManager) execCommand(command Command) {
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
		sm.login()
	case "connect":
		sm.login()
	case "disconnect":
		sm.uiHandler.PrintError(sm.disconnect())
	case "logout":
		sm.uiHandler.PrintError(sm.logout())
	case "send":
		sm.sendText(command.Params[0], command.Params[1])
	case "select":
		sm.setCurrentReceiver(command.Params[0])
	case "info":
		sm.uiHandler.PrintText(sm.db.GetMessageInfo(command.Params[0]))
	case "download":
		if path, err := sm.downloadMessage(command.Params[0], false); err != nil {
			sm.uiHandler.PrintError(err)
		} else {
			sm.uiHandler.PrintText("[::d] -> " + path + "[::-]")
		}
	case "open":
		if path, err := sm.downloadMessage(command.Params[0], true); err == nil {
			sm.uiHandler.OpenFile(path)
		} else {
			sm.uiHandler.PrintError(err)
		}
	case "show":
		if path, err := sm.downloadMessage(command.Params[0], true); err == nil {
			sm.uiHandler.PrintFile(path)
		} else {
			sm.uiHandler.PrintError(err)
		}
	}
}

// load data for message specified by message id TODO: support types
func (sm *SessionManager) loadMessageData(wid string) ([]byte, error) {
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
func (sm *SessionManager) downloadMessage(wid string, preview bool) (string, error) {
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

// starts the receiver and message handling thread
// TODO: can't be stopped, can only be called once!
func (sm *SessionManager) StartTextReceiver() error {
	var wac = sm.GetConnection()
	err := sm.loginWithConnection(wac)
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
					sm.uiHandler.NewMessage(getTextMessageString(&msg), msg.Info.Id)
				} else {
					screen, ids := sm.db.GetMessagesString(sm.currentReceiver)
					sm.uiHandler.NewScreen(screen, ids)
				}
			}
			sm.uiHandler.SetContacts(sm.db.GetContactIds())
		case other := <-sm.OtherChannel:
			sm.db.AddOtherMessage(&other)
		case command := <-sm.CommandChannel:
			sm.execCommand(command)
		}
	}
	fmt.Fprintln(sm.uiHandler.GetWriter(), "closing the receiver")
	wac.Disconnect()
	return nil
}

// sends text to whatsapp id
func (sm SessionManager) sendText(wid string, text string) {
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
		sm.uiHandler.NewMessage(getTextMessageString(&msg), msg.Info.Id)
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
