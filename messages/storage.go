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
	textMessages  map[string][]whatsapp.TextMessage // text messages stored by RemoteJid
	latestMessage map[string]uint64                 // last message from RemoteJid
	otherMessages map[string]interface{}            // other non-text messages, stored by ID
}

// initialize the database
func (db *MessageDatabase) Init() {
	//var this = *db
	(*db).textMessages = make(map[string][]whatsapp.TextMessage)
	(*db).otherMessages = make(map[string]interface{})
	(*db).latestMessage = make(map[string]uint64)
}

// add a text message to the database, stored by RemoteJid
func (db *MessageDatabase) AddTextMessage(msg whatsapp.TextMessage) bool {
	//var this = *db
	var didNew = false
	var wid = msg.Info.RemoteJid
	if (*db).textMessages[wid] == nil {
		var newArr = []whatsapp.TextMessage{}
		(*db).textMessages[wid] = newArr
		(*db).latestMessage[wid] = msg.Info.Timestamp
		didNew = true
	} else if (*db).latestMessage[wid] < msg.Info.Timestamp {
		(*db).latestMessage[wid] = msg.Info.Timestamp
		didNew = true
	}
	(*db).textMessages[wid] = append((*db).textMessages[wid], msg)
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
		(*db).otherMessages[id] = (*msg)
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

// get a string containing all messages for a chat by chat id
func (db *MessageDatabase) GetMessagesString(wid string) (string, []string) {
	//var this = *db
	var out = ""
	var arr = []string{}
	for _, element := range (*db).textMessages[wid] {
		out += GetTextMessageString(&element)
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

// attempts to download a messages attachments, returns path or error message
func (db *MessageDatabase) DownloadMessage(wid string, open bool) (string, error) {
	if msg, ok := (*db).otherMessages[wid]; ok {
		var fileName string = ""
		switch v := msg.(type) {
		default:
		case whatsapp.ImageMessage:
			if data, err := v.Download(); err == nil {
				fileName = v.Info.Id + "." + strings.TrimPrefix(v.Type, "image/")
				err := saveAttachment(data, fileName, open)
				return fileName, err
			} else {
				return fileName, err
			}
		case whatsapp.DocumentMessage:
			if data, err := v.Download(); err == nil {
				fileName = v.Info.Id + "." + strings.TrimPrefix(strings.TrimPrefix(v.Type, "application/"), "document/")
				err := saveAttachment(data, fileName, open)
				return fileName, err
			} else {
				return fileName, err
			}
		case whatsapp.AudioMessage:
			if data, err := v.Download(); err == nil {
				fileName = v.Info.Id + "." + strings.TrimPrefix(v.Type, "audio/")
				err := saveAttachment(data, fileName, open)
				return fileName, err
			} else {
				return fileName, err
			}
		case whatsapp.VideoMessage:
			if data, err := v.Download(); err == nil {
				fileName = v.Info.Id + "." + strings.TrimPrefix(v.Type, "video/")
				err := saveAttachment(data, fileName, open)
				return fileName, err
			} else {
				return fileName, err
			}
		}
	}
	return "", errors.New("No attachments found")
}

func saveAttachment(data []byte, fileName string, openIt bool) error {
	path := GetHomeDir() + "Downloads" + string(os.PathSeparator) + fileName
	err := ioutil.WriteFile(path, data, 0644)
	if err == nil {
		if openIt {
			open.Run(path)
		}
	} else {
		return err
	}
	return nil
}
