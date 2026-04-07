package messages

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gen2brain/beeep"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/qrcode"
	"github.com/rivo/tview"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

var urlPattern = regexp.MustCompile(`https?://[^\s]+`)

// SessionManager deals with the connection and receives commands from the UI.
type SessionManager struct {
	db              *MessageDatabase
	currentReceiver string
	uiHandler       UiMessageHandler
	client          *whatsmeow.Client
	container       *sqlstore.Container
	BatteryChannel  chan BatteryMsg
	StatusChannel   chan StatusMsg
	CommandChannel  chan Command
	ChatChannel     chan Chat
	ContactChannel  chan Contact
	TextChannel     chan *waProto.Message
	statusInfo      SessionStatus
	lastSent        time.Time
	started         bool
	eventHandler    *eventHandler
}

// Init initializes the SessionManager.
func (sm *SessionManager) Init(handler UiMessageHandler) {
	sm.db = &MessageDatabase{}
	sm.db.Init()
	sm.uiHandler = handler
	sm.BatteryChannel = make(chan BatteryMsg, 10)
	sm.StatusChannel = make(chan StatusMsg, 10)
	sm.CommandChannel = make(chan Command, 10)
	sm.ChatChannel = make(chan Chat, 10)
	sm.ContactChannel = make(chan Contact, 10)
	sm.TextChannel = make(chan *waProto.Message, 10)
	sm.eventHandler = &eventHandler{sm: sm}
}

// StartManager starts the receiver and message handling goroutine.
func (sm *SessionManager) StartManager() error {
	if sm.started {
		return errors.New("session manager running, send commands to control")
	}
	sm.started = true
	go sm.runManager()
	return nil
}

func (sm *SessionManager) runManager() error {
	client, err := sm.getConnection()
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("failed to create WhatsApp connection: %v", err))
		return err
	}
	if client == nil {
		return errors.New("could not establish WhatsApp connection")
	}

	if err = sm.loginWithConnection(client); err != nil {
		sm.uiHandler.PrintError(err)
	}

	for sm.started {
		select {
		case command := <-sm.CommandChannel:
			sm.execCommand(command)
		case batteryMsg := <-sm.BatteryChannel:
			sm.statusInfo.BatteryLoading = batteryMsg.loading
			sm.statusInfo.BatteryPowersave = batteryMsg.powersave
			sm.statusInfo.BatteryCharge = batteryMsg.charge
			sm.uiHandler.SetStatus(sm.statusInfo)
		case statusMsg := <-sm.StatusChannel:
			prevStatus := sm.statusInfo.Connected
			if statusMsg.err == nil {
				sm.statusInfo.Connected = statusMsg.connected
			}
			if sm.client != nil {
				sm.statusInfo.Connected = sm.client.IsConnected()
			} else {
				sm.statusInfo.Connected = false
			}
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
	if sm.client != nil {
		sm.client.Disconnect()
	}
	return nil
}

func (sm *SessionManager) setCurrentReceiver(id string) {
	sm.currentReceiver = id
	sm.uiHandler.NewScreen(sm.getMessages(id))
}

func (sm *SessionManager) getConnection() (*whatsmeow.Client, error) {
	if sm.client == nil {
		dbPath := config.GetSessionFilePath() + ".db"
		container, err := sqlstore.New(context.Background(), "sqlite3", "file:"+dbPath+"?_foreign_keys=on", waLog.Noop)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %v", err)
		}
		deviceStore, err := container.GetFirstDevice(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get device: %v", err)
		}
		client := whatsmeow.NewClient(deviceStore, waLog.Noop)
		client.AddEventHandler(sm.eventHandler.Handle)
		sm.client = client
		sm.container = container
	}
	return sm.client, nil
}

func (sm *SessionManager) login() error {
	sm.client = nil
	client, err := sm.getConnection()
	if err != nil {
		return fmt.Errorf("failed to create WhatsApp connection: %v", err)
	}
	return sm.loginWithConnection(client)
}

func (sm *SessionManager) loginWithConnection(client *whatsmeow.Client) error {
	sm.uiHandler.PrintText("connecting..")
	if client.IsConnected() {
		client.Disconnect()
		sm.StatusChannel <- StatusMsg{false, nil}
		time.Sleep(500 * time.Millisecond)
	}

	if client.Store.ID == nil {
		return sm.loginWithQRCode(client)
	}

	if err := client.Connect(); err != nil {
		if errors.Is(err, whatsmeow.ErrNotConnected) || errors.Is(err, whatsmeow.ErrNotLoggedIn) {
			sm.uiHandler.PrintText("Session expired, need to scan QR code again")
			if delErr := client.Store.Delete(context.Background()); delErr != nil {
				return fmt.Errorf("failed to clear expired session: %v", delErr)
			}
			sm.client = nil
			client, err = sm.getConnection()
			if err != nil {
				return fmt.Errorf("failed to create new connection: %v", err)
			}
			return sm.loginWithQRCode(client)
		}
		return fmt.Errorf("connection failed: %v", err)
	}

	sm.uiHandler.PrintText("Session restored successfully")
	sm.StatusChannel <- StatusMsg{true, nil}
	go sm.loadRecentChats()
	return nil
}

