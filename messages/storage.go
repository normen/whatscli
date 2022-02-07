package messages

import (
	"sort"
	"strings"
)

type MessageDatabase struct {
	textMessages  map[string][]*Message   // text messages stored by RemoteJid
	messagesById  map[string]*Message     // text messages stored by message ID
	latestMessage map[string]uint64       // last message from RemoteJid
	otherMessages map[string]*interface{} // other non-text messages, stored by ID
	contacts      map[string]Contact
	chats         map[string]Chat
}

// initialize the database
func NewMessageDatabase() *MessageDatabase {
	db := &MessageDatabase{}
	db.textMessages = make(map[string][]*Message)
	db.messagesById = make(map[string]*Message)
	db.otherMessages = make(map[string]*interface{})
	db.latestMessage = make(map[string]uint64)
	db.contacts = make(map[string]Contact)
	db.chats = make(map[string]Chat)
	return db
}

// add a text message to the database, stored by RemoteJid
func (db *MessageDatabase) Message(msg *Message) bool {
	//var this = *db
	var didNew = false
	var wid = msg.ContactId
	if db.textMessages[wid] == nil {
		var newArr = []*Message{}
		db.textMessages[wid] = newArr
		db.latestMessage[wid] = msg.Timestamp
		didNew = true
	} else if db.latestMessage[wid] < msg.Timestamp {
		db.latestMessage[wid] = msg.Timestamp
		didNew = true
	}
	//do we know this chat? if not add
	if _, ok := db.chats[msg.ChatId]; !ok {
		//don't have this chat!
		isGroup := strings.Contains(msg.ChatId, GROUPSUFFIX)
		db.chats[msg.ChatId] = Chat{
			msg.ChatId,
			isGroup,
			db.GetIdName(msg.ChatId),
			1,
			int64(msg.Timestamp),
		}
	}
	//check if message exists, ignore otherwise
	if _, ok := db.messagesById[msg.Id]; !ok {
		db.messagesById[msg.Id] = msg
		db.textMessages[wid] = append(db.textMessages[wid], msg)
		sort.Slice(db.textMessages[wid], func(i, j int) bool {
			return db.textMessages[wid][i].Timestamp < db.textMessages[wid][j].Timestamp
		})
	}
	return didNew
}

func (db *MessageDatabase) NewUnreadChat(id string) {
	if chat, ok := db.chats[id]; ok {
		chat.Unread++
		db.chats[id] = chat
	}
}

// add audio/video/image/doc message, stored by message id
//func (db *MessageDatabase) AddOtherMessage(msg *interface{}) {
//TODO: needed?
//var id = ""
//switch v := (*msg).(type) {
//default:
//case whatsapp.ImageMessage:
//id = v.Info.Id
//case whatsapp.DocumentMessage:
//id = v.Info.Id
//case whatsapp.AudioMessage:
//id = v.Info.Id
//case whatsapp.VideoMessage:
//id = v.Info.Id
//}
//if id != "" {
//db.otherMessages[id] = msg
//}
//}

func (db *MessageDatabase) AddContact(contact Contact) {
	db.contacts[contact.Id] = contact
}

func (db *MessageDatabase) AddChat(chat Chat) {
	db.chats[chat.Id] = chat
}

// get an array of all chat ids
func (db *MessageDatabase) GetChatIds() []Chat {
	keys := make([]Chat, len(db.chats))
	i := 0
	for _, k := range db.chats {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool {
		return db.latestMessage[keys[i].Id] > db.latestMessage[keys[j].Id]
	})
	return keys
}

// gets a pretty name for a whatsapp id
func (sm *MessageDatabase) GetIdName(id string) string {
	if val, ok := sm.contacts[id]; ok {
		if val.Name != "" {
			return val.Name
		} else if val.Short != "" {
			return val.Short
		}
	}
	return strings.TrimSuffix(strings.TrimSuffix(id, CONTACTSUFFIX), GROUPSUFFIX)
}

// gets a short name for a whatsapp id
func (sm *MessageDatabase) GetIdShort(id string) string {
	if val, ok := sm.contacts[id]; ok {
		//TODO val.notify from whatsapp??
		if val.Short != "" {
			return val.Short
		} else if val.Name != "" {
			return val.Name
		}
	}
	return strings.TrimSuffix(strings.TrimSuffix(id, CONTACTSUFFIX), GROUPSUFFIX)
}

//func (db *MessageDatabase) GetMessageInfo(id string) string {
//if _, ok := db.otherMessages[id]; ok {
//return "[yellow]OtherMessage[-]"
//}
//out := ""
//if msg, ok := db.messagesById[id]; ok {
//out += "[yellow]ID: " + msg.Id + "[-]\n"
//out += "[yellow]Contact ID: " + msg.ContactId + "[-]\n"
//out += "[yellow]SenderJid: " + msg.Info.SenderJid + "[-]\n"
//out += "[yellow]Participant: " + msg.ContextInfo.Participant + "[-]\n"
//out += "[yellow]QuotedMessageID: " + msg.ContextInfo.QuotedMessageID + "[-]\n"
//}
//return out
//}

// get a string containing all messages for a chat by chat id
func (db *MessageDatabase) GetMessages(wid string) []Message {
	var arr = []Message{}
	for _, element := range db.textMessages[wid] {
		arr = append(arr, *element)
	}
	return arr
}
