package messages

import (
	"database/sql"
	"strings"
)

const DB_VERSION = 1

type MessageDatabase struct {
	store *sql.DB
	//TODO: db locks when using contacts in db..
	contacts map[string]Contact
}

// initialize the database
func NewMessageDatabase() (*MessageDatabase, error) {
	db := &MessageDatabase{}
	db.contacts = make(map[string]Contact)
	var err error
	if db.store, err = sql.Open("sqlite3", "file:messages.db"); err != nil {
		return nil, err
	}
	db.store.SetMaxOpenConns(1)
	if _, err = db.store.Exec(`CREATE TABLE IF NOT EXISTS version(
    dbversion INTEGER
  )`); err != nil {
		return nil, err
	}
	if _, err = db.store.Exec(`CREATE TABLE IF NOT EXISTS chats(
    id TEXT PRIMARY KEY,
    isgroup INTEGER,
    name TEXT,
    unread INTEGER,
    lastmessage INTEGER
  )`); err != nil {
		return nil, err
	}
	if _, err = db.store.Exec(`CREATE TABLE IF NOT EXISTS messages(
    id TEXT PRIMARY KEY,
    chatid TEXT,
    contactid TEXT,
    timestamp INTEGER,
    fromme INTEGER,
    forwarded INTEGER,
    text TEXT,
    link TEXT,
    messagetype TEXT,
    medialink TEXT,
    mediadata1 BLOB,
    mediadata2 BLOB,
    mediadata3 BLOB
  )`); err != nil {
		return nil, err
	}
	if err := db.UpdateDatabaseVersion(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *MessageDatabase) UpdateDatabaseVersion() error {
	if rows, err := db.store.Query("SELECT dbversion from version"); err == nil {
		defer rows.Close()
		if rows.Next() {
			var dbVersion int
			rows.Scan(&dbVersion)
			rows.Close()
			if DB_VERSION > dbVersion {
				if _, err := db.store.Exec(`DELETE FROM version`); err != nil {
					return err
				}
				if dbVersion < 1 {
					if _, err := db.store.Exec(`ALTER TABLE messages ADD mediadata1 BLOB`); err != nil {
						return err
					}
					if _, err := db.store.Exec(`ALTER TABLE messages ADD mediadata2 BLOB`); err != nil {
						return err
					}
					if _, err := db.store.Exec(`ALTER TABLE messages ADD mediadata3 BLOB`); err != nil {
						return err
					}
				}
				if dbVersion < 2 {
					//do stuff etc.
				}
			} else {
				return nil
			}
		}
	} else {
		defer rows.Close()
		return err
	}
	if _, err := db.store.Exec(`INSERT INTO version (dbversion) VALUES($1)`, DB_VERSION); err != nil {
		return err
	}
	return nil
}

// add a message to the database
func (db *MessageDatabase) Message(msg *Message) (bool, error) {
	//TODO: didnew/error
	if _, err := db.store.Exec(`INSERT INTO messages(
      id,
      chatid,
      contactid,
      timestamp,
      fromme,
      forwarded,
      text,
      link,
      messagetype,
      medialink,
      mediadata1,
      mediadata2,
      mediadata3
    )
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO NOTHING
    `,
		msg.Id,
		msg.ChatId,
		msg.ContactId,
		msg.Timestamp,
		msg.FromMe,
		msg.Forwarded,
		msg.Text,
		msg.Link,
		msg.MessageType,
		msg.MediaLink,
		msg.MediaData1,
		msg.MediaData2,
		msg.MediaData3); err != nil {
		return false, err
	}

	return false, nil
}

func (db *MessageDatabase) AddContact(contact Contact) error {
	db.contacts[contact.Id] = contact
	return nil
}

func (db *MessageDatabase) AddChat(chat Chat) error {
	db.store.Exec(`INSERT INTO chats (
        id,
        isgroup,
        name,
        unread,
        lastmessage
      )
      VALUES ($1,$2,$3,$4,$5)
      ON CONFLICT (id) DO UPDATE SET name=$3, unread=$4, lastmessage=$5
      WHERE lastmessage<=$5
      `,
		chat.Id,
		chat.IsGroup,
		chat.Name,
		chat.Unread,
		chat.LastMessage,
	)
	return nil
}

// get an array of all chat ids
func (db *MessageDatabase) GetChatIds() []Chat {
	var ret []Chat
	if result, err := db.store.Query("SELECT id, isgroup, name, unread, lastmessage FROM chats ORDER by lastmessage DESC"); err == nil {
		defer result.Close()
		for result.Next() {
			chat := Chat{}
			result.Scan(&chat.Id, &chat.IsGroup, &chat.Name, &chat.Unread, &chat.LastMessage)
			if chat.Name == "" {
				chat.Name = db.GetIdShort(chat.Id)
			}
			ret = append(ret, chat)
		}
	} else {
		defer result.Close()
	}
	//TODO:sort, lastmsg
	return ret
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

// get a string containing all messages for a chat by chat id
func (db *MessageDatabase) GetMessages(wid string) []Message {
	var ret []Message
	if result, err := db.store.Query(`SELECT 
      id,
      chatid,
      contactid,
      timestamp,
      fromme,
      forwarded,
      text,
      link,
      messagetype,
      medialink,
      mediadata1,
      mediadata2,
      mediadata3
    FROM messages where chatid=$1 ORDER BY timestamp`, wid); err == nil {
		defer result.Close()
		for result.Next() {
			message := Message{}
			result.Scan(
				&message.Id,
				&message.ChatId,
				&message.ContactId,
				&message.Timestamp,
				&message.FromMe,
				&message.Forwarded,
				&message.Text,
				&message.Link,
				&message.MessageType,
				&message.MediaLink,
				&message.MediaData1,
				&message.MediaData2,
				&message.MediaData3,
			)
			message.ContactName = db.GetIdName(message.ContactId)
			message.ContactShort = db.GetIdShort(message.ContactId)
			ret = append(ret, message)
		}
	} else {
		defer result.Close()
	}
	//TODO:sort, lastmsg
	return ret
}
