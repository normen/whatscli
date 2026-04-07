package messages

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MessageDatabase stores messages and contact data.
type MessageDatabase struct {
	messages     map[string][]Message
	messagesById map[string]Message
	chats        map[string]Chat
	contacts     map[string]Contact

	contactLock sync.RWMutex
	chatLock    sync.RWMutex
	messageLock sync.RWMutex
}

// Init initializes the message database.
func (md *MessageDatabase) Init() {
	md.messages = make(map[string][]Message)
	md.messagesById = make(map[string]Message)
	md.chats = make(map[string]Chat)
	md.contacts = make(map[string]Contact)
}

// AddMessage stores a message and updates related chat state.
func (md *MessageDatabase) AddMessage(msg Message, markUnread bool) bool {
	md.messageLock.Lock()
	defer md.messageLock.Unlock()

	if existing, ok := md.messagesById[msg.Id]; ok {
		// Keep the first version, but upgrade metadata if the newer message has richer data.
		if existing.RawMessage == nil && msg.RawMessage != nil {
			existing.RawMessage = msg.RawMessage
		}
		if existing.Kind == MessageKindUnknown && msg.Kind != MessageKindUnknown {
			existing.Kind = msg.Kind
		}
		if existing.Text == "" && msg.Text != "" {
			existing.Text = msg.Text
		}
		if existing.FileName == "" && msg.FileName != "" {
			existing.FileName = msg.FileName
		}
		if existing.MimeType == "" && msg.MimeType != "" {
			existing.MimeType = msg.MimeType
		}
		existing.Unread = existing.Unread || markUnread
		md.messagesById[msg.Id] = existing
		md.replaceMessageLocked(existing)
		md.updateChatFromMessageLocked(existing, markUnread)
		return false
	}

	msg.Unread = markUnread
	md.messagesById[msg.Id] = msg
	md.messages[msg.ChatId] = append(md.messages[msg.ChatId], msg)
	sort.Slice(md.messages[msg.ChatId], func(i, j int) bool {
		if md.messages[msg.ChatId][i].Timestamp == md.messages[msg.ChatId][j].Timestamp {
			return md.messages[msg.ChatId][i].Id < md.messages[msg.ChatId][j].Id
		}
		return md.messages[msg.ChatId][i].Timestamp < md.messages[msg.ChatId][j].Timestamp
	})
	md.updateChatFromMessageLocked(msg, markUnread)
	return true
}

func (md *MessageDatabase) replaceMessageLocked(msg Message) {
	msgs := md.messages[msg.ChatId]
	for idx, current := range msgs {
		if current.Id == msg.Id {
			msgs[idx] = msg
			md.messages[msg.ChatId] = msgs
			return
		}
	}
}

func (md *MessageDatabase) updateChatFromMessageLocked(msg Message, markUnread bool) {
	md.chatLock.Lock()
	defer md.chatLock.Unlock()

	chat, exists := md.chats[msg.ChatId]
	if !exists {
		chat = Chat{
			Id:      msg.ChatId,
			IsGroup: strings.Contains(msg.ChatId, GROUPSUFFIX),
			Name:    msg.ContactName,
		}
	}
	if chat.Name == "" {
		chat.Name = msg.ContactName
	}
	if int64(msg.Timestamp) > chat.LastMessage {
		chat.LastMessage = int64(msg.Timestamp)
	}
	if markUnread {
		chat.Unread++
	}
	md.chats[msg.ChatId] = chat

	if msg.ContactId != "" {
		md.contactLock.Lock()
		if _, ok := md.contacts[msg.ContactId]; !ok {
			md.contacts[msg.ContactId] = Contact{
				Id:    msg.ContactId,
				Name:  msg.ContactName,
				Short: msg.ContactShort,
			}
		}
		md.contactLock.Unlock()
	}
}

// AddChat adds or updates a chat in the database.
func (md *MessageDatabase) AddChat(chat Chat) {
	md.chatLock.Lock()
	defer md.chatLock.Unlock()

	existing, ok := md.chats[chat.Id]
	if ok {
		if chat.Name == "" {
			chat.Name = existing.Name
		}
		if chat.LastMessage < existing.LastMessage {
			chat.LastMessage = existing.LastMessage
		}
		if chat.Unread < existing.Unread {
			chat.Unread = existing.Unread
		}
	}
	md.chats[chat.Id] = chat
}

// UpdateChatUnread syncs unread counts from external sources such as history sync.
func (md *MessageDatabase) UpdateChatUnread(chatID string, unread int) {
	md.messageLock.Lock()
	defer md.messageLock.Unlock()

	ids := md.lastIncomingMessageIDsLocked(chatID, unread)
	unreadSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		unreadSet[id] = struct{}{}
	}

	msgs := md.messages[chatID]
	for idx, msg := range msgs {
		_, ok := unreadSet[msg.Id]
		msg.Unread = ok
		msgs[idx] = msg
		if stored, found := md.messagesById[msg.Id]; found {
			stored.Unread = ok
			md.messagesById[msg.Id] = stored
		}
	}
	md.messages[chatID] = msgs

	md.chatLock.Lock()
	if chat, ok := md.chats[chatID]; ok {
		chat.Unread = len(ids)
		md.chats[chatID] = chat
	}
	md.chatLock.Unlock()
}

func (md *MessageDatabase) lastIncomingMessageIDsLocked(chatID string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	msgs := md.messages[chatID]
	ids := make([]string, 0, limit)
	for idx := len(msgs) - 1; idx >= 0 && len(ids) < limit; idx-- {
		if !msgs[idx].FromMe {
			ids = append(ids, msgs[idx].Id)
		}
	}
	return ids
}

