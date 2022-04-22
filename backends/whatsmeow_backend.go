package backends

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	_ "mime"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"github.com/normen/whatscli/messages"
)

var dbDialect = flag.String("db-dialect", "sqlite3", "Database dialect (sqlite3 or postgres)")
var dbAddress = flag.String("db-address", "file:mdtest.db?_foreign_keys=on", "Database address")

type MeowBackend struct {
	msgdb       *sql.DB
	cli         *whatsmeow.Client
	uiHandler   messages.UiMessageHandler
	backChannel chan interface{}
}

func NewMeowBackend(handler messages.UiMessageHandler) *MeowBackend {
	b := &MeowBackend{}
	//db, err := sql.Open(dialect, address)
	//TODO: remove handler
	b.uiHandler = handler
	return b
}

func (b *MeowBackend) Warnf(text string, parms ...interface{}) {
	//b.uiHandler.PrintText(fmt.Sprintf(text, parms))
}

func (b *MeowBackend) Errorf(text string, parms ...interface{}) {
	b.uiHandler.PrintText(fmt.Sprintf(text, parms))
}

func (b *MeowBackend) Infof(text string, parms ...interface{}) {
	//b.uiHandler.PrintText(fmt.Sprintf(text, parms))
}

func (b *MeowBackend) Debugf(text string, parms ...interface{}) {
	//b.uiHandler.PrintText(fmt.Sprintf(text, parms))
}

func (b *MeowBackend) Logf(text string, parms ...interface{}) {
	b.uiHandler.PrintText(fmt.Sprintf(text, parms))
}

func (b *MeowBackend) Sub(module string) waLog.Logger {
	return b
}

func (b *MeowBackend) Command(cmd string, args []string) error {
	switch cmd {
	case "backlog":
		//TODO
	case "disconnect":
		b.cli.Disconnect()
	case "connect":
		err := b.cli.Connect()
		if err != nil {
			b.Errorf("Failed to connect: %v", err)
		}
	case "login":
		err := b.cli.Connect()
		if err != nil {
			b.Errorf("Failed to connect: %v", err)
		}
	case "logout":
		err := b.cli.Logout()
		if err != nil {
			b.Errorf("Error logging out: %v", err)
		} else {
			b.Logf("Successfully logged out")
		}
	case "send":
		if len(args) < 2 {
			b.Errorf("Usage: send <jid> <text>")
			return nil
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return nil
		}
		msg := &waProto.Message{Conversation: proto.String(strings.Join(args[1:], " "))}
		_, err := b.cli.SendMessage(recipient, "", msg)
		if err != nil {
			b.Errorf("Error sending message: %v", err)
		}
	case "presence":
		b.Logf("%s", b.cli.SendPresence(types.Presence(args[0])))
	case "chatpresence":
		jid, _ := types.ParseJID(args[1])
		b.Logf("%s", b.cli.SendChatPresence(types.ChatPresence(args[0]), jid))
	case "privacysettings":
		resp, err := b.cli.TryFetchPrivacySettings(false)
		if err != nil {
			b.Errorf("%s", err)
		} else {
			b.Logf("%+v\n", resp)
		}
	case "getgroup":
		if len(args) < 1 {
			b.Errorf("Usage: getgroup <jid>")
			return nil
		}
		group, ok := parseJID(args[0])
		if !ok {
			return nil
		} else if group.Server != types.GroupServer {
			b.Errorf("Input must be a group JID (@%s)", types.GroupServer)
			return nil
		}
		resp, err := b.cli.GetGroupInfo(group)
		if err != nil {
			b.Errorf("Failed to get group info: %v", err)
		} else {
			b.Logf("Group info: %+v", resp)
		}
	case "listgroups":
		groups, err := b.cli.GetJoinedGroups()
		if err != nil {
			b.Errorf("Failed to get group list: %v", err)
		} else {
			for _, group := range groups {
				b.Logf("%+v", group)
			}
		}
	case "getinvitelink":
		if len(args) < 1 {
			b.Errorf("Usage: getinvitelink <jid> [--reset]")
			return nil
		}
		group, ok := parseJID(args[0])
		if !ok {
			return nil
		} else if group.Server != types.GroupServer {
			b.Errorf("Input must be a group JID (@%s)", types.GroupServer)
			return nil
		}
		resp, err := b.cli.GetGroupInviteLink(group, len(args) > 1 && args[1] == "--reset")
		if err != nil {
			b.Errorf("Failed to get group invite link: %v", err)
		} else {
			b.Logf("Group invite link: %s", resp)
		}
	case "queryinvitelink":
		if len(args) < 1 {
			b.Errorf("Usage: queryinvitelink <link>")
			return nil
		}
		resp, err := b.cli.GetGroupInfoFromLink(args[0])
		if err != nil {
			b.Errorf("Failed to resolve group invite link: %v", err)
		} else {
			b.Logf("Group info: %+v", resp)
		}
	case "querybusinesslink":
		if len(args) < 1 {
			b.Errorf("Usage: querybusinesslink <link>")
			return nil
		}
		resp, err := b.cli.ResolveBusinessMessageLink(args[0])
		if err != nil {
			b.Errorf("Failed to resolve business message link: %v", err)
		} else {
			b.Logf("Business info: %+v", resp)
		}
	case "joininvitelink":
		if len(args) < 1 {
			b.Errorf("Usage: acceptinvitelink <link>")
			return nil
		}
		groupID, err := b.cli.JoinGroupWithLink(args[0])
		if err != nil {
			b.Errorf("Failed to join group via invite link: %v", err)
		} else {
			b.Logf("Joined %s", groupID)
		}
	case "sendimg":
		if len(args) < 2 {
			b.Errorf("Usage: sendimg <jid> <image path> [caption]")
			return nil
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return nil
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			b.Errorf("Failed to read %s: %v", args[0], err)
			return nil
		}
		uploaded, err := b.cli.Upload(context.Background(), data, whatsmeow.MediaImage)
		if err != nil {
			b.Errorf("Failed to upload file: %v", err)
			return nil
		}
		msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(strings.Join(args[2:], " ")),
			Url:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSha256: uploaded.FileEncSHA256,
			FileSha256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		}}
		_, err = b.cli.SendMessage(recipient, "", msg)
		if err != nil {
			b.Errorf("Error sending image message: %v", err)
		}
	}
	return nil
}