func (sm *SessionManager) loginWithQRCode(client *whatsmeow.Client) error {
	sm.uiHandler.PrintText("Please scan the QR code with your phone")
	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize QR channel: %v", err)
	}
	if err = client.Connect(); err != nil {
		return fmt.Errorf("error connecting to WhatsApp: %v", err)
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			terminal := qrcode.New()
			terminal.SetOutput(tview.ANSIWriter(sm.uiHandler.GetWriter()))
			terminal.Get(evt.Code).Print()
		case "success":
			sm.uiHandler.PrintText("Successfully logged in!")
			sm.StatusChannel <- StatusMsg{true, nil}
			go sm.loadRecentChats()
			return nil
		default:
			sm.uiHandler.PrintText("QR event: " + evt.Event)
		}
	}
	return errors.New("QR code channel closed without success")
}

func (sm *SessionManager) loadRecentChats() {
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	sm.loadContacts()
	addedChats := 0

	if sm.client.Store != nil && sm.client.Store.Contacts != nil {
		contacts, err := sm.client.Store.Contacts.GetAllContacts(context.Background())
		if err == nil {
			for jid, contact := range contacts {
				if jid.Server != types.DefaultUserServer {
					continue
				}
				name := contact.FullName
				if name == "" {
					name = contact.PushName
				}
				if name == "" {
					name = jid.User
				}
				sm.db.AddChat(Chat{
					Id:      jid.String(),
					IsGroup: false,
					Name:    name,
				})
				addedChats++
			}
		}
	}

	groups, err := sm.client.GetJoinedGroups(context.Background())
	if err == nil {
		for _, group := range groups {
			sm.db.AddChat(Chat{
				Id:      group.JID.String(),
				IsGroup: true,
				Name:    group.Name,
			})
			addedChats++
		}
	}

	sm.uiHandler.SetChats(sm.db.GetChatIds())
	if addedChats > 0 {
		sm.uiHandler.PrintText(fmt.Sprintf("Loaded %d chats", addedChats))
	}
}

func (sm *SessionManager) loadContacts() {
	if sm.client == nil || sm.client.Store == nil || sm.client.Store.Contacts == nil {
		return
	}

	contacts, err := sm.client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("failed to load contacts: %v", err))
		return
	}

	contactCount := 0
	for jid, contact := range contacts {
		name := contact.FullName
		if name == "" {
			name = contact.PushName
		}
		if name == "" {
			name = jid.User
		}
		sm.db.AddContact(Contact{
			Id:    jid.String(),
			Name:  name,
			Short: contact.PushName,
		})
		contactCount++
	}
	if contactCount > 0 {
		sm.uiHandler.PrintText(fmt.Sprintf("Loaded %d contacts", contactCount))
	}
}

func (sm *SessionManager) getChatName(jid types.JID) string {
	if jid.Server == types.GroupServer {
		groupInfo, err := sm.client.GetGroupInfo(context.Background(), jid)
		if err == nil && groupInfo.Name != "" {
			return groupInfo.Name
		}
	}
	if sm.client != nil && sm.client.Store != nil && sm.client.Store.Contacts != nil {
		contact, err := sm.client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.Found {
			if contact.FullName != "" {
				return contact.FullName
			}
			if contact.PushName != "" {
				return contact.PushName
			}
		}
	}
	return sm.db.GetIdName(jid.String())
}

func (sm *SessionManager) disconnect() error {
	if sm.client != nil && sm.client.IsConnected() {
		sm.client.Disconnect()
		sm.StatusChannel <- StatusMsg{false, nil}
	}
	return nil
}

func (sm *SessionManager) logout() error {
	if sm.client == nil {
		sm.StatusChannel <- StatusMsg{false, nil}
		sm.uiHandler.PrintText("Already logged out")
		return nil
	}

	if sm.client.Store != nil && sm.client.Store.ID != nil {
		if err := sm.client.Logout(context.Background()); err != nil && !errors.Is(err, whatsmeow.ErrNotConnected) {
			sm.uiHandler.PrintText("Warning: Couldn't fully log out: " + err.Error())
		}
	}
	sm.client = nil
	sm.container = nil
	sm.StatusChannel <- StatusMsg{false, nil}
	sm.uiHandler.PrintText("Successfully logged out")
	return nil
}

