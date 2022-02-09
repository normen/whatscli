package backends

import (
	"context"
	"go.mau.fi/whatsmeow/appstate"
	//"encoding/json"
	"errors"
	"flag"
	"fmt"
	//"mime"
	"net/http"
	"os"
	"strings"
	//"sync/atomic"
	"time"

	"database/sql"
	//"database/sql/driver"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"

	"go.mau.fi/whatsmeow"
	//"go.mau.fi/whatsmeow/appstate"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"github.com/normen/whatscli/messages"
)

var logLevel = "ERROR"
var debugLogs = flag.Bool("debug", false, "Enable debug logs?")
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

func (b *MeowBackend) logf(text string, parms ...interface{}) {
	b.uiHandler.PrintText(fmt.Sprintf(text, parms))
}

func (b *MeowBackend) errf(text string, parms ...interface{}) {
	b.uiHandler.PrintText(fmt.Sprintf(text, parms))
}

func (b *MeowBackend) Command(cmd string, args []string) error {
	switch cmd {
	case "reconnect":
		b.cli.Disconnect()
		err := b.cli.Connect()
		if err != nil {
			b.errf("Failed to connect: %v", err)
		}
	case "logout":
		err := b.cli.Logout()
		if err != nil {
			b.errf("Error logging out: %v", err)
		} else {
			b.logf("Successfully logged out")
		}
	case "appstate":
		if len(args) < 1 {
			b.errf("Usage: appstate <types...>")
			return nil
		}
		names := []appstate.WAPatchName{appstate.WAPatchName(args[0])}
		if args[0] == "all" {
			names = []appstate.WAPatchName{appstate.WAPatchRegular, appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow, appstate.WAPatchCriticalUnblockLow, appstate.WAPatchCriticalBlock}
		}
		resync := len(args) > 1 && args[1] == "resync"
		for _, name := range names {
			err := b.cli.FetchAppState(name, resync, false)
			if err != nil {
				b.errf("Failed to sync app state: %v", err)
			}
		}
	case "checkuser":
		if len(args) < 1 {
			b.errf("Usage: checkuser <phone numbers...>")
			return nil
		}
		resp, err := b.cli.IsOnWhatsApp(args)
		if err != nil {
			b.errf("Failed to check if users are on WhatsApp:", err)
		} else {
			for _, item := range resp {
				if item.VerifiedName != nil {
					b.logf("%s: on whatsapp: %t, JID: %s, business name: %s", item.Query, item.IsIn, item.JID, item.VerifiedName.Details.GetVerifiedName())
				} else {
					b.logf("%s: on whatsapp: %t, JID: %s", item.Query, item.IsIn, item.JID)
				}
			}
		}
	case "subscribepresence":
		if len(args) < 1 {
			b.errf("Usage: subscribepresence <jid>")
			return nil
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return nil
		}
		err := b.cli.SubscribePresence(jid)
		if err != nil {
			fmt.Println(err)
		}
	case "presence":
		b.logf("%s", b.cli.SendPresence(types.Presence(args[0])))
	case "chatpresence":
		jid, _ := types.ParseJID(args[1])
		b.logf("%s", b.cli.SendChatPresence(types.ChatPresence(args[0]), jid))
	case "privacysettings":
		resp, err := b.cli.TryFetchPrivacySettings(false)
		if err != nil {
			b.logf("%s", err)
		} else {
			b.logf("%+v\n", resp)
		}
	case "getuser":
		if len(args) < 1 {
			b.errf("Usage: getuser <jids...>")
			return nil
		}
		var jids []types.JID
		for _, arg := range args {
			jid, ok := parseJID(arg)
			if !ok {
				return nil
			}
			jids = append(jids, jid)
		}
		resp, err := b.cli.GetUserInfo(jids)
		if err != nil {
			b.errf("Failed to get user info: %v", err)
		} else {
			for jid, info := range resp {
				b.logf("%s: %+v", jid, info)
			}
		}
	case "getavatar":
		if len(args) < 1 {
			b.errf("Usage: getavatar <jid>")
			return nil
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return nil
		}
		pic, err := b.cli.GetProfilePictureInfo(jid, len(args) > 1 && args[1] == "preview")
		if err != nil {
			b.errf("Failed to get avatar: %v", err)
		} else if pic != nil {
			b.logf("Got avatar ID %s: %s", pic.ID, pic.URL)
		} else {
			b.logf("No avatar found")
		}
	case "getgroup":
		if len(args) < 1 {
			b.errf("Usage: getgroup <jid>")
			return nil
		}
		group, ok := parseJID(args[0])
		if !ok {
			return nil
		} else if group.Server != types.GroupServer {
			b.errf("Input must be a group JID (@%s)", types.GroupServer)
			return nil
		}
		resp, err := b.cli.GetGroupInfo(group)
		if err != nil {
			b.errf("Failed to get group info: %v", err)
		} else {
			b.logf("Group info: %+v", resp)
		}
	case "listgroups":
		groups, err := b.cli.GetJoinedGroups()
		if err != nil {
			b.errf("Failed to get group list: %v", err)
		} else {
			for _, group := range groups {
				b.logf("%+v", group)
			}
		}
	case "getinvitelink":
		if len(args) < 1 {
			b.errf("Usage: getinvitelink <jid> [--reset]")
			return nil
		}
		group, ok := parseJID(args[0])
		if !ok {
			return nil
		} else if group.Server != types.GroupServer {
			b.errf("Input must be a group JID (@%s)", types.GroupServer)
			return nil
		}
		resp, err := b.cli.GetGroupInviteLink(group, len(args) > 1 && args[1] == "--reset")
		if err != nil {
			b.errf("Failed to get group invite link: %v", err)
		} else {
			b.logf("Group invite link: %s", resp)
		}
	case "queryinvitelink":
		if len(args) < 1 {
			b.errf("Usage: queryinvitelink <link>")
			return nil
		}
		resp, err := b.cli.GetGroupInfoFromLink(args[0])
		if err != nil {
			b.errf("Failed to resolve group invite link: %v", err)
		} else {
			b.logf("Group info: %+v", resp)
		}
	case "querybusinesslink":
		if len(args) < 1 {
			b.errf("Usage: querybusinesslink <link>")
			return nil
		}
		resp, err := b.cli.ResolveBusinessMessageLink(args[0])
		if err != nil {
			b.errf("Failed to resolve business message link: %v", err)
		} else {
			b.logf("Business info: %+v", resp)
		}
	case "joininvitelink":
		if len(args) < 1 {
			b.errf("Usage: acceptinvitelink <link>")
			return nil
		}
		groupID, err := b.cli.JoinGroupWithLink(args[0])
		if err != nil {
			b.errf("Failed to join group via invite link: %v", err)
		} else {
			b.logf("Joined %s", groupID)
		}
	case "send":
		if len(args) < 2 {
			b.errf("Usage: send <jid> <text>")
			return nil
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return nil
		}
		msg := &waProto.Message{Conversation: proto.String(strings.Join(args[1:], " "))}
		ts, err := b.cli.SendMessage(recipient, "", msg)
		if err != nil {
			b.errf("Error sending message: %v", err)
		} else {
			b.logf("Message sent (server timestamp: %s)", ts)
		}
	case "sendimg":
		if len(args) < 2 {
			b.errf("Usage: sendimg <jid> <image path> [caption]")
			return nil
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return nil
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			b.errf("Failed to read %s: %v", args[0], err)
			return nil
		}
		uploaded, err := b.cli.Upload(context.Background(), data, whatsmeow.MediaImage)
		if err != nil {
			b.errf("Failed to upload file: %v", err)
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
		ts, err := b.cli.SendMessage(recipient, "", msg)
		if err != nil {
			b.errf("Error sending image message: %v", err)
		} else {
			b.logf("Image message sent (server timestamp: %s)", ts)
		}
	}
	return nil
}

func (b *MeowBackend) Start(bkChan chan interface{}) error {
	b.backChannel = bkChan
	//dbLog := waLog.Stdout("Database", logLevel, true)
	storeContainer, err := sqlstore.New(*dbDialect, *dbAddress, waLog.Noop)
	if err != nil {
		return err
	}
	device, err := storeContainer.GetFirstDevice()
	if err != nil {
		return err
	}
	b.cli = whatsmeow.NewClient(device, waLog.Stdout("Client", logLevel, true))
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
					b.logf("QR channel result: %s", evt.Event)
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
				b.logf("AppSync: Failed to send available presence: %v", err)
			} else {
				b.logf("AppSync: Marked self as available")
			}
		}
	case *events.Connected, *events.PushNameSetting:
		b.syncAllContacts()
		if len(b.cli.Store.PushName) == 0 {
			return
		}
		//b.cli.FetchAppState(,true,false)
		// Send presence available when connecting and when the pushname is changed.
		// This makes sure that outgoing messages always have the right pushname.
		err := b.cli.SendPresence(types.PresenceAvailable)
		if err != nil {
			b.logf("Failed to send available presence: %v", err)
		} else {
			b.logf("Marked self as available")
		}
	case *events.StreamReplaced:
		os.Exit(0)
	case *events.Message:
		metaParts := []string{fmt.Sprintf("pushname: %s", evt.Info.PushName), fmt.Sprintf("timestamp: %s", evt.Info.Timestamp)}
		if evt.Info.Type != "" {
			metaParts = append(metaParts, fmt.Sprintf("type: %s", evt.Info.Type))
		}
		if evt.Info.Category != "" {
			metaParts = append(metaParts, fmt.Sprintf("category: %s", evt.Info.Category))
		}
		if evt.IsViewOnce {
			metaParts = append(metaParts, "view once")
		}
		if evt.IsViewOnce {
			metaParts = append(metaParts, "ephemeral")
		}

		b.logf("Received message %s from %s (%s): %+v", evt.Info.ID, evt.Info.SourceString(), strings.Join(metaParts, ", "), evt.Message)

		//img := evt.Message.GetImageMessage()
		//if img != nil {
		//  data, err := b.cli.Download(img)
		//  if err != nil {
		//    b.logf("Failed to download image: %v", err)
		//    return
		//  }
		//  exts, _ := mime.ExtensionsByType(img.GetMimetype())
		//  path := fmt.Sprintf("%s%s", evt.Info.ID, exts[0])
		//  err = os.WriteFile(path, data, 0600)
		//  if err != nil {
		//    b.logf("Failed to save image: %v", err)
		//    return
		//  }
		//  b.logf("Saved image in message to %s", path)
		//}
		//rmessage := messages.Message{
		//  Id:           *msg.Message.Key.Id,
		//  ChatId:       *conversation.Id,
		//  ContactId:    *msg.Message.Key.RemoteJid,
		//  ContactName:  "NAME",
		//  ContactShort: "SHORT", //TODO
		//  Timestamp:    *msg.Message.MessageTimestamp,
		//  FromMe:       *msg.Message.Key.FromMe,
		//  Forwarded:    false, //TODO
		//  Text:         *msg.Message.Message.Conversation,
		//}
		message := &messages.Message{
			Id:        evt.Info.ID,
			ChatId:    evt.Info.Chat.String(),
			ContactId: evt.Info.Sender.String(),
			//ContactName:  evt.Info.PushName,
			//ContactShort: evt.Info.PushName, //TODO
			Timestamp: uint64(evt.Info.Timestamp.Unix()),
			FromMe:    evt.Info.IsFromMe,
			Forwarded: false, //TODO
			Text:      evt.Message.GetConversation(),
			//Orig:         evt, //TODO:needed?
		}
		b.backChannel <- message
	case *events.Receipt:
		if evt.Type == events.ReceiptTypeRead || evt.Type == events.ReceiptTypeReadSelf {
			b.logf("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
		} else if evt.Type == events.ReceiptTypeDelivered {
			b.logf("%s was delivered to %s at %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp)
		}
	case *events.Presence:
		if evt.Unavailable {
			if evt.LastSeen.IsZero() {
				b.logf("%s is now offline", evt.From)
			} else {
				b.logf("%s is now offline (last seen: %s)", evt.From, evt.LastSeen)
			}
		} else {
			b.logf("%s is now online", evt.From)
		}
	case *events.HistorySync:
		//id := atomic.AddInt32(&historySyncID, 1)
		//fileName := fmt.Sprintf("history-%d-%d.json", startupTime, id)
		//file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
		//if err != nil {
		//  b.logf("Failed to open file to write history sync: %v", err)
		//  return
		//}
		//enc := json.NewEncoder(file)
		//enc.SetIndent("", "  ")
		//err = enc.Encode(evt.Data)
		//if err != nil {
		//  b.logf("Failed to write history sync: %v", err)
		//  return
		//}
		//b.logf("Wrote history sync to %s", fileName)
		//_ = file.Close()

		for _, conversation := range evt.Data.Conversations {
			//TODO: add chats here
			for _, msg := range conversation.Messages {
				if msg.Message.Message != nil {
					message := messages.Message{
						Id:        *msg.Message.Key.Id,
						ContactId: *msg.Message.Key.RemoteJid,
						FromMe:    *msg.Message.Key.FromMe,
						ChatId:    *conversation.Id,
						//ContactName:  "NAME",
						//ContactShort: "SHORT", //TODO
						Timestamp: *msg.Message.MessageTimestamp,
						Forwarded: false, //TODO
						Text:      msg.Message.Message.GetConversation(),
					}
					//b.uiHandler.PrintText("Found " + message.ContactId)
					b.backChannel <- message
				}
			}
		}
	case *events.AppState:
		b.logf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
	case *events.Contact:
		b.logf("Contact coming in")
		contact := &messages.Contact{
			Id:    evt.JID.String(),
			Name:  *evt.Action.FullName,
			Short: *evt.Action.FullName,
		}
		b.backChannel <- contact
	}
}

func (b *MeowBackend) Stop() error {
	b.cli.Disconnect()
	b.backChannel = nil
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
