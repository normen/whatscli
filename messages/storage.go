package messages

import (
	"errors"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Rhymen/go-whatsapp"
	"github.com/normen/whatscli/config"
	"github.com/rivo/tview"
)

const GROUPSUFFIX = "@g.us"
const CONTACTSUFFIX = "@s.whatsapp.net"

type MessageDatabase struct {
	textMessages  map[string][]*whatsapp.TextMessage // text messages stored by RemoteJid
	messagesById  map[string]*whatsapp.TextMessage   // text messages stored by message ID
	latestMessage map[string]uint64                  // last message from RemoteJid
	otherMessages map[string]*interface{}            // other non-text messages, stored by ID
	mutex         sync.Mutex
}

// initialize the database
func (db *MessageDatabase) Init() {
	//var this = *db
	db.textMessages = make(map[string][]*whatsapp.TextMessage)
	db.messagesById = make(map[string]*whatsapp.TextMessage)
	db.otherMessages = make(map[string]*interface{})
	db.latestMessage = make(map[string]uint64)
}

// add a text message to the database, stored by RemoteJid
func (db *MessageDatabase) AddTextMessage(msg *whatsapp.TextMessage) bool {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	//var this = *db
	var didNew = false
	var wid = msg.Info.RemoteJid
	if db.textMessages[wid] == nil {
		var newArr = []*whatsapp.TextMessage{}
		db.textMessages[wid] = newArr
		db.latestMessage[wid] = msg.Info.Timestamp
		didNew = true
	} else if db.latestMessage[wid] < msg.Info.Timestamp {
		db.latestMessage[wid] = msg.Info.Timestamp
		didNew = true
	}
	db.textMessages[wid] = append(db.textMessages[wid], msg)
	db.messagesById[msg.Info.Id] = msg
	sort.Slice(db.textMessages[wid], func(i, j int) bool {
		return db.textMessages[wid][i].Info.Timestamp < db.textMessages[wid][j].Info.Timestamp
	})
	return didNew
}

// add audio/video/image/doc message, stored by message id
func (db *MessageDatabase) AddOtherMessage(msg *interface{}) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	var id = ""
	switch v := (*msg).(type) {
	default:
	case whatsapp.ImageMessage:
		id = v.Info.Id
	case whatsapp.DocumentMessage:
		id = v.Info.Id
	case whatsapp.AudioMessage:
		id = v.Info.Id
	case whatsapp.VideoMessage:
		id = v.Info.Id
	}
	if id != "" {
		db.otherMessages[id] = msg
	}
}

// get an array of all chat ids
func (db *MessageDatabase) GetContactIds() []string {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	//var this = *db
	keys := make([]string, len(db.textMessages))
	i := 0
	for k := range db.textMessages {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool {
		return db.latestMessage[keys[i]] > db.latestMessage[keys[j]]
	})
	return keys
}

func (db *MessageDatabase) GetMessageInfo(id string) string {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	if _, ok := db.otherMessages[id]; ok {
		return "[yellow]OtherMessage[-]"
	}
	out := ""
	if msg, ok := db.messagesById[id]; ok {
		out += "[yellow]ID: " + msg.Info.Id + "[-]\n"
		out += "[yellow]PushName: " + msg.Info.PushName + "[-]\n"
		out += "[yellow]RemoteJid: " + msg.Info.RemoteJid + "[-]\n"
		out += "[yellow]SenderJid: " + msg.Info.SenderJid + "[-]\n"
		out += "[yellow]Participant: " + msg.ContextInfo.Participant + "[-]\n"
		out += "[yellow]QuotedMessageID: " + msg.ContextInfo.QuotedMessageID + "[-]\n"
	}
	return out
}

// get a string containing all messages for a chat by chat id
func (db *MessageDatabase) GetMessagesString(wid string) (string, []string) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	//var this = *db
	var out = ""
	var arr = []string{}
	for _, element := range db.textMessages[wid] {
		out += GetTextMessageString(element)
		out += "\n"
		arr = append(arr, element.Info.Id)
	}
	return out, arr
}

// load data for message specified by message id TODO: support types
func (db *MessageDatabase) LoadMessageData(wid string) ([]byte, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	if msg, ok := db.otherMessages[wid]; ok {
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
func (db *MessageDatabase) DownloadMessage(wid string, preview bool) (string, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	if msg, ok := db.otherMessages[wid]; ok {
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

// helper to save an attachment and open it if specified
func saveAttachment(data []byte, path string) (string, error) {
	err := ioutil.WriteFile(path, data, 0644)
	return path, err
}
