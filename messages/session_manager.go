package messages

import (
	"errors"
	"time"
)

type Backend interface {
	Start(chan interface{}) error
	Stop() error
	Command(string, []string) error
}

type SessionManager struct {
	backChannel     chan interface{}
	CommandChannel  chan Command
	db              *MessageDatabase
	backend         Backend
	started         bool
	uiHandler       UiMessageHandler
	currentReceiver string
	statusInfo      SessionStatus
	lastSent        time.Time
}

func NewSessionManager(handler UiMessageHandler, back Backend) *SessionManager {
	sm := &SessionManager{}
	sm.db = NewMessageDatabase()
	sm.uiHandler = handler
	sm.backend = back
	sm.backChannel = make(chan interface{}, 10)
	sm.CommandChannel = make(chan Command, 10)
	return sm
}

func (sm *SessionManager) StartManager() error {
	if sm.started {
		return errors.New("session manager running, send commands to control")
	}
	sm.started = true
	go sm.runManager()
	return nil
}

func (sm *SessionManager) runManager() {
	sm.backend.Start(sm.backChannel)
	for sm.started == true {
		select {
		case cmd := <-sm.CommandChannel:
			sm.backend.Command(cmd.Name, cmd.Params)
		case in := <-sm.backChannel:
			switch msg := in.(type) {
			default:
			case Message:
				didNew := sm.db.Message(&msg)
				if msg.ChatId == sm.currentReceiver {
					if didNew {
						sm.uiHandler.NewMessage(msg)
					} else {
						screen := sm.db.GetMessages(sm.currentReceiver)
						sm.uiHandler.NewScreen(screen)
					}
					// notify if chat is in focus and we didn't send a message recently
					// TODO: move notify to UI
					//if int64(msg.Timestamp) > time.Now().Unix()-30 {
					//  if int64(msg.Timestamp) > sm.lastSent.Unix()+config.Config.General.NotificationTimeout {
					//    sm.db.NewUnreadChat(msg.ChatId)
					//    if !msg.FromMe {
					//      err := notify(sm.db.GetIdShort(msg.Info.RemoteJid), msg.Text)
					//      if err != nil {
					//        sm.uiHandler.PrintError(err)
					//      }
					//    }
					//  }
					//}
				} else {
					// notify if message is younger than 30 sec and not in focus
					//if int64(msg.Info.Timestamp) > time.Now().Unix()-30 {
					//  sm.db.NewUnreadChat(msg.Info.RemoteJid)
					//  if !msg.Info.FromMe {
					//    err := notify(sm.db.GetIdShort(msg.Info.RemoteJid), msg.Text)
					//    if err != nil {
					//      sm.uiHandler.PrintError(err)
					//    }
					//  }
					//}
				}
				sm.uiHandler.SetChats(sm.db.GetChatIds())
			case Contact:
				sm.db.AddContact(msg)
				sm.uiHandler.SetChats(sm.db.GetChatIds())
			case Chat:
				sm.db.AddChat(msg)
				sm.uiHandler.SetChats(sm.db.GetChatIds())
			case BatteryMsg:
				sm.statusInfo.BatteryLoading = msg.loading
				sm.statusInfo.BatteryPowersave = msg.powersave
				sm.statusInfo.BatteryCharge = msg.charge
				sm.uiHandler.SetStatus(sm.statusInfo)
			case StatusMsg:
				prevStatus := sm.statusInfo.Connected
				if msg.err != nil {
				} else {
					sm.statusInfo.Connected = msg.connected
				}
				//TODO: check connection?
				//wac := sm.getConnection()
				//connected := wac.GetConnected()
				//connected := true
				//sm.statusInfo.Connected = connected
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
	}
	sm.backend.Stop()
}