// MarkChatRead clears unread state for the given chat and returns the unread messages that were cleared.
func (md *MessageDatabase) MarkChatRead(chatID string) []Message {
	md.messageLock.Lock()
	defer md.messageLock.Unlock()

	msgs := md.messages[chatID]
	cleared := make([]Message, 0)
	for idx, msg := range msgs {
		if msg.Unread {
			cleared = append(cleared, msg)
			msg.Unread = false
			msgs[idx] = msg
			stored := md.messagesById[msg.Id]
			stored.Unread = false
			md.messagesById[msg.Id] = stored
		}
	}
	md.messages[chatID] = msgs

	md.chatLock.Lock()
	if chat, ok := md.chats[chatID]; ok {
		chat.Unread = 0
		md.chats[chatID] = chat
	}
	md.chatLock.Unlock()

	return cleared
}

// MarkMessageRevoked updates a message to show that it was revoked.
func (md *MessageDatabase) MarkMessageRevoked(messageID string) bool {
	md.messageLock.Lock()
	defer md.messageLock.Unlock()

	msg, ok := md.messagesById[messageID]
	if !ok {
		return false
	}
	msg.Text = "[message revoked]"
	msg.RawMessage = nil
	msg.Kind = MessageKindUnknown
	md.messagesById[messageID] = msg
	md.replaceMessageLocked(msg)
	return true
}

// AddContact adds or updates a contact in the database.
func (md *MessageDatabase) AddContact(contact Contact) {
	md.contactLock.Lock()
	defer md.contactLock.Unlock()

	existing, ok := md.contacts[contact.Id]
	if ok {
		if contact.Name == "" {
			contact.Name = existing.Name
		}
		if contact.Short == "" {
			contact.Short = existing.Short
		}
	}
	md.contacts[contact.Id] = contact
}

// GetChatIds returns chats sorted by most recent message first.
func (md *MessageDatabase) GetChatIds() []Chat {
	md.chatLock.RLock()
	defer md.chatLock.RUnlock()

	allChats := make([]Chat, 0, len(md.chats))
	for _, chat := range md.chats {
		allChats = append(allChats, chat)
	}
	sort.Slice(allChats, func(i, j int) bool {
		if allChats[i].LastMessage == allChats[j].LastMessage {
			return allChats[i].Name < allChats[j].Name
		}
		return allChats[i].LastMessage > allChats[j].LastMessage
	})
	return allChats
}

// GetMessages returns all messages for the given chat.
func (md *MessageDatabase) GetMessages(chatID string) []Message {
	md.messageLock.RLock()
	defer md.messageLock.RUnlock()

	msgs := md.messages[chatID]
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out
}

// GetMessage returns a single message by ID.
func (md *MessageDatabase) GetMessage(id string) (Message, bool) {
	md.messageLock.RLock()
	defer md.messageLock.RUnlock()
	msg, ok := md.messagesById[id]
	return msg, ok
}

// GetOldestMessage returns the oldest stored message in a chat.
func (md *MessageDatabase) GetOldestMessage(chatID string) (Message, bool) {
	md.messageLock.RLock()
	defer md.messageLock.RUnlock()
	msgs := md.messages[chatID]
	if len(msgs) == 0 {
		return Message{}, false
	}
	return msgs[0], true
}

// GetMessageInfo returns a human-readable description of a message.
func (md *MessageDatabase) GetMessageInfo(id string) string {
	msg, ok := md.GetMessage(id)
	if !ok {
		return "Message not found"
	}

	name := md.GetIdName(msg.ContactId)
	short := md.GetIdShort(msg.ContactId)
	direction := "←"
	if msg.FromMe {
		direction = "→"
	}

	kind := string(msg.Kind)
	if kind == "" {
		kind = string(MessageKindUnknown)
	}

	info := fmt.Sprintf(
		"ID: %s\nType: %s\nFrom: %s (%s) %s\nTime: %s\nChat: %s",
		msg.Id,
		kind,
		name,
		short,
		direction,
		time.Unix(int64(msg.Timestamp), 0).Format(time.RFC1123),
		msg.ChatId,
	)
	if msg.FileName != "" {
		info += "\nFile: " + msg.FileName
	}
	if msg.MimeType != "" {
		info += "\nMIME: " + msg.MimeType
	}
	if msg.SenderId != "" {
		info += "\nSender: " + msg.SenderId
	}
	return info
}

// GetIdName resolves a contact or chat ID to a display name.
func (md *MessageDatabase) GetIdName(id string) string {
	if id == "" {
		return "Unknown"
	}

	md.contactLock.RLock()
	contact, ok := md.contacts[id]
	md.contactLock.RUnlock()
	if ok {
		if contact.Name != "" {
			return contact.Name
		}
		if contact.Short != "" {
			return contact.Short
		}
	}

	md.chatLock.RLock()
	chat, ok := md.chats[id]
	md.chatLock.RUnlock()
	if ok && chat.Name != "" {
		return chat.Name
	}

	return strings.TrimSuffix(strings.TrimSuffix(id, CONTACTSUFFIX), GROUPSUFFIX)
}

// GetIdShort resolves a contact or chat ID to a short display name.
func (md *MessageDatabase) GetIdShort(id string) string {
	if id == "" {
		return "Unknown"
	}

	md.contactLock.RLock()
	contact, ok := md.contacts[id]
	md.contactLock.RUnlock()
	if ok {
		if contact.Short != "" {
			return contact.Short
		}
		if contact.Name != "" {
			return contact.Name
		}
	}

	md.chatLock.RLock()
	chat, ok := md.chats[id]
	md.chatLock.RUnlock()
	if ok && chat.Name != "" {
		return chat.Name
	}

	return strings.TrimSuffix(strings.TrimSuffix(id, CONTACTSUFFIX), GROUPSUFFIX)
}
