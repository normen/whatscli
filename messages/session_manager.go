package messages

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Rhymen/go-whatsapp"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gdamore/tcell/v2"
	"github.com/gen2brain/beeep"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/qrcode"
	"github.com/rivo/tview"
	"mvdan.cc/xurls/v2"
)

// SessionManager deals with the connection and receives commands from the UI
// it updates the UI accordingly
type SessionManager struct {
	db              *MessageDatabase
	currentReceiver string // currently selected chat for message handling
	uiHandler       UiMessageHandler
	connection      *whatsapp.Conn
	BatteryChannel  chan BatteryMsg
	StatusChannel   chan StatusMsg
	CommandChannel  chan Command
	ChatChannel     chan whatsapp.Chat
	ContactChannel  chan whatsapp.Contact
	TextChannel     chan whatsapp.TextMessage
	OtherChannel    chan interface{}
	statusInfo      SessionStatus
	lastSent        time.Time
	started         bool
}

// initialize the SessionManager
func (sm *SessionManager) Init(handler UiMessageHandler) {
	sm.db = &MessageDatabase{}
	sm.db.Init()
	sm.uiHandler = handler
	sm.BatteryChannel = make(chan BatteryMsg, 10)
	sm.StatusChannel = make(chan StatusMsg, 10)
	sm.CommandChannel = make(chan Command, 10)
	sm.ChatChannel = make(chan whatsapp.Chat, 10)
	sm.ContactChannel = make(chan whatsapp.Contact, 10)
	sm.TextChannel = make(chan whatsapp.TextMessage, 10)
	sm.OtherChannel = make(chan interface{}, 10)
}

// starts the receiver and message handling go routine
func (sm *SessionManager) StartManager() error {
	if sm.started {
		return errors.New("session manager running, send commands to control")
	}
	sm.started = true
	go sm.runManager()
	return nil
}

func (sm *SessionManager) runManager() error {
	var wac = sm.getConnection()
	err := sm.loginWithConnection(wac)
	if err != nil {
		sm.uiHandler.PrintError(err)
	}
	wac.AddHandler(sm)
	for sm.started == true {
		select {
		case msg := <-sm.TextChannel:
			didNew := sm.db.AddTextMessage(&msg)
			if msg.Info.RemoteJid == sm.currentReceiver {
				if didNew {
					sm.uiHandler.NewMessage(sm.createMessage(&msg))
				} else {
					screen := sm.getMessages(sm.currentReceiver)
					sm.uiHandler.NewScreen(screen)
				}
				// notify if chat is in focus and we didn't send a message recently
				// TODO: move notify to UI
				if int64(msg.Info.Timestamp) > time.Now().Unix()-30 {
					if int64(msg.Info.Timestamp) > sm.lastSent.Unix()+config.Config.General.NotificationTimeout {
						sm.db.NewUnreadChat(msg.Info.RemoteJid)
						if !msg.Info.FromMe {
							err := notify(sm.db.GetIdShort(msg.Info.RemoteJid), msg.Text)
							if err != nil {
								sm.uiHandler.PrintError(err)
							}
						}
					}
				}
			} else {
				// notify if message is younger than 30 sec and not in focus
				if int64(msg.Info.Timestamp) > time.Now().Unix()-30 {
					sm.db.NewUnreadChat(msg.Info.RemoteJid)
					if !msg.Info.FromMe {
						err := notify(sm.db.GetIdShort(msg.Info.RemoteJid), msg.Text)
						if err != nil {
							sm.uiHandler.PrintError(err)
						}
					}
				}
			}
			sm.uiHandler.SetChats(sm.db.GetChatIds())
		case other := <-sm.OtherChannel:
			sm.db.AddOtherMessage(&other)
		case c := <-sm.ContactChannel:
			contact := Contact{
				c.Jid,
				c.Name,
				c.Short,
			}
			if contact.Name == "" && c.Notify != "" {
				contact.Name = c.Notify
			}
			if contact.Short == "" && c.Notify != "" {
				contact.Short = c.Notify
			}
			sm.db.AddContact(contact)
			sm.uiHandler.SetChats(sm.db.GetChatIds())
		case c := <-sm.ChatChannel:
			if c.IsMarkedSpam == "false" {
				isGroup := strings.Contains(c.Jid, GROUPSUFFIX)
				unread, _ := strconv.ParseInt(c.Unread, 10, 0)
				last, _ := strconv.ParseInt(c.LastMessageTime, 10, 64)
				chat := Chat{
					c.Jid,
					isGroup,
					c.Name,
					int(unread),
					last,
				}
				sm.db.AddChat(chat)
				sm.uiHandler.SetChats(sm.db.GetChatIds())
			}
		case command := <-sm.CommandChannel:
			sm.execCommand(command)
		case batteryMsg := <-sm.BatteryChannel:
			sm.statusInfo.BatteryLoading = batteryMsg.loading
			sm.statusInfo.BatteryPowersave = batteryMsg.powersave
			sm.statusInfo.BatteryCharge = batteryMsg.charge
			sm.uiHandler.SetStatus(sm.statusInfo)
		case statusMsg := <-sm.StatusChannel:
			prevStatus := sm.statusInfo.Connected
			if statusMsg.err != nil {
			} else {
				sm.statusInfo.Connected = statusMsg.connected
			}
			wac := sm.getConnection()
			connected := wac.GetConnected()
			sm.statusInfo.Connected = connected
			sm.uiHandler.SetStatus(sm.statusInfo)
			if prevStatus != sm.statusInfo.Connected {
				if sm.statusInfo.Connected {
					sm.uiHandler.PrintText("connected")
				} else {
					sm.uiHandler.PrintText("disconnected")
				}
			}
		}
	}
	fmt.Fprintln(sm.uiHandler.GetWriter(), "closing the receiver")
	wac.Disconnect()
	return nil
}

