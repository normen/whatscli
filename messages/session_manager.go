package messages

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gdamore/tcell/v2"
	"github.com/gen2brain/beeep"
	"github.com/rivo/tview"
	"mvdan.cc/xurls/v2"

	"github.com/Rhymen/go-whatsapp"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/qrcode"
)

// TODO: move message styling and ordering into UI, don't use strings

// SessionManager deals with the connection and receives commands from the UI
// it updates the UI accordingly
type SessionManager struct {
	db              *MessageDatabase
	currentReceiver string // currently selected contact for message handling
	uiHandler       UiMessageHandler
	connection      *whatsapp.Conn
	BatteryChannel  chan BatteryMsg
	StatusChannel   chan StatusMsg
	CommandChannel  chan Command
	TextChannel     chan whatsapp.TextMessage
	OtherChannel    chan interface{}
	statusInfo      SessionStatus
	lastSent        time.Time
}

// initialize the SessionManager
func (sm *SessionManager) Init(handler UiMessageHandler) {
	sm.db = &MessageDatabase{}
	sm.db.Init()
	sm.uiHandler = handler
	sm.BatteryChannel = make(chan BatteryMsg, 10)
	sm.StatusChannel = make(chan StatusMsg, 10)
	sm.CommandChannel = make(chan Command, 10)
	sm.TextChannel = make(chan whatsapp.TextMessage, 10)
	sm.OtherChannel = make(chan interface{}, 10)
}

// starts the receiver and message handling thread
// TODO: can't be stopped, can only be called once!
func (sm *SessionManager) StartManager() error {
	var wac = sm.getConnection()
	err := sm.loginWithConnection(wac)
	if err != nil {
		sm.uiHandler.PrintError(err)
	}
	wac.AddHandler(sm)
	for {
		select {
		case msg := <-sm.TextChannel:
			didNew := sm.db.AddTextMessage(&msg)
			if msg.Info.RemoteJid == sm.currentReceiver {
				if didNew {
					sm.uiHandler.NewMessage(sm.getTextMessageString(&msg), msg.Info.Id)
				} else {
					screen, ids := sm.getMessagesString(sm.currentReceiver)
					sm.uiHandler.NewScreen(screen, ids)
				}
				// notify if contact is in focus and we didn't send a message recently
				// TODO: move to UI (when UI has time in messages)
				if config.Config.General.EnableNotifications && !msg.Info.FromMe {
					if int64(msg.Info.Timestamp) > time.Now().Unix()-30 {
						if int64(msg.Info.Timestamp) > sm.lastSent.Unix()+config.Config.General.NotificationTimeout {
							err := beeep.Notify(sm.GetIdShort(msg.Info.RemoteJid), msg.Text, "")
							if err != nil {
								sm.uiHandler.PrintError(err)
							}
						}
					}
				}
			} else {
				if config.Config.General.EnableNotifications && !msg.Info.FromMe {
					// notify if message is younger than 30 sec and not in focus
					if int64(msg.Info.Timestamp) > time.Now().Unix()-30 {
						err := beeep.Notify(sm.GetIdShort(msg.Info.RemoteJid), msg.Text, "")
						if err != nil {
							sm.uiHandler.PrintError(err)
						}
					}
				}
			}
			sm.uiHandler.SetContacts(sm.db.GetContactIds())
		case other := <-sm.OtherChannel:
			sm.db.AddOtherMessage(&other)
		case command := <-sm.CommandChannel:
			sm.execCommand(command)
		case batteryMsg := <-sm.BatteryChannel:
			sm.statusInfo.BatteryLoading = batteryMsg.loading
			sm.statusInfo.BatteryPowersave = batteryMsg.powersave
			sm.statusInfo.BatteryCharge = batteryMsg.charge
			sm.uiHandler.SetStatus(sm.statusInfo)
		case statusMsg := <-sm.StatusChannel:
			if statusMsg.err != nil {
			} else {
				sm.statusInfo.Connected = statusMsg.connected
			}
			wac := sm.getConnection()
			connected := wac.GetConnected()
			sm.statusInfo.Connected = connected
			sm.uiHandler.SetStatus(sm.statusInfo)
		}
	}
	fmt.Fprintln(sm.uiHandler.GetWriter(), "closing the receiver")
	wac.Disconnect()
	return nil
}

// set the currently selected contact
func (sm *SessionManager) setCurrentReceiver(id string) {
	sm.currentReceiver = id
	screen, ids := sm.getMessagesString(id)
	sm.uiHandler.NewScreen(screen, ids)
}