func (sm *SessionManager) execCommand(command Command) {
	switch command.Name {
	default:
		sm.uiHandler.PrintText("[" + config.Config.Colors.Negative + "]Unknown command: [-]" + command.Name)
	case "backlog":
		sm.loadBacklog()
	case "login", "connect":
		err := sm.login()
		if err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("WhatsApp connection failed: %v", err))
			sm.uiHandler.PrintText("Try using /reset to completely reset the connection")
		} else {
			sm.uiHandler.PrintText("Successfully connected to WhatsApp")
		}
	case "reset":
		sm.resetSession()
	case "disconnect":
		sm.uiHandler.PrintError(sm.disconnect())
	case "logout":
		sm.uiHandler.PrintError(sm.logout())
	case "send":
		if checkParam(command.Params, 2) {
			sm.sendText(command.Params[0], strings.Join(command.Params[1:], " "))
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
		sm.markCurrentChatRead()
	case "info":
		if checkParam(command.Params, 1) {
			sm.uiHandler.PrintText(sm.db.GetMessageInfo(command.Params[0]))
		} else {
			sm.printCommandUsage("info", "[message-id[]")
		}
	case "download":
		sm.downloadCommand(command.Params, false, false)
	case "open":
		sm.downloadCommand(command.Params, true, false)
	case "show":
		sm.downloadCommand(command.Params, true, true)
	case "url":
		sm.openMessageURL(command.Params)
	case "upload":
		sm.sendMediaCommand(command.Params, MessageKindDocument)
	case "sendimage":
		sm.sendMediaCommand(command.Params, MessageKindImage)
	case "sendvideo":
		sm.sendMediaCommand(command.Params, MessageKindVideo)
	case "sendaudio":
		sm.sendMediaCommand(command.Params, MessageKindAudio)
	case "revoke":
		sm.revokeMessage(command.Params)
	case "leave":
		sm.leaveCurrentGroup()
	case "create":
		sm.createGroup(command.Params)
	case "add":
		sm.updateCurrentGroupParticipants(command.Params, whatsmeow.ParticipantChangeAdd, "add", "added new members")
	case "remove":
		sm.updateCurrentGroupParticipants(command.Params, whatsmeow.ParticipantChangeRemove, "remove", "removed members")
	case "admin":
		sm.updateCurrentGroupParticipants(command.Params, whatsmeow.ParticipantChangePromote, "admin", "promoted members")
	case "removeadmin":
		sm.updateCurrentGroupParticipants(command.Params, whatsmeow.ParticipantChangeDemote, "removeadmin", "demoted members")
	case "subject":
		sm.updateCurrentGroupSubject(command.Params)
	case "colorlist":
		out := ""
		for idx := range tcell.ColorNames {
			out += "[" + idx + "]" + idx + "[-]\n"
		}
		sm.uiHandler.PrintText(out)
	case "more":
		sm.loadBacklog()
	}
}

func (sm *SessionManager) loadBacklog() {
	if sm.currentReceiver == "" {
		sm.printCommandUsage("backlog", "-> only works in a chat")
		return
	}
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	jid, err := types.ParseJID(sm.currentReceiver)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("invalid JID: %v", err))
		return
	}

	existingMessages := sm.db.GetMessages(sm.currentReceiver)
	sm.uiHandler.PrintText("Retrieving message history...")

	if oldest, ok := sm.db.GetOldestMessage(sm.currentReceiver); ok {
		senderJID := types.EmptyJID
		if oldest.SenderId != "" {
			if parsedSender, parseErr := types.ParseJID(oldest.SenderId); parseErr == nil {
				senderJID = parsedSender
			}
		}
		req := sm.client.BuildHistorySyncRequest(&types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     jid,
				Sender:   senderJID,
				IsFromMe: oldest.FromMe,
				IsGroup:  strings.Contains(sm.currentReceiver, GROUPSUFFIX),
			},
			ID:        types.MessageID(oldest.Id),
			Timestamp: time.Unix(int64(oldest.Timestamp), 0),
		}, config.Config.General.BacklogMsgQuantity)
		if _, err = sm.client.SendMessage(context.Background(), jid, req); err == nil {
			time.Sleep(2 * time.Second)
		}
	}

	if len(sm.db.GetMessages(sm.currentReceiver)) == len(existingMessages) {
		err = sm.client.MarkRead(context.Background(), []types.MessageID{}, time.Now(), jid, jid, types.ReceiptTypeRead)
		if err != nil {
			sm.uiHandler.PrintText(fmt.Sprintf("Note: Could not send read receipt: %v", err))
		}
		time.Sleep(2 * time.Second)
	}

	updated := sm.db.GetMessages(sm.currentReceiver)
	if len(updated) > len(existingMessages) {
		sm.uiHandler.PrintText(fmt.Sprintf("Loaded %d additional messages", len(updated)-len(existingMessages)))
	} else {
		sm.uiHandler.PrintText("No additional messages found. WhatsApp may limit history access.")
	}
	sm.uiHandler.NewScreen(updated)
}

func (sm *SessionManager) resetSession() {
	if sm.client != nil {
		if sm.client.IsConnected() {
			sm.client.Disconnect()
		}
		if sm.client.Store != nil {
			if err := sm.client.Store.Delete(context.Background()); err != nil {
				sm.uiHandler.PrintText("Warning: Couldn't remove session: " + err.Error())
			}
		}
	}

	sm.client = nil
	sm.container = nil
	dbPath := config.GetSessionFilePath() + ".db"
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		sm.uiHandler.PrintText("Warning: Couldn't remove database file: " + err.Error())
	}
	sm.StatusChannel <- StatusMsg{false, nil}
	sm.uiHandler.PrintText("Session reset. Use /connect to reconnect with a new QR code.")
}