func (b *MeowBackend) Start(bkChan chan interface{}) error {
	b.backChannel = bkChan
	//dbLog := waLog.Stdout("Database", logLevel, true)
	storeContainer, err := sqlstore.New(*dbDialect, *dbAddress, b)
	if err != nil {
		return err
	}
	device, err := storeContainer.GetFirstDevice()
	if err != nil {
		return err
	}
	b.cli = whatsmeow.NewClient(device, b)
	ch, err := b.cli.GetQRChannel(context.Background())
	if err != nil {
		// This error means that we're already logged in, so ignore it.
		if !errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			return err
		}
	} else {
		go func() {
			for evt := range ch {
				if evt.Event == "code" {
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, b.uiHandler.GetWriter())
				} else {
					b.Infof("QR channel result: %s", evt.Event)
					//fmt.Printf("QR channel result: %s\n", evt.Event)
				}
			}
		}()
	}
	b.cli.AddEventHandler(b.handler)
	err = b.cli.Connect()
	if err != nil {
		return err
	}
	return nil
}

func (b *MeowBackend) syncAllChats() {

}

func (b *MeowBackend) syncAllContacts() {
	if contacts, err := b.cli.Store.Contacts.GetAllContacts(); err == nil {
		for jid, contact := range contacts {
			newcont := messages.Contact{
				Id:    jid.String(),
				Name:  contact.FullName,
				Short: contact.PushName,
			}
			b.backChannel <- newcont
		}
	}
}

var historySyncID int32
var startupTime = time.Now().Unix()