// gets an existing connection or creates one
func (sm *SessionManager) getConnection() *whatsapp.Conn {
	var wac *whatsapp.Conn
	if sm.connection == nil {
		wacc, err := whatsapp.NewConn(5 * time.Second)
		if err != nil {
			return nil
		}
		wac = wacc
		sm.connection = wac
		//wac.SetClientVersion(2, 2021, 4)
	} else {
		wac = sm.connection
	}
	return wac
}

// login logs in the user. It ries to see if a session already exists. If not, tries to create a
// new one using qr scanned on the terminal.
func (sm *SessionManager) login() error {
	return sm.loginWithConnection(sm.getConnection())
}

// loginWithConnection logs in the user using a provided connection. It ries to see if a session already exists. If not, tries to create a
// new one using qr scanned on the terminal.
func (sm *SessionManager) loginWithConnection(wac *whatsapp.Conn) error {
	if wac != nil && wac.GetConnected() {
		wac.Disconnect()
		sm.StatusChannel <- StatusMsg{false, nil}
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
	sm.StatusChannel <- StatusMsg{true, nil}
	return nil
}

// disconnects the session
func (sm *SessionManager) disconnect() error {
	wac := sm.getConnection()
	var err error
	if wac != nil && wac.GetConnected() {
		_, err = wac.Disconnect()
	}
	sm.StatusChannel <- StatusMsg{false, err}
	return err
}

// logout logs out the user, deletes session file
func (ub *SessionManager) logout() error {
	ub.getConnection().Disconnect()
	return removeSession()
}

// executes a command
func (sm *SessionManager) execCommand(command Command) {
	cmd := command.Name
	switch cmd {
	default:
		sm.uiHandler.PrintText("[" + config.Config.Colors.Negative + "]Unknown command: [-]" + cmd)
	case "backlog":
		if sm.currentReceiver == "" {
			return
		}
		count := 10
		if currentMsgs, ok := sm.db.textMessages[sm.currentReceiver]; ok {
			if len(currentMsgs) > 0 {
				firstMsg := currentMsgs[0]
				go sm.getConnection().LoadChatMessages(sm.currentReceiver, count, firstMsg.Info.Id, firstMsg.Info.FromMe, false, sm)
			}
		}
	case "login":
		sm.uiHandler.PrintError(sm.login())
	case "connect":
		sm.uiHandler.PrintError(sm.login())
	case "disconnect":
		sm.uiHandler.PrintError(sm.disconnect())
	case "logout":
		sm.uiHandler.PrintError(sm.logout())
	case "send":
		if checkParam(command.Params, 2) {
			textParams := command.Params[1:]
			text := strings.Join(textParams, " ")
			sm.sendText(command.Params[0], text)
		} else {
			sm.printCommandUsage("send", "[user-id[] [message text[]")
		}
	case "select":
		if checkParam(command.Params, 1) {
			sm.setCurrentReceiver(command.Params[0])
		} else {
			sm.printCommandUsage("select", "[user-id[]")
		}
	case "info":
		if checkParam(command.Params, 1) {
			sm.uiHandler.PrintText(sm.db.GetMessageInfo(command.Params[0]))
		} else {
			sm.printCommandUsage("info", "[message-id[]")
		}
	case "download":
		if checkParam(command.Params, 1) {
			if path, err := sm.downloadMessage(command.Params[0], false); err != nil {
				sm.uiHandler.PrintError(err)
			} else {
				sm.uiHandler.PrintText("[::d] -> " + path + "[::-]")
			}
		} else {
			sm.printCommandUsage("download", "[message-id[]")
		}
	case "open":
		if checkParam(command.Params, 1) {
			if path, err := sm.downloadMessage(command.Params[0], true); err == nil {
				sm.uiHandler.OpenFile(path)
			} else {
				sm.uiHandler.PrintError(err)
			}
		} else {
			sm.printCommandUsage("open", "[message-id[]")
		}
	case "show":
		if checkParam(command.Params, 1) {
			if path, err := sm.downloadMessage(command.Params[0], true); err == nil {
				sm.uiHandler.PrintFile(path)
			} else {
				sm.uiHandler.PrintError(err)
			}
		} else {
			sm.printCommandUsage("show", "[message-id[]")
		}
	case "url":
		if checkParam(command.Params, 1) {
			if msg, ok := sm.db.messagesById[command.Params[0]]; ok {
				urlParser := xurls.Relaxed()
				url := urlParser.FindString(msg.Text)
				if url != "" {
					sm.uiHandler.OpenFile(url)
				}
			}
		} else {
			sm.printCommandUsage("url", "[message-id[]")
		}
	case "upload":
		if sm.currentReceiver == "" {
			return
		}
		var err error
		var mime *mimetype.MIME
		var file *os.File
		if checkParam(command.Params, 1) {
			path := strings.Join(command.Params, " ")
			if mime, err = mimetype.DetectFile(path); err == nil {
				if file, err = os.Open(path); err == nil {
					msg := whatsapp.DocumentMessage{
						Info: whatsapp.MessageInfo{
							RemoteJid: sm.currentReceiver,
						},
						Type:     mime.String(),
						FileName: filepath.Base(file.Name()),
					}
					wac := sm.getConnection()
					sm.lastSent = time.Now()
					_, err = wac.Send(msg)
				}
			}
		} else {
			sm.printCommandUsage("upload", "/path/to/file")
		}
		sm.uiHandler.PrintError(err)
	case "sendimage":
		if sm.currentReceiver == "" {
			return
		}
		var err error
		var mime *mimetype.MIME
		var file *os.File
		if checkParam(command.Params, 1) {
			path := strings.Join(command.Params, " ")
			if mime, err = mimetype.DetectFile(path); err == nil {
				if file, err = os.Open(path); err == nil {
					msg := whatsapp.ImageMessage{
						Info: whatsapp.MessageInfo{
							RemoteJid: sm.currentReceiver,
						},
						Type:    mime.String(),
						Content: file,
					}
					wac := sm.getConnection()
					sm.lastSent = time.Now()
					_, err = wac.Send(msg)
				}
			}
		} else {
			sm.printCommandUsage("sendimage", "/path/to/file")
		}
		sm.uiHandler.PrintError(err)
	case "sendvideo":
		if sm.currentReceiver == "" {
			return
		}
		var err error
		var mime *mimetype.MIME
		var file *os.File
		if checkParam(command.Params, 1) {
			path := strings.Join(command.Params, " ")
			if mime, err = mimetype.DetectFile(path); err == nil {
				if file, err = os.Open(path); err == nil {
					msg := whatsapp.VideoMessage{
						Info: whatsapp.MessageInfo{
							RemoteJid: sm.currentReceiver,
						},
						Type:    mime.String(),
						Content: file,
					}
					wac := sm.getConnection()
					sm.lastSent = time.Now()
					_, err = wac.Send(msg)
				}
			}
		} else {
			sm.printCommandUsage("sendvideo", "/path/to/file")
		}
		sm.uiHandler.PrintError(err)
	case "sendaudio":
		if sm.currentReceiver == "" {
			return
		}
		var err error
		var mime *mimetype.MIME
		var file *os.File
		if checkParam(command.Params, 1) {
			path := strings.Join(command.Params, " ")
			if mime, err = mimetype.DetectFile(path); err == nil {
				if file, err = os.Open(path); err == nil {
					msg := whatsapp.AudioMessage{
						Info: whatsapp.MessageInfo{
							RemoteJid: sm.currentReceiver,
						},
						Type:    mime.String(),
						Content: file,
					}
					wac := sm.getConnection()
					sm.lastSent = time.Now()
					_, err = wac.Send(msg)
				}
			}
		} else {
			sm.printCommandUsage("sendaudio", "/path/to/file")
		}
		sm.uiHandler.PrintError(err)
	case "revoke":
		if checkParam(command.Params, 1) {
			wac := sm.getConnection()
			var revId string
			var err error
			if msgg, ok := sm.db.otherMessages[command.Params[0]]; ok {
				switch msg := (*msgg).(type) {
				default:
				case whatsapp.ImageMessage:
					revId, err = wac.RevokeMessage(msg.Info.RemoteJid, msg.Info.Id, msg.Info.FromMe)
				case whatsapp.DocumentMessage:
					revId, err = wac.RevokeMessage(msg.Info.RemoteJid, msg.Info.Id, msg.Info.FromMe)
				case whatsapp.AudioMessage:
					revId, err = wac.RevokeMessage(msg.Info.RemoteJid, msg.Info.Id, msg.Info.FromMe)
				case whatsapp.VideoMessage:
					revId, err = wac.RevokeMessage(msg.Info.RemoteJid, msg.Info.Id, msg.Info.FromMe)
				}
			} else {
				if msg, ok := sm.db.messagesById[command.Params[0]]; ok {
					revId, err = wac.RevokeMessage(msg.Info.RemoteJid, msg.Info.Id, msg.Info.FromMe)
				}
			}
			if err == nil {
				sm.uiHandler.PrintText("revoked: " + revId)
			}
			sm.uiHandler.PrintError(err)
		} else {
			sm.printCommandUsage("revoke", "[message-id[]")
		}
	case "leave":
		groupId := sm.currentReceiver
		if checkParam(command.Params, 1) {
			groupId = command.Params[0]
		}
		wac := sm.getConnection()
		var err error
		_, err = wac.LeaveGroup(groupId)
		if err == nil {
			sm.uiHandler.PrintText("left group " + groupId)
		}
		sm.uiHandler.PrintError(err)
	case "colorlist":
		out := ""
		for idx, _ := range tcell.ColorNames {
			out = out + "[" + idx + "]" + idx + "[-]\n"
		}
		sm.uiHandler.PrintText(out)
	}
}

// helper for built-in command help
func (sm *SessionManager) printCommandUsage(command string, usage string) {
	sm.uiHandler.PrintText("[" + config.Config.Colors.Negative + "]Usage:[-] " + command + " " + usage)
}

// check if parameters for command are okay
func checkParam(arr []string, length int) bool {
	if arr == nil || len(arr) < length {
		return false
	}
	return true
}

// gets a pretty name for a whatsapp id
func (sm *SessionManager) GetIdName(id string) string {
	if val, ok := sm.getConnection().Store.Contacts[id]; ok {
		if val.Name != "" {
			return val.Name
		} else if val.Short != "" {
			return val.Short
		} else if val.Notify != "" {
			return val.Notify
		}
	}
	return strings.TrimSuffix(id, CONTACTSUFFIX)
}

// gets a short name for a whatsapp id
func (sm *SessionManager) GetIdShort(id string) string {
	if val, ok := sm.getConnection().Store.Contacts[id]; ok {
		if val.Short != "" {
			return val.Short
		} else if val.Name != "" {
			return val.Name
		} else if val.Notify != "" {
			return val.Notify
		}
	}
	return strings.TrimSuffix(id, CONTACTSUFFIX)
}

// get a string representation of all messages for contact
func (sm *SessionManager) getMessagesString(wid string) (string, []string) {
	out := ""
	ids := []string{}
	msgs := sm.db.GetMessages(wid)
	for _, msg := range msgs {
		out += sm.getTextMessageString(&msg)
		out += "\n"
		ids = append(ids, msg.Info.Id)
	}
	return out, ids
}

// create a formatted string with regions based on message ID from a text message
// TODO: move message styling into UI
func (sm *SessionManager) getTextMessageString(msg *whatsapp.TextMessage) string {
	colorMe := config.Config.Colors.ChatMe
	colorContact := config.Config.Colors.ChatContact
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
		out += "[-::d](" + time + ") [" + colorContact + "::b]" + sm.GetIdShort(userId) + ": [-::-]" + text
	} else { // message from others
		out += "[-::d](" + time + ") [" + colorContact + "::b]" + sm.GetIdShort(msg.Info.RemoteJid) + ": [-::-]" + text
	}
	out += "[\"\"]"
	return out
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
			fileName += config.Config.General.PreviewPath
		} else {
			fileName += config.Config.General.DownloadPath
		}
		fileName += string(os.PathSeparator)
		switch v := (*msg).(type) {
		default:
		case whatsapp.ImageMessage:
			fileName += v.Info.Id
			if exts, err := mime.ExtensionsByType(v.Type); err == nil {
				fileName += exts[0]
			}
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
			fileName += v.Info.Id
			if exts, err := mime.ExtensionsByType(v.Type); err == nil {
				fileName += exts[0]
			}
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
			fileName += v.Info.Id
			if exts, err := mime.ExtensionsByType(v.Type); err == nil {
				fileName += exts[0]
			}
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
			fileName += v.Info.Id
			if exts, err := mime.ExtensionsByType(v.Type); err == nil {
				fileName += exts[0]
			}
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

// sends text to whatsapp id
func (sm *SessionManager) sendText(wid string, text string) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: wid,
			FromMe:    true,
			Timestamp: uint64(time.Now().Unix()),
		},
		Text: text,
	}

	sm.lastSent = time.Now()
	_, err := sm.getConnection().Send(msg)
	if err != nil {
		sm.uiHandler.PrintError(err)
	} else {
		sm.db.AddTextMessage(&msg)
		if sm.currentReceiver == wid {
			sm.uiHandler.NewMessage(sm.getTextMessageString(&msg), msg.Info.Id)
		}
	}
}

// handler struct for whatsapp callbacks

// HandleError implements the error handler interface for go-whatsapp
func (sm *SessionManager) HandleError(err error) {
	sm.uiHandler.PrintText("[" + config.Config.Colors.Negative + "]go-whatsapp reported an error:[-]")
	sm.uiHandler.PrintError(err)
	statusMsg := StatusMsg{false, err}
	sm.StatusChannel <- statusMsg
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
func (sm *SessionManager) HandleBatteryMessage(msg whatsapp.BatteryMessage) {
	sm.BatteryChannel <- BatteryMsg{msg.Percentage, msg.Plugged, msg.Powersave}
}

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
		file, err = os.Open(config.GetHomeDir() + ".whatscli.session")
		if err != nil {
			return session, err
		} else {
			os.Remove(config.GetHomeDir() + ".whatscli.session")
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