func (sm *SessionManager) markCurrentChatRead() {
	if sm.currentReceiver == "" {
		sm.printCommandUsage("read", "-> only works in a chat")
		return
	}
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	chatJID, err := types.ParseJID(sm.currentReceiver)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("invalid JID: %v", err))
		return
	}

	unreadMessages := sm.db.MarkChatRead(sm.currentReceiver)
	if len(unreadMessages) == 0 {
		sm.uiHandler.SetChats(sm.db.GetChatIds())
		sm.uiHandler.PrintText("No unread messages in current chat")
		return
	}

	type senderBatch struct {
		sender    types.JID
		ids       []types.MessageID
		timestamp time.Time
	}
	batches := make(map[string]*senderBatch)
	for _, msg := range unreadMessages {
		sender := chatJID
		if strings.Contains(sm.currentReceiver, GROUPSUFFIX) && msg.SenderId != "" {
			sender, err = types.ParseJID(msg.SenderId)
			if err != nil {
				continue
			}
		}
		key := sender.String()
		if _, ok := batches[key]; !ok {
			batches[key] = &senderBatch{sender: sender}
		}
		batches[key].ids = append(batches[key].ids, types.MessageID(msg.Id))
		ts := time.Unix(int64(msg.Timestamp), 0)
		if ts.After(batches[key].timestamp) {
			batches[key].timestamp = ts
		}
	}

	for _, batch := range batches {
		if batch.timestamp.IsZero() {
			batch.timestamp = time.Now()
		}
		if err := sm.client.MarkRead(context.Background(), batch.ids, batch.timestamp, chatJID, batch.sender); err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("failed to mark messages as read: %v", err))
		}
	}

	sm.uiHandler.SetChats(sm.db.GetChatIds())
}

func (sm *SessionManager) downloadCommand(params []string, preview, show bool) {
	if !checkParam(params, 1) {
		name := "download"
		if preview && !show {
			name = "open"
		} else if show {
			name = "show"
		}
		sm.printCommandUsage(name, "[message-id[]")
		return
	}

	msg, ok := sm.db.GetMessage(params[0])
	if !ok {
		sm.uiHandler.PrintError(errors.New("message not found"))
		return
	}
	if show && msg.Kind != MessageKindImage {
		sm.uiHandler.PrintError(errors.New("show only works for image messages"))
		return
	}

	path, err := sm.downloadMessage(msg, preview)
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	if show {
		sm.uiHandler.PrintFile(path)
		return
	}
	if preview {
		sm.uiHandler.OpenFile(path)
		return
	}
	sm.uiHandler.PrintText("[::d] -> " + path + "[::-]")
}

func (sm *SessionManager) openMessageURL(params []string) {
	if !checkParam(params, 1) {
		sm.printCommandUsage("url", "[message-id[]")
		return
	}
	msg, ok := sm.db.GetMessage(params[0])
	if !ok {
		sm.uiHandler.PrintError(errors.New("message not found"))
		return
	}
	url := urlPattern.FindString(msg.Text)
	if url == "" {
		sm.uiHandler.PrintText("No URL found in message")
		return
	}
	sm.uiHandler.OpenFile(url)
}

func (sm *SessionManager) sendMediaCommand(params []string, kind MessageKind) {
	if sm.currentReceiver == "" {
		sm.printCommandUsage(commandNameForKind(kind), "-> only works in a chat")
		return
	}
	if !checkParam(params, 1) {
		sm.printCommandUsage(commandNameForKind(kind), "/path/to/file")
		return
	}
	path := strings.Join(params, " ")
	sm.uiHandler.PrintError(sm.sendMedia(sm.currentReceiver, path, kind))
}

func (sm *SessionManager) revokeMessage(params []string) {
	if !checkParam(params, 1) {
		sm.printCommandUsage("revoke", "[message-id[]")
		return
	}
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	msg, ok := sm.db.GetMessage(params[0])
	if !ok {
		sm.uiHandler.PrintError(errors.New("message not found"))
		return
	}
	chatJID, err := types.ParseJID(msg.ChatId)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("invalid chat JID: %v", err))
		return
	}
	if _, err = sm.client.RevokeMessage(context.Background(), chatJID, types.MessageID(msg.Id)); err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	sm.db.MarkMessageRevoked(msg.Id)
	if sm.currentReceiver == msg.ChatId {
		sm.uiHandler.NewScreen(sm.getMessages(msg.ChatId))
	}
	sm.uiHandler.PrintText("revoked: " + msg.Id)
}

func (sm *SessionManager) leaveCurrentGroup() {
	groupJID, err := sm.currentGroupJID()
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	if err = sm.client.LeaveGroup(context.Background(), groupJID); err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	sm.uiHandler.PrintText("left group " + groupJID.String())
}

