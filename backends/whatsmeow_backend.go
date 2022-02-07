package backends

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"mime"
	"os"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/normen/whatscli/messages"
)

var logLevel = "INFO"
var debugLogs = flag.Bool("debug", false, "Enable debug logs?")
var dbDialect = flag.String("db-dialect", "sqlite3", "Database dialect (sqlite3 or postgres)")
var dbAddress = flag.String("db-address", "file:mdtest.db?_foreign_keys=on", "Database address")

type MeowBackend struct {
	cli         *whatsmeow.Client
	uiHandler   messages.UiMessageHandler
	backChannel chan interface{}
}

func NewMeowBackend(handler messages.UiMessageHandler) *MeowBackend {
	b := &MeowBackend{}
	//TODO: remove handler
	b.uiHandler = handler
	return b
}

func (b *MeowBackend) Command(cmd string, args []string) error {
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
					b.uiHandler.PrintText(fmt.Sprintf("QR channel result: %s", evt.Event))
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

var historySyncID int32
var startupTime = time.Now().Unix()

func (sm *MeowBackend) handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.AppStateSyncComplete:
		if len(sm.cli.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			err := sm.cli.SendPresence(types.PresenceAvailable)
			if err != nil {
				sm.uiHandler.PrintText(fmt.Sprintf("Failed to send available presence: %v", err))
			} else {
				sm.uiHandler.PrintText(fmt.Sprintf("Marked self as available"))
			}
		}
	case *events.Connected, *events.PushNameSetting:
		if len(sm.cli.Store.PushName) == 0 {
			return
		}
		// Send presence available when connecting and when the pushname is changed.
		// This makes sure that outgoing messages always have the right pushname.
		err := sm.cli.SendPresence(types.PresenceAvailable)
		if err != nil {
			sm.uiHandler.PrintText(fmt.Sprintf("Failed to send available presence: %v", err))
		} else {
			sm.uiHandler.PrintText(fmt.Sprintf("Marked self as available"))
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

		sm.uiHandler.PrintText(fmt.Sprintf("Received message %s from %s (%s): %+v", evt.Info.ID, evt.Info.SourceString(), strings.Join(metaParts, ", "), evt.Message))

		img := evt.Message.GetImageMessage()
		if img != nil {
			data, err := sm.cli.Download(img)
			if err != nil {
				sm.uiHandler.PrintText(fmt.Sprintf("Failed to download image: %v", err))
				return
			}
			exts, _ := mime.ExtensionsByType(img.GetMimetype())
			path := fmt.Sprintf("%s%s", evt.Info.ID, exts[0])
			err = os.WriteFile(path, data, 0600)
			if err != nil {
				sm.uiHandler.PrintText(fmt.Sprintf("Failed to save image: %v", err))
				return
			}
			sm.uiHandler.PrintText(fmt.Sprintf("Saved image in message to %s", path))
		}
		message := &messages.Message{
			Id:           evt.Info.ID,
			ChatId:       evt.Info.Chat.String(),
			ContactId:    evt.Info.Sender.String(),
			ContactName:  evt.Info.PushName,
			ContactShort: evt.Info.PushName, //TODO
			Timestamp:    uint64(evt.Info.Timestamp.Unix()),
			FromMe:       evt.Info.IsFromMe,
			Forwarded:    false, //TODO
			Text:         evt.Message.String(),
			Orig:         evt,
		}
		sm.backChannel <- message
	case *events.Receipt:
		if evt.Type == events.ReceiptTypeRead || evt.Type == events.ReceiptTypeReadSelf {
			sm.uiHandler.PrintText(fmt.Sprintf("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp))
		} else if evt.Type == events.ReceiptTypeDelivered {
			sm.uiHandler.PrintText(fmt.Sprintf("%s was delivered to %s at %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp))
		}
	case *events.Presence:
		if evt.Unavailable {
			if evt.LastSeen.IsZero() {
				sm.uiHandler.PrintText(fmt.Sprintf("%s is now offline", evt.From))
			} else {
				sm.uiHandler.PrintText(fmt.Sprintf("%s is now offline (last seen: %s)", evt.From, evt.LastSeen))
			}
		} else {
			sm.uiHandler.PrintText(fmt.Sprintf("%s is now online", evt.From))
		}
	case *events.HistorySync:
		id := atomic.AddInt32(&historySyncID, 1)
		fileName := fmt.Sprintf("history-%d-%d.json", startupTime, id)
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			sm.uiHandler.PrintText(fmt.Sprintf("Failed to open file to write history sync: %v", err))
			return
		}
		enc := json.NewEncoder(file)
		enc.SetIndent("", "  ")
		err = enc.Encode(evt.Data)
		if err != nil {
			sm.uiHandler.PrintText(fmt.Sprintf("Failed to write history sync: %v", err))
			return
		}
		sm.uiHandler.PrintText(fmt.Sprintf("Wrote history sync to %s", fileName))
		_ = file.Close()
	case *events.AppState:
		sm.uiHandler.PrintText(fmt.Sprintf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue))
	}
}

func (b *MeowBackend) Stop() error {
	b.backChannel = nil
	return nil
}
