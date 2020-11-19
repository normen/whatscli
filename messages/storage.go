package messages

import (
	"errors"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Rhymen/go-whatsapp"
	"github.com/rivo/tview"
	"github.com/skratchdot/open-golang/open"
)

const GROUPSUFFIX = "@g.us"
const CONTACTSUFFIX = "@s.whatsapp.net"

type MessageDatabase struct {
	textMessages  map[string][]*whatsapp.TextMessage // text messages stored by RemoteJid
	messagesById  map[string]*whatsapp.TextMessage   // text messages stored by message ID
	latestMessage map[string]uint64                  // last message from RemoteJid
	otherMessages map[string]*interface{}            // other non-text messages, stored by ID
}

// initialize the database
func (db *MessageDatabase) Init() {
	//var this = *db
	(*db).textMessages = make(map[string][]*whatsapp.TextMessage)
	(*db).messagesById = make(map[string]*whatsapp.TextMessage)
	(*db).otherMessages = make(map[string]*interface{})
	(*db).latestMessage = make(map[string]uint64)
}

// add a text message to the database, stored by RemoteJid
func (db *MessageDatabase) AddTextMessage(msg *whatsapp.TextMessage) bool {
	//var this = *db
	var didNew = false
	var wid = msg.Info.RemoteJid
	if (*db).textMessages[wid] == nil {
		var newArr = []*whatsapp.TextMessage{}
		(*db).textMessages[wid] = newArr
		(*db).latestMessage[wid] = msg.Info.Timestamp
		didNew = true
	} else if (*db).latestMessage[wid] < msg.Info.Timestamp {
		(*db).latestMessage[wid] = msg.Info.Timestamp
		didNew = true
	}
	(*db).textMessages[wid] = append((*db).textMessages[wid], msg)
	(*db).messagesById[msg.Info.Id] = msg
	sort.Slice((*db).textMessages[wid], func(i, j int) bool {
		return (*db).textMessages[wid][i].Info.Timestamp < (*db).textMessages[wid][j].Info.Timestamp
	})
	return didNew
}

// add audio/video/image/doc message, stored by message id
func (db *MessageDatabase) AddOtherMessage(msg *interface{}) {
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
		(*db).otherMessages[id] = msg
	}
}

// get an array of all chat ids
func (db *MessageDatabase) GetContactIds() []string {
	//var this = *db
	keys := make([]string, len((*db).textMessages))
	i := 0
	for k := range (*db).textMessages {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool {
		return (*db).latestMessage[keys[i]] > (*db).latestMessage[keys[j]]
	})
	return keys
}

func (db *MessageDatabase) GetMessageInfo(id string) string {
	if _, ok := (*db).otherMessages[id]; ok {
		return "[yellow]OtherMessage[-]"
	}
	out := ""
	if msg, ok := (*db).messagesById[id]; ok {
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
	//var this = *db
	var out = ""
	var arr = []string{}
	for _, element := range (*db).textMessages[wid] {
		out += GetTextMessageString(element)
		out += "\n"
		arr = append(arr, element.Info.Id)
	}
	return out, arr
}

// create a formatted string with regions based on message ID from a text message
func GetTextMessageString(msg *whatsapp.TextMessage) string {
	out := ""
	text := tview.Escape((*msg).Text)
	tim := time.Unix(int64((*msg).Info.Timestamp), 0)
	out += "[\""
	out += (*msg).Info.Id
	out += "\"]"
	if (*msg).Info.FromMe { //msg from me
		out += "[-::d](" + tim.Format("02-01-06 15:04:05") + ") [blue::b]Me: [-::-]" + text
	} else if strings.Contains((*msg).Info.RemoteJid, GROUPSUFFIX) { // group msg
		userId := (*msg).Info.SenderJid
		out += "[-::d](" + tim.Format("02-01-06 15:04:05") + ") [green::b]" + GetIdShort(userId) + ": [-::-]" + text
	} else { // message from others
		out += "[-::d](" + tim.Format("02-01-06 15:04:05") + ") [green::b]" + GetIdShort((*msg).Info.RemoteJid) + ": [-::-]" + text
	}
	out += "[\"\"]"
	return out
}

// load data for message specified by message id TODO: support types
func (db *MessageDatabase) LoadMessageData(wid string) ([]byte, error) {
	if msg, ok := (*db).otherMessages[wid]; ok {
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
func (db *MessageDatabase) DownloadMessage(wid string, open bool) (string, error) {
	if msg, ok := (*db).otherMessages[wid]; ok {
		var fileName string = ""
		switch v := (*msg).(type) {
		default:
		case whatsapp.ImageMessage:
			if data, err := v.Download(); err == nil {
				fileName = v.Info.Id + "." + strings.TrimPrefix(v.Type, "image/")
				path, err := saveAttachment(data, fileName, open)
				return path, err
			} else {
				return fileName, err
			}
		case whatsapp.DocumentMessage:
			if data, err := v.Download(); err == nil {
				fileName = v.Info.Id + "." + strings.TrimPrefix(strings.TrimPrefix(v.Type, "application/"), "document/")
				path, err := saveAttachment(data, fileName, open)
				return path, err
			} else {
				return fileName, err
			}
		case whatsapp.AudioMessage:
			if data, err := v.Download(); err == nil {
				fileName = v.Info.Id + "." + strings.TrimPrefix(v.Type, "audio/")
				path, err := saveAttachment(data, fileName, open)
				return path, err
			} else {
				return fileName, err
			}
		case whatsapp.VideoMessage:
			if data, err := v.Download(); err == nil {
				fileName = v.Info.Id + "." + strings.TrimPrefix(v.Type, "video/")
				path, err := saveAttachment(data, fileName, open)
				return path, err
			} else {
				return fileName, err
			}
		}
	}
	return "", errors.New("No attachments found")
}

// helper to save an attachment and open it if specified
func saveAttachment(data []byte, fileName string, openIt bool) (string, error) {
	path := GetHomeDir() + "Downloads" + string(os.PathSeparator) + fileName
	err := ioutil.WriteFile(path, data, 0644)
	if err == nil {
		if openIt {
			open.Run(path)
		}
	} else {
		return path, err
	}
	return path, nil
}
