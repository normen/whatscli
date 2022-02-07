package messages

import (
	"context"
	//"encoding/json"
	"errors"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	//"github.com/normen/whatscli/config"
	//"github.com/normen/whatscli/qrcode"
	//"github.com/rivo/tview"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"mime"
	//"net/http"
	"os"
	//"os/signal"
	//"strings"
	//"sync/atomic"
	//"syscall"
	//"time"
)

var logLevel = "INFO"
var debugLogs = flag.Bool("debug", false, "Enable debug logs?")
var dbDialect = flag.String("db-dialect", "sqlite3", "Database dialect (sqlite3 or postgres)")
var dbAddress = flag.String("db-address", "file:mdtest.db?_foreign_keys=on", "Database address")

type MeowBackend struct {
	uiHandler       UiMessageHandler
	CommandChannel  chan Command
	cli             *whatsmeow.Client
	currentReceiver string
}

func NewMeowBackend(handler UiMessageHandler) *MeowBackend {
	b := &MeowBackend{}
	b.uiHandler = handler
	b.CommandChannel = make(chan Command, 10)
	return b
}

func (b *MeowBackend) StartManager() error {
	dbLog := waLog.Stdout("Database", logLevel, true)
	storeContainer, err := sqlstore.New(*dbDialect, *dbAddress, dbLog)
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
					fmt.Printf("QR channel result: %s\n", evt.Event)
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

// HandleError implements the error handler interface for go-whatsapp
func (b *MeowBackend) HandleError(err error) {
	b.uiHandler.PrintError(err)
	//TODO
	//statusMsg := StatusMsg{false, err}
	//b.StatusChannel <- statusMsg
	return
}

func (b *MeowBackend) handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.AppStateSyncComplete:
		if len(b.cli.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			err := b.cli.SendPresence(types.PresenceAvailable)
			if err != nil {
				//log.Warnf("Failed to send available presence: %v", err)
			} else {
				//log.Infof("Marked self as available")
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
			//log.Warnf("Failed to send available presence: %v", err)
		} else {
			//log.Infof("Marked self as available")
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

		//log.Infof("Received message %s from %s (%s): %+v", evt.Info.ID, evt.Info.SourceString(), strings.Join(metaParts, ", "), evt.Message)

		img := evt.Message.GetImageMessage()
		if img != nil {
			data, err := b.cli.Download(img)
			if err != nil {
				//log.Errorf("Failed to download image: %v", err)
				return
			}
			exts, _ := mime.ExtensionsByType(img.GetMimetype())
			path := fmt.Sprintf("%s%s", evt.Info.ID, exts[0])
			err = os.WriteFile(path, data, 0600)
			if err != nil {
				//log.Errorf("Failed to save image: %v", err)
				return
			}
			//log.Infof("Saved image in message to %s", path)
		}
	case *events.Receipt:
		if evt.Type == events.ReceiptTypeRead || evt.Type == events.ReceiptTypeReadSelf {
			//log.Infof("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
		} else if evt.Type == events.ReceiptTypeDelivered {
			//log.Infof("%s was delivered to %s at %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp)
		}
	case *events.Presence:
		if evt.Unavailable {
			if evt.LastSeen.IsZero() {
				//log.Infof("%s is now offline", evt.From)
			} else {
				//log.Infof("%s is now offline (last seen: %s)", evt.From, evt.LastSeen)
			}
		} else {
			//log.Infof("%s is now online", evt.From)
		}
	case *events.HistorySync:
		//id := atomic.AddInt32(&historySyncID, 1)
		//fileName := fmt.Sprintf("history-%d-%d.json", startupTime, id)
		//file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
		//if err != nil {
		//log.Errorf("Failed to open file to write history sync: %v", err)
		//return
		//}
		//enc := json.NewEncoder(file)
		//enc.SetIndent("", "  ")
		//err = enc.Encode(evt.Data)
		//if err != nil {
		//log.Errorf("Failed to write history sync: %v", err)
		//return
		//}
		//log.Infof("Wrote history sync to %s", fileName)
		//_ = file.Close()
	case *events.AppState:
		//log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
	}
}