func (sm *SessionManager) createGroup(params []string) {
	if !checkParam(params, 1) {
		sm.printCommandUsage("create", "[user-id[] [user-id[] New Group Subject")
		sm.printCommandUsage("create", "New Group Subject")
		return
	}

	participants := make([]types.JID, 0)
	idx := 0
	for idx < len(params) && strings.Contains(params[idx], CONTACTSUFFIX) {
		participant, err := types.ParseJID(params[idx])
		if err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("invalid user id %q: %v", params[idx], err))
			return
		}
		participants = append(participants, participant)
		idx++
	}

	name := strings.Join(params[idx:], " ")
	if name == "" {
		name = strings.Join(params, " ")
		participants = nil
	}

	groupInfo, err := sm.client.CreateGroup(context.Background(), whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: participants,
	})
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}

	sm.db.AddChat(Chat{
		Id:          groupInfo.JID.String(),
		IsGroup:     true,
		Name:        groupInfo.Name,
		LastMessage: time.Now().Unix(),
	})
	sm.uiHandler.SetChats(sm.db.GetChatIds())
	sm.uiHandler.PrintText("created new group " + groupInfo.JID.String())
}

func (sm *SessionManager) updateCurrentGroupParticipants(params []string, action whatsmeow.ParticipantChange, command, success string) {
	groupJID, err := sm.currentGroupJID()
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	if !checkParam(params, 1) {
		sm.printCommandUsage(command, "[user-id[]")
		return
	}

	participants := make([]types.JID, 0, len(params))
	for _, raw := range params {
		jid, err := types.ParseJID(raw)
		if err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("invalid user id %q: %v", raw, err))
			return
		}
		participants = append(participants, jid)
	}

	if _, err = sm.client.UpdateGroupParticipants(context.Background(), groupJID, participants, action); err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	sm.uiHandler.PrintText(success + " for " + groupJID.String())
}

func (sm *SessionManager) updateCurrentGroupSubject(params []string) {
	groupJID, err := sm.currentGroupJID()
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	if !checkParam(params, 1) {
		sm.printCommandUsage("subject", "new-subject -> in group chat")
		return
	}

	name := strings.Join(params, " ")
	if err = sm.client.SetGroupName(context.Background(), groupJID, name); err != nil {
		sm.uiHandler.PrintError(err)
		return
	}

	sm.db.AddChat(Chat{
		Id:      groupJID.String(),
		IsGroup: true,
		Name:    name,
	})
	sm.uiHandler.SetChats(sm.db.GetChatIds())
	sm.uiHandler.PrintText("updated subject for " + groupJID.String())
}

func (sm *SessionManager) currentGroupJID() (types.JID, error) {
	if sm.currentReceiver == "" || !strings.Contains(sm.currentReceiver, GROUPSUFFIX) {
		return types.JID{}, errors.New("not a group")
	}
	return types.ParseJID(sm.currentReceiver)
}

func (sm *SessionManager) printCommandUsage(command, usage string) {
	sm.uiHandler.PrintText("[" + config.Config.Colors.Negative + "]Usage:[-] " + command + " " + usage)
}

func checkParam(arr []string, length int) bool {
	return arr != nil && len(arr) >= length
}

func (sm *SessionManager) getMessages(wid string) []Message {
	return sm.db.GetMessages(wid)
}

func (sm *SessionManager) sendText(wid, text string) {
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	receiver, err := types.ParseJID(wid)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("invalid JID: %v", err))
		return
	}

	raw := &waProto.Message{Conversation: proto.String(text)}
	sm.lastSent = time.Now()
	resp, err := sm.client.SendMessage(context.Background(), receiver, raw)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("failed to send message: %v", err))
		return
	}

	newMsg := sm.outgoingMessageFromSendResponse(resp, wid, raw, MessageKindText, text, "", "")
	sm.db.AddMessage(newMsg, false)
	if sm.currentReceiver == wid {
		sm.uiHandler.NewMessage(newMsg)
	}
	sm.uiHandler.SetChats(sm.db.GetChatIds())
}