// set the currently selected chat
func (sm *SessionManager) setCurrentReceiver(id string) {
	sm.currentReceiver = id
	screen := sm.getMessages(id)
	sm.uiHandler.NewScreen(screen)
}

// gets an existing connection or creates one
func (sm *SessionManager) getConnection() *whatsapp.Conn {
	var wac *whatsapp.Conn
	if sm.connection == nil {
		options := &whatsapp.Options{
			Timeout:         5 * time.Second,
			LongClientName:  "WhatsCLI Client",
			ShortClientName: "whatscli",
		}
		wacc, err := whatsapp.NewConnWithOptions(options)
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
	sm.uiHandler.PrintText("connecting..")
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
	//get initial battery state
	sm.BatteryChannel <- BatteryMsg{
		wac.Info.Battery,
		wac.Info.Plugged,
		false,
	}
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
	err := ub.getConnection().Logout()
	ub.StatusChannel <- StatusMsg{false, err}
	ub.uiHandler.PrintText("removing login data..")
	return removeSession()
}

// executes a command
func (sm *SessionManager) execCommand(command Command) {
	cmd := command.Name
	switch cmd {
	default:
		sm.uiHandler.PrintText("[" + config.Config.Colors.Negative + "]Unknown command: [-]" + cmd)
	case "backlog":
		if sm.currentReceiver != "" {
			count := 10
			if currentMsgs, ok := sm.db.textMessages[sm.currentReceiver]; ok {
				if len(currentMsgs) > 0 {
					firstMsg := currentMsgs[0]
					go sm.getConnection().LoadChatMessages(sm.currentReceiver, count, firstMsg.Info.Id, firstMsg.Info.FromMe, false, sm)
				}
			} else {
				go sm.getConnection().LoadChatMessages(sm.currentReceiver, count, "", false, false, sm)
			}
		} else {
			sm.printCommandUsage("backlog", "-> only works in a chat")
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
			sm.printCommandUsage("send", "[chat-id[] [message text[]")
		}
	case "select":
		if checkParam(command.Params, 1) {
			sm.setCurrentReceiver(command.Params[0])
		} else {
			sm.printCommandUsage("select", "[chat-id[]")
		}
	case "read":
		if sm.currentReceiver != "" {
			// need to send message id, so get all (unread count)
			// recent messages and send "read"
			if chat, ok := sm.db.chats[sm.currentReceiver]; ok {
				count := chat.Unread
				msgs := sm.db.GetMessages(chat.Id)
				length := len(msgs)
				for idx, msg := range msgs {
					if idx >= length-count {
						sm.getConnection().Read(chat.Id, msg.Info.Id)
					}
				}
				chat.Unread = 0
				sm.db.chats[sm.currentReceiver] = chat
				sm.uiHandler.SetChats(sm.db.GetChatIds())
			}
		} else {
			sm.printCommandUsage("read", "-> only works in a chat")
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
			sm.printCommandUsage("upload", "-> only works in a chat")
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
			sm.printCommandUsage("sendimage", "-> only works in a chat")
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
			sm.printCommandUsage("sendvideo", "-> only works in a chat")
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
			sm.printCommandUsage("sendaudio", "-> only works in a chat")
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
		if strings.Index(groupId, GROUPSUFFIX) < 0 {
			sm.uiHandler.PrintText("not a group")
			return
		}
		wac := sm.getConnection()
		var err error
		_, err = wac.LeaveGroup(groupId)
		if err == nil {
			sm.uiHandler.PrintText("left group " + groupId)
		}
		sm.uiHandler.PrintError(err)
	case "create":
		if !checkParam(command.Params, 1) {
			sm.printCommandUsage("create", "[user-id[] [user-id[] New Group Subject")
			sm.printCommandUsage("create", "New Group Subject")
			return
		}
		// first params are users if ending in CONTACTSUFFIX, rest is name
		users := []string{}
		idx := 0
		size := len(command.Params)
		for idx = 0; idx < size && strings.Index(command.Params[idx], CONTACTSUFFIX) > 0; idx++ {
			users = append(users, command.Params[idx])
		}
		name := ""
		if len(command.Params) > idx {
			name = strings.Join(command.Params[idx:], " ")
		}
		wac := sm.getConnection()
		var err error
		var groupId <-chan string
		groupId, err = wac.CreateGroup(name, users)
		if err == nil {
			sm.uiHandler.PrintText("creating new group " + name)
			resultInfo := <-groupId
			//{"status":200,"gid":"491600000009-0606000436@g.us","participants":[{"491700000000@c.us":{"code":"200"}},{"4917600000001@c.us":{"code": "200"}}]}
			var result map[string]interface{}
			json.Unmarshal([]byte(resultInfo), &result)
			newChatId := result["gid"].(string)
			sm.uiHandler.PrintText("got new Id " + newChatId)
			newChat := Chat{}
			newChat.Id = newChatId
			newChat.Name = name
			newChat.IsGroup = true
			sm.db.chats[newChatId] = newChat
			sm.uiHandler.SetChats(sm.db.GetChatIds())
		}
		sm.uiHandler.PrintError(err)
	case "add":
		groupId := sm.currentReceiver
		if strings.Index(groupId, GROUPSUFFIX) < 0 {
			sm.uiHandler.PrintText("not a group")
			return
		}
		if !checkParam(command.Params, 1) {
			sm.printCommandUsage("add", "[user-id[]")
			return
		}
		wac := sm.getConnection()
		var err error
		_, err = wac.AddMember(groupId, command.Params)
		if err == nil {
			sm.uiHandler.PrintText("added new members for " + groupId)
		}
		sm.uiHandler.PrintError(err)
	case "remove":
		groupId := sm.currentReceiver
		if strings.Index(groupId, GROUPSUFFIX) < 0 {
			sm.uiHandler.PrintText("not a group")
			return
		}
		if !checkParam(command.Params, 1) {
			sm.printCommandUsage("remove", "[user-id[]")
			return
		}
		wac := sm.getConnection()
		var err error
		_, err = wac.RemoveMember(groupId, command.Params)
		if err == nil {
			sm.uiHandler.PrintText("removed from " + groupId)
		}
		sm.uiHandler.PrintError(err)
	case "removeadmin":
		groupId := sm.currentReceiver
		if strings.Index(groupId, GROUPSUFFIX) < 0 {
			sm.uiHandler.PrintText("not a group")
			return
		}
		if !checkParam(command.Params, 1) {
			sm.printCommandUsage("removeadmin", "[user-id[]")
			return
		}
		wac := sm.getConnection()
		var err error
		_, err = wac.RemoveAdmin(groupId, command.Params)
		if err == nil {
			sm.uiHandler.PrintText("removed admin for " + groupId)
		}
		sm.uiHandler.PrintError(err)
	case "admin":
		groupId := sm.currentReceiver
		if strings.Index(groupId, GROUPSUFFIX) < 0 {
			sm.uiHandler.PrintText("not a group")
			return
		}
		if !checkParam(command.Params, 1) {
			sm.printCommandUsage("admin", "[user-id[]")
			return
		}
		wac := sm.getConnection()
		var err error
		_, err = wac.SetAdmin(groupId, command.Params)
		if err == nil {
			sm.uiHandler.PrintText("added admin for " + groupId)
		}
		sm.uiHandler.PrintError(err)
	case "subject":
		groupId := sm.currentReceiver
		if strings.Index(groupId, GROUPSUFFIX) < 0 {
			sm.uiHandler.PrintText("not a group")
			return
		}
		if !checkParam(command.Params, 1) || groupId == "" {
			sm.printCommandUsage("subject", "new-subject -> in group chat")
			return
		}
		name := strings.Join(command.Params, " ")
		wac := sm.getConnection()
		var err error
		_, err = wac.UpdateGroupSubject(name, groupId)
		if err == nil {
			sm.uiHandler.PrintText("updated subject for " + groupId)
		}
		newChat := sm.db.chats[groupId]
		newChat.Name = name
		sm.db.chats[groupId] = newChat
		sm.uiHandler.SetChats(sm.db.GetChatIds())
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

// get all messages for one chat id
func (sm *SessionManager) getMessages(wid string) []Message {
	msgs := sm.db.GetMessages(wid)
	ids := []Message{}
	for _, msg := range msgs {
		ids = append(ids, sm.createMessage(&msg))
	}
	return ids
}

// create internal message from whatsapp message
// TODO: store these instead of generating each time
func (sm *SessionManager) createMessage(msg *whatsapp.TextMessage) Message {
	newMsg := Message{}
	newMsg.Id = msg.Info.Id
	newMsg.ChatId = msg.Info.RemoteJid
	newMsg.FromMe = msg.Info.FromMe
	newMsg.Timestamp = msg.Info.Timestamp
	newMsg.Text = msg.Text
	newMsg.Forwarded = msg.ContextInfo.IsForwarded
	if strings.Contains(msg.Info.RemoteJid, STATUSSUFFIX) {
		newMsg.ContactId = msg.Info.SenderJid
		newMsg.ContactName = sm.db.GetIdName(msg.Info.SenderJid)
		newMsg.ContactShort = sm.db.GetIdShort(msg.Info.SenderJid)
	} else if strings.Contains(msg.Info.RemoteJid, GROUPSUFFIX) {
		newMsg.ContactId = msg.Info.SenderJid
		newMsg.ContactName = sm.db.GetIdName(msg.Info.SenderJid)
		newMsg.ContactShort = sm.db.GetIdShort(msg.Info.SenderJid)
	} else {
		newMsg.ContactId = msg.Info.RemoteJid
		newMsg.ContactName = sm.db.GetIdName(msg.Info.RemoteJid)
		newMsg.ContactShort = sm.db.GetIdShort(msg.Info.RemoteJid)
	}
	return newMsg
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
	newid, err := sm.getConnection().Send(msg)
	msg.Info.Id = newid
	if err != nil {
		sm.uiHandler.PrintError(err)
	} else {
		sm.db.AddTextMessage(&msg)
		if sm.currentReceiver == wid {
			sm.uiHandler.NewMessage(sm.createMessage(&msg))
		}
	}
}

// handler struct for whatsapp callbacks

// HandleError implements the error handler interface for go-whatsapp
func (sm *SessionManager) HandleError(err error) {
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
	sm.ContactChannel <- contact
}

// handle battery messages
func (sm *SessionManager) HandleBatteryMessage(msg whatsapp.BatteryMessage) {
	sm.BatteryChannel <- BatteryMsg{msg.Percentage, msg.Plugged, msg.Powersave}
}

func (sm *SessionManager) HandleContactList(contacts []whatsapp.Contact) {
	for _, c := range contacts {
		sm.ContactChannel <- c
	}
}

func (sm *SessionManager) HandleChatList(chats []whatsapp.Chat) {
	for _, c := range chats {
		sm.ChatChannel <- c
	}
}

func (sm *SessionManager) HandleJsonMessage(message string) {
	//sm.uiHandler.PrintText(message)
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

// notify will send a notification via beeep if EnableNotification is true. If
// UseTerminalBell is true it will send a terminal bell instead.
func notify(title, message string) error {
	if !config.Config.General.EnableNotifications {
		return nil
	} else if config.Config.General.UseTerminalBell {
		_, err := fmt.Printf("\a")
		return err
	}
	return beeep.Notify(title, message, "")
}