func (b *MeowBackend) handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.AppStateSyncComplete:
		if len(b.cli.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			err := b.cli.SendPresence(types.PresenceAvailable)
			if err != nil {
				b.Infof("AppSync: Failed to send available presence: %v", err)
			} else {
				b.Infof("AppSync: Marked self as available")
			}
		}
	case *events.Connected, *events.PushNameSetting:
		if len(b.cli.Store.PushName) == 0 {
			return
		}
		// Send presence available when connecting and when the pushname is changed.
		// This makes sure that outgoing messages always have the right pushname.
		err := b.cli.SendPresence(types.PresenceAvailable)
		if err != nil {
			b.Infof("Failed to send available presence: %v", err)
		}
		b.syncAllContacts()
	case *events.Message:
		message := &messages.Message{
			Id:        evt.Info.ID,
			ChatId:    evt.Info.Chat.String(),
			ContactId: evt.Info.Sender.String(),
			Timestamp: uint64(evt.Info.Timestamp.Unix()),
			FromMe:    evt.Info.IsFromMe,
			Forwarded: false, //TODO
			Text:      evt.Message.GetConversation(),
		}
		b.getExtendedMessage(evt.Message, message)
		b.backChannel <- message
	case *events.Receipt:
		if evt.Type == events.ReceiptTypeRead || evt.Type == events.ReceiptTypeReadSelf {
			b.Infof("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
		} else if evt.Type == events.ReceiptTypeDelivered {
			b.Infof("%s was delivered to %s at %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp)
		}
	case *events.Presence:
		if evt.Unavailable {
			if evt.LastSeen.IsZero() {
				b.Infof("%s is now offline", evt.From)
			} else {
				b.Infof("%s is now offline (last seen: %s)", evt.From, evt.LastSeen)
			}
		} else {
			b.Infof("%s is now online", evt.From)
		}
	case *events.HistorySync:
		b.syncAllContacts()
		for _, conversation := range evt.Data.Conversations {
			chat := messages.Chat{
				Id:          conversation.GetId(),
				Name:        conversation.GetName(),
				Unread:      int(conversation.GetUnreadCount()),
				LastMessage: conversation.GetConversationTimestamp(),
			}
			if conversation.Name != nil {
				chat.IsGroup = true
			}
			b.backChannel <- chat
			for _, msg := range conversation.Messages {
				var contact = msg.Message.Key.Participant
				if contact == nil {
					contact = msg.Message.Key.RemoteJid
				}
				if msg.Message.Message != nil {
					message := messages.Message{
						Id:        *msg.Message.Key.Id,
						ContactId: *contact,
						FromMe:    *msg.Message.Key.FromMe,
						ChatId:    *conversation.Id,
						Timestamp: *msg.Message.MessageTimestamp,
						Forwarded: false, //TODO
						Text:      msg.Message.Message.GetConversation(),
					}
					b.getExtendedMessage(msg.Message.Message, &message)
					b.backChannel <- message
				}
			}
		}
	case *events.AppState:
		b.Infof("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
	case *events.Contact:
		b.Infof("Contact coming in")
		contact := &messages.Contact{
			Id:    evt.JID.String(),
			Name:  *evt.Action.FullName,
			Short: *evt.Action.FullName,
		}
		b.backChannel <- contact
	}
}

func (b *MeowBackend) getExtendedMessage(msg *waProto.Message, message *messages.Message) {
	if msg != nil {
		pmsg := msg.GetProtocolMessage()
		if pmsg != nil {
			message.Id = pmsg.Key.GetId()
			message.ContactId = pmsg.Key.GetRemoteJid()
			message.FromMe = pmsg.Key.GetFromMe()
		}
		img := msg.ImageMessage
		if img != nil {
			message.MediaLink = img.GetDirectPath()
			message.MessageType = img.GetMimetype()
			message.MediaData1 = img.GetMediaKey()
			message.MediaData2 = img.GetFileSha256()
			message.MediaData3 = img.GetFileEncSha256()
			message.Text = "[IMAGE] " + img.GetCaption()
		}
		audio := msg.AudioMessage
		if audio != nil {
			message.MediaLink = audio.GetDirectPath()
			message.MessageType = audio.GetMimetype()
			message.MediaData1 = audio.GetMediaKey()
			message.MediaData2 = audio.GetFileSha256()
			message.MediaData3 = audio.GetFileEncSha256()
			message.Text = "[AUDIO]"
		}
		doc := msg.DocumentMessage
		if doc != nil {
			message.MediaLink = doc.GetDirectPath()
			message.MessageType = doc.GetMimetype()
			message.MediaData1 = doc.GetMediaKey()
			message.MediaData2 = doc.GetFileSha256()
			message.MediaData3 = doc.GetFileEncSha256()
			message.Text = "[DOC]"
		}
	}
}

func (b *MeowBackend) Stop() error {
	b.cli.Disconnect()
	b.backChannel = nil
	return nil
}

func (b *MeowBackend) Download(message *messages.Message, path string) error {
	//exts, _ := mime.ExtensionsByType(message.MessageType)
	dlmsg := NewDlMsg(message)
	data, err := b.cli.Download(dlmsg)
	if err != nil {
		return err
	}
	err = os.WriteFile(path, data, 0600)
	if err != nil {
		return err
	}
	return nil
}

func parseJID(arg string) (types.JID, bool) {
	if arg[0] == '+' {
		arg = arg[1:]
	}
	if !strings.ContainsRune(arg, '@') {
		return types.NewJID(arg, types.DefaultUserServer), true
	} else {
		recipient, err := types.ParseJID(arg)
		if err != nil {
			//log.Errorf("Invalid JID %s: %v", arg, err)
			return recipient, false
		} else if recipient.User == "" {
			//log.Errorf("Invalid JID %s: no server specified", arg)
			return recipient, false
		}
		return recipient, true
	}
}

// wrapper for downloading messages using whatsmeow
type DlMsg struct {
	proto.Message
	m *messages.Message
}

func NewDlMsg(message *messages.Message) *DlMsg {
	return &DlMsg{m: message}
}

func (m *DlMsg) GetDirectPath() string {
	return m.m.MediaLink
}
func (m *DlMsg) GetMediaKey() []byte {
	return m.m.MediaData1
}
func (m *DlMsg) GetFileSha256() []byte {
	return m.m.MediaData2
}
func (m *DlMsg) GetFileEncSha256() []byte {
	return m.m.MediaData3
}