func (sm *SessionManager) sendMedia(chatID, path string, kind MessageKind) error {
	if sm.client == nil || !sm.client.IsConnected() {
		return errors.New("not connected to WhatsApp")
	}

	data, mimeType, fileName, err := readUploadFile(path)
	if err != nil {
		return err
	}

	receiver, err := types.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid JID: %v", err)
	}

	uploadResp, err := sm.client.Upload(context.Background(), data, uploadMediaType(kind))
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	fileLength := uploadResp.FileLength
	raw := &waProto.Message{}
	switch kind {
	case MessageKindImage:
		raw.ImageMessage = &waProto.ImageMessage{
			Mimetype:      proto.String(mimeType),
			URL:           &uploadResp.URL,
			DirectPath:    &uploadResp.DirectPath,
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    &fileLength,
		}
	case MessageKindVideo:
		raw.VideoMessage = &waProto.VideoMessage{
			Mimetype:      proto.String(mimeType),
			URL:           &uploadResp.URL,
			DirectPath:    &uploadResp.DirectPath,
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    &fileLength,
		}
	case MessageKindAudio:
		raw.AudioMessage = &waProto.AudioMessage{
			Mimetype:      proto.String(mimeType),
			URL:           &uploadResp.URL,
			DirectPath:    &uploadResp.DirectPath,
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    &fileLength,
			PTT:           proto.Bool(false),
		}
	case MessageKindDocument:
		raw.DocumentMessage = &waProto.DocumentMessage{
			Mimetype:      proto.String(mimeType),
			Title:         proto.String(fileName),
			FileName:      proto.String(fileName),
			URL:           &uploadResp.URL,
			DirectPath:    &uploadResp.DirectPath,
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    &fileLength,
		}
	default:
		return errors.New("unsupported media type")
	}

	sm.lastSent = time.Now()
	resp, err := sm.client.SendMessage(context.Background(), receiver, raw)
	if err != nil {
		return fmt.Errorf("failed to send media message: %v", err)
	}

	text := mediaDisplayText(kind, fileName, "")
	newMsg := sm.outgoingMessageFromSendResponse(resp, chatID, raw, kind, text, mimeType, fileName)
	sm.db.AddMessage(newMsg, false)
	if sm.currentReceiver == chatID {
		sm.uiHandler.NewMessage(newMsg)
	}
	sm.uiHandler.SetChats(sm.db.GetChatIds())
	return nil
}

func (sm *SessionManager) outgoingMessageFromSendResponse(resp whatsmeow.SendResponse, chatID string, raw *waProto.Message, kind MessageKind, text, mimeType, fileName string) Message {
	selfID := ""
	if sm.client != nil && sm.client.Store != nil && sm.client.Store.ID != nil {
		selfID = sm.client.Store.ID.String()
	}

	contactID := chatID
	if strings.Contains(chatID, GROUPSUFFIX) {
		contactID = selfID
	}

	return Message{
		Id:           string(resp.ID),
		ChatId:       chatID,
		SenderId:     selfID,
		ContactId:    contactID,
		ContactName:  sm.db.GetIdName(contactID),
		ContactShort: sm.db.GetIdShort(contactID),
		Timestamp:    uint64(resp.Timestamp.Unix()),
		FromMe:       true,
		Text:         text,
		Kind:         kind,
		MimeType:     mimeType,
		FileName:     fileName,
		RawMessage:   raw,
	}
}

func notify(title, message string) error {
	if !config.Config.General.EnableNotifications {
		return nil
	} else if config.Config.General.UseTerminalBell {
		_, err := fmt.Printf("\a")
		return err
	}
	return beeep.Notify(title, message, "")
}

type eventHandler struct {
	sm *SessionManager
}

func (eh *eventHandler) Handle(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		eh.handleLiveMessage(v)
	case *events.HistorySync:
		eh.handleHistorySync(v)
	case *events.Connected:
		eh.sm.StatusChannel <- StatusMsg{true, nil}
	case *events.Disconnected:
		eh.sm.StatusChannel <- StatusMsg{false, nil}
	case *events.LoggedOut:
		eh.sm.StatusChannel <- StatusMsg{false, nil}
		eh.sm.uiHandler.PrintText("Logged out: " + fmt.Sprintf("%v", v.Reason))
	}
}

func (eh *eventHandler) handleLiveMessage(evt *events.Message) {
	msg, action, ok := eh.normalizeEventMessage(evt)
	if !ok {
		return
	}

	switch action {
	case "revoke":
		if eh.sm.db.MarkMessageRevoked(msg.Id) && eh.sm.currentReceiver == msg.ChatId {
			eh.sm.uiHandler.NewScreen(eh.sm.getMessages(msg.ChatId))
		}
		eh.sm.uiHandler.SetChats(eh.sm.db.GetChatIds())
		return
	case "ignore":
		return
	}

	markUnread := !msg.FromMe && msg.ChatId != eh.sm.currentReceiver
	isNew := eh.sm.db.AddMessage(msg, markUnread)
	if msg.ChatId == eh.sm.currentReceiver {
		if isNew {
			eh.sm.uiHandler.NewMessage(msg)
		} else {
			eh.sm.uiHandler.NewScreen(eh.sm.getMessages(msg.ChatId))
		}
	} else if markUnread && msg.Timestamp > uint64(time.Now().Unix()-30) {
		if err := notify(msg.ContactShort, msg.Text); err != nil {
			eh.sm.uiHandler.PrintError(err)
		}
	}
	eh.sm.uiHandler.SetChats(eh.sm.db.GetChatIds())
}

