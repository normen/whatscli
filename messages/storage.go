package messages

import (
	"github.com/Rhymen/go-whatsapp"
	"github.com/rivo/tview"
	"sort"
	"strings"
	"time"
)

const GROUPSUFFIX = "@g.us"
const CONTACTSUFFIX = "@s.whatsapp.net"

type MessageDatabase struct {
	textMessages  map[string][]whatsapp.TextMessage
	latestMessage map[string]uint64
}

func (db *MessageDatabase) Init() {
	//var this = *db
	(*db).textMessages = make(map[string][]whatsapp.TextMessage)
	(*db).latestMessage = make(map[string]uint64)
}

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
	//sort.Strings(keys)
	return keys
}

func (db *MessageDatabase) GetMessagesString(wid string) string {
	//var this = *db
	var out = ""
	for _, element := range (*db).textMessages[wid] {
		out += GetTextMessageString(&element)
	}
	return out
}

func GetTextMessageString(msg *whatsapp.TextMessage) string {
	out := ""
	text := tview.Escape((*msg).Text)
	tim := time.Unix(int64((*msg).Info.Timestamp), 0)
	if (*msg).Info.FromMe { //msg from me
		out += "\n[-:-:d](" + tim.Format("01-02-06 15:04:05") + ") [blue:-:b]Me: [-:-:-]" + text
	} else if strings.Contains((*msg).Info.RemoteJid, GROUPSUFFIX) { // group msg
		//(*msg).Info.SenderJid
		userId := (*msg).Info.SenderJid
		//userId := strings.Split(string((*msg).Info.RemoteJid), "-")[0] + CONTACTSUFFIX
		out += "\n[-:-:d](" + tim.Format("01-02-06 15:04:05") + ") [green:-:b]" + GetIdName(userId) + ": [-:-:-]" + text
	} else { // message from others
		out += "\n[-:-:d](" + tim.Format("01-02-06 15:04:05") + ") [green:-:b]" + GetIdName((*msg).Info.RemoteJid) + ": [-:-:-]" + text
	}
	return out
}