func (eh *eventHandler) handleHistorySync(evt *events.HistorySync) {
	if evt == nil || evt.Data == nil {
		return
	}

	for _, conv := range evt.Data.GetConversations() {
		chatID := conv.GetID()
		if chatID == "" {
			chatID = conv.GetNewJID()
		}
		if chatID == "" {
			continue
		}

		chatJID, err := types.ParseJID(chatID)
		if err != nil {
			continue
		}

		chatName := conv.GetName()
		if chatName == "" {
			chatName = conv.GetDisplayName()
		}
		if chatName == "" {
			chatName = eh.sm.getChatName(chatJID)
		}

		lastMessage := int64(conv.GetLastMsgTimestamp())
		if lastMessage == 0 {
			lastMessage = int64(conv.GetConversationTimestamp())
		}
		eh.sm.db.AddChat(Chat{
			Id:          chatID,
			IsGroup:     chatJID.Server == types.GroupServer,
			Name:        chatName,
			Unread:      int(conv.GetUnreadCount()),
			LastMessage: lastMessage,
		})

		for _, histMsg := range conv.GetMessages() {
			webMsg := histMsg.GetMessage()
			if webMsg == nil {
				continue
			}
			parsed, err := eh.sm.client.ParseWebMessage(chatJID, webMsg)
			if err != nil {
				continue
			}
			msg, action, ok := eh.normalizeEventMessage(parsed)
			if !ok || action != "" {
				continue
			}
			eh.sm.db.AddMessage(msg, false)
		}
		eh.sm.db.UpdateChatUnread(chatID, int(conv.GetUnreadCount()))
	}

	eh.sm.uiHandler.SetChats(eh.sm.db.GetChatIds())
	if eh.sm.currentReceiver != "" {
		eh.sm.uiHandler.NewScreen(eh.sm.getMessages(eh.sm.currentReceiver))
	}
}

func (eh *eventHandler) normalizeEventMessage(evt *events.Message) (Message, string, bool) {
	if evt == nil || evt.Message == nil {
		return Message{}, "ignore", false
	}

	if protocol := evt.Message.GetProtocolMessage(); protocol != nil {
		if protocol.GetType() == waProto.ProtocolMessage_REVOKE && protocol.GetKey() != nil {
			return Message{
				Id:     protocol.GetKey().GetID(),
				ChatId: evt.Info.Chat.String(),
			}, "revoke", true
		}
		return Message{}, "ignore", false
	}

	msg, ok := eh.messageFromInfo(evt.Info, evt.Message)
	return msg, "", ok
}

func (eh *eventHandler) messageFromInfo(info types.MessageInfo, raw *waProto.Message) (Message, bool) {
	if raw == nil {
		return Message{}, false
	}

	chatID := info.Chat.String()
	if chatID == "" {
		return Message{}, false
	}

	contactID, contactName, contactShort := eh.contactForMessage(info)
	msg := Message{
		Id:           string(info.ID),
		ChatId:       chatID,
		SenderId:     info.Sender.String(),
		ContactId:    contactID,
		ContactName:  contactName,
		ContactShort: contactShort,
		Timestamp:    uint64(info.Timestamp.Unix()),
		FromMe:       info.IsFromMe,
		RawMessage:   raw,
	}

	switch {
	case raw.GetConversation() != "":
		msg.Kind = MessageKindText
		msg.Text = raw.GetConversation()
		return msg, true
	case raw.GetExtendedTextMessage() != nil:
		ext := raw.GetExtendedTextMessage()
		msg.Kind = MessageKindText
		msg.Text = ext.GetText()
		msg.Forwarded = ext.GetContextInfo().GetIsForwarded()
		return msg, true
	case raw.GetImageMessage() != nil:
		image := raw.GetImageMessage()
		msg.Kind = MessageKindImage
		msg.MimeType = image.GetMimetype()
		msg.Text = mediaDisplayText(MessageKindImage, "", image.GetCaption())
		msg.Forwarded = image.GetContextInfo().GetIsForwarded()
		return msg, true
	case raw.GetVideoMessage() != nil:
		video := raw.GetVideoMessage()
		msg.Kind = MessageKindVideo
		msg.MimeType = video.GetMimetype()
		msg.Text = mediaDisplayText(MessageKindVideo, "", video.GetCaption())
		msg.Forwarded = video.GetContextInfo().GetIsForwarded()
		return msg, true
	case raw.GetAudioMessage() != nil:
		audio := raw.GetAudioMessage()
		msg.Kind = MessageKindAudio
		msg.MimeType = audio.GetMimetype()
		msg.Text = mediaDisplayText(MessageKindAudio, "", "")
		msg.Forwarded = audio.GetContextInfo().GetIsForwarded()
		return msg, true
	case raw.GetDocumentMessage() != nil:
		doc := raw.GetDocumentMessage()
		msg.Kind = MessageKindDocument
		msg.MimeType = doc.GetMimetype()
		msg.FileName = doc.GetFileName()
		msg.Text = mediaDisplayText(MessageKindDocument, doc.GetFileName(), doc.GetCaption())
		msg.Forwarded = doc.GetContextInfo().GetIsForwarded()
		return msg, true
	default:
		return Message{}, false
	}
}

func (eh *eventHandler) contactForMessage(info types.MessageInfo) (string, string, string) {
	if info.IsGroup {
		id := info.Sender.String()
		return id, eh.getContactName(info.Sender), eh.getContactShort(info.Sender)
	}
	id := info.Chat.String()
	chat := info.Chat
	return id, eh.getContactName(chat), eh.getContactShort(chat)
}

func (eh *eventHandler) getContactName(jid types.JID) string {
	if eh.sm.client != nil && eh.sm.client.Store != nil && eh.sm.client.Store.Contacts != nil {
		contact, err := eh.sm.client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.Found {
			if contact.FullName != "" {
				return contact.FullName
			}
			if contact.PushName != "" {
				return contact.PushName
			}
		}
	}
	return eh.sm.db.GetIdName(jid.String())
}

func (eh *eventHandler) getContactShort(jid types.JID) string {
	if eh.sm.client != nil && eh.sm.client.Store != nil && eh.sm.client.Store.Contacts != nil {
		contact, err := eh.sm.client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.Found && contact.PushName != "" {
			return contact.PushName
		}
	}
	return eh.sm.db.GetIdShort(jid.String())
}

func (sm *SessionManager) downloadMessage(msg Message, preview bool) (string, error) {
	if sm.client == nil || !sm.client.IsConnected() {
		return "", errors.New("not connected to WhatsApp")
	}

	downloadable, err := downloadableFromMessage(msg)
	if err != nil {
		return "", err
	}

	baseDir := config.Config.General.DownloadPath
	if preview {
		baseDir = config.Config.General.PreviewPath
	}
	if err = os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}

	fileName := downloadFileName(msg)
	fullPath := filepath.Join(baseDir, fileName)
	if _, err = os.Stat(fullPath); err == nil {
		return fullPath, nil
	}

	data, err := sm.client.Download(context.Background(), downloadable)
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", err
	}
	return fullPath, nil
}

func downloadableFromMessage(msg Message) (whatsmeow.DownloadableMessage, error) {
	if msg.RawMessage == nil {
		return nil, errors.New("This is not a downloadable message")
	}
	switch msg.Kind {
	case MessageKindImage:
		if media := msg.RawMessage.GetImageMessage(); media != nil {
			return media, nil
		}
	case MessageKindVideo:
		if media := msg.RawMessage.GetVideoMessage(); media != nil {
			return media, nil
		}
	case MessageKindAudio:
		if media := msg.RawMessage.GetAudioMessage(); media != nil {
			return media, nil
		}
	case MessageKindDocument:
		if media := msg.RawMessage.GetDocumentMessage(); media != nil {
			return media, nil
		}
	}
	return nil, errors.New("This is not a downloadable message")
}

func readUploadFile(path string) ([]byte, string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", "", err
	}
	fileName := filepath.Base(path)
	mimeType := detectMimeType(path, data)
	return data, mimeType, fileName, nil
}

func detectMimeType(path string, data []byte) string {
	if len(data) == 0 {
		if extType := mime.TypeByExtension(filepath.Ext(path)); extType != "" {
			return stripMimeParams(extType)
		}
		return "application/octet-stream"
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	detected := stripMimeParams(http.DetectContentType(sample))
	if extType := mime.TypeByExtension(filepath.Ext(path)); extType != "" {
		extType = stripMimeParams(extType)
		if detected == "application/octet-stream" || strings.HasPrefix(extType, "audio/") || strings.HasPrefix(extType, "video/") {
			return extType
		}
	}
	return detected
}

func stripMimeParams(value string) string {
	if idx := strings.Index(value, ";"); idx >= 0 {
		return value[:idx]
	}
	return value
}

func downloadFileName(msg Message) string {
	if msg.FileName != "" {
		return msg.FileName
	}
	ext := ""
	if msg.MimeType != "" {
		if exts, err := mime.ExtensionsByType(msg.MimeType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}
	return msg.Id + ext
}

func uploadMediaType(kind MessageKind) whatsmeow.MediaType {
	switch kind {
	case MessageKindImage:
		return whatsmeow.MediaImage
	case MessageKindVideo:
		return whatsmeow.MediaVideo
	case MessageKindAudio:
		return whatsmeow.MediaAudio
	default:
		return whatsmeow.MediaDocument
	}
}

func commandNameForKind(kind MessageKind) string {
	switch kind {
	case MessageKindImage:
		return "sendimage"
	case MessageKindVideo:
		return "sendvideo"
	case MessageKindAudio:
		return "sendaudio"
	default:
		return "upload"
	}
}

func mediaDisplayText(kind MessageKind, fileName, caption string) string {
	label := "[FILE]"
	switch kind {
	case MessageKindImage:
		label = "[IMAGE]"
	case MessageKindVideo:
		label = "[VIDEO]"
	case MessageKindAudio:
		label = "[AUDIO]"
	case MessageKindDocument:
		label = "[DOCUMENT]"
	}
	parts := []string{label}
	if fileName != "" && kind == MessageKindDocument {
		parts = append(parts, fileName)
	}
	if caption != "" {
		parts = append(parts, caption)
	}
	return strings.Join(parts, " ")
}
