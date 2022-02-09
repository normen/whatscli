package messages

import (
	"database/sql"
	//"fmt"
	"strings"
)

const DB_VERSION = 0

type MessageDatabase struct {
	store *sql.DB
	//TODO: db locks when using contacts in db..
	contacts map[string]Contact
}

//   container, err := sqlstore.New("sqlite3", "file:yoursqlitefile.db?_foreign_keys=on", nil)
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
	//if _, err = db.store.Exec(`CREATE TABLE IF NOT EXISTS contacts(
	//  id TEXT,
	//  name TEXT,
	//  short TEXT
	//)`); err != nil {
	//  return nil, err
	//}
	if _, err = db.store.Exec(`CREATE TABLE IF NOT EXISTS chats(
    id TEXT,
    isgroup INTEGER,
    unread INTEGER,
    lastmessage INTEGER
  )`); err != nil {
		return nil, err
	}
	if _, err = db.store.Exec(`CREATE TABLE IF NOT EXISTS messages(
    id TEXT,
    chatid TEXT,
    contactid TEXT,
    timestamp INTEGER,
    fromme INTEGER,
    forwarded INTEGER,
    text TEXT,
    link TEXT,
    messagetype TEXT,
    medialink TEXT
  )`); err != nil {
		return nil, err
	}
	if err := db.UpdateDatabaseVersion(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *MessageDatabase) UpdateDatabaseVersion() error {
	if rows, err := db.store.Query("SELECT dbversion from version"); err != nil {
		defer rows.Close()
		if rows.Next() {
			var dbVersion int
			rows.Scan(&dbVersion)
			if DB_VERSION > dbVersion {
				//TODO:update db
				if dbVersion < 1 {
					//do stuff etc.
				}
				if dbVersion < 2 {
					//do stuff etc.
				}
				if _, err := db.store.Exec(`DELETE * FROM version`); err != nil {
					return err
				}
				if _, err := db.store.Exec(`INSERT INTO version (dbversion) VALUES($1)`, DB_VERSION); err != nil {
					return err
				}
			}
		} else {
			if _, err := db.store.Exec(`INSERT INTO version (dbversion) VALUES($1)`, DB_VERSION); err != nil {
				return err
			}
		}
	} else {
		defer rows.Close()
		return err
	}
	return nil
}

// add a message to the database
func (db *MessageDatabase) Message(msg *Message) (bool, error) {
	//TODO: didnew/error
	if result, err := db.store.Query("SELECT * FROM messages WHERE id=$1", msg.Id); err == nil {
		defer result.Close()
		if result.Next() {
			return false, err
		}
	} else {
		defer result.Close()
		return false, err
	}
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
      medialink
    )
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		msg.Id,
		msg.ChatId,
		msg.ContactId,
		msg.Timestamp,
		msg.FromMe,
		msg.Forwarded,
		msg.Text,
		msg.Link,
		msg.MessageType,
		msg.MediaLink); err != nil {
		return false, err
	}
	//var didNew = false
	//var wid = msg.ContactId
	//if db.textMessages[wid] == nil {
	//  var newArr = []*Message{}
	//  db.textMessages[wid] = newArr
	//  db.latestMessage[wid] = msg.Timestamp
	//  didNew = true
	//} else if db.latestMessage[wid] < msg.Timestamp {
	//  db.latestMessage[wid] = msg.Timestamp
	//  didNew = true
	//}

	//do we know this chat? if not add
	if result, err := db.store.Query("SELECT * FROM chats WHERE id=$1", msg.ChatId); err == nil {
		defer result.Close()
		if !result.Next() {
			//new
			//TODO: check group else, e.g.
			//isGroup := msg.ChatId != msg.ContactId
			isGroup := strings.Contains(msg.ChatId, GROUPSUFFIX)
			_, err := db.store.Exec(`INSERT INTO chats (
          id,
          isgroup,
          unread,
          lastmessage
        )
        VALUES ($1,$2,$3,$4)`,
				msg.ChatId,
				isGroup,
				1,
				msg.Timestamp,
			)
			if err != nil {
				return false, err
			}
		}
	} else {
		defer result.Close()
		return false, err
	}
	return false, nil
}

func (db *MessageDatabase) AddContact(contact Contact) error {
	db.contacts[contact.Id] = contact
	return nil
}

//func (db *MessageDatabase) AddContact(contact Contact) error {
//  //fmt.Printf("Get: %s ", contact.Id)
//  if result, err := db.store.Query("SELECT * FROM contacts WHERE id=$1", contact.Id); err == nil {
//    defer result.Close()
//    if result.Next() {
//      db.store.Exec(`DELETE FROM contacts WHERE id=$1`, contact.Id)
//    }
//  } else {
//    defer result.Close()
//    return err
//  }
//  if _, err := db.store.Exec(`INSERT INTO contacts (
//        id,
//        name,
//        short
//      )
//      VALUES ($1,$2,$3)`,
//    contact.Id,
//    contact.Name,
//    contact.Short,
//  ); err != nil {
//    return err
//  }
//  return nil
//}

func (db *MessageDatabase) AddChat(chat Chat) error {
	if result, err := db.store.Query("SELECT * FROM chats WHERE id=$1", chat.Id); err == nil {
		defer result.Close()
		if result.Next() {
			db.store.Exec(`DELETE FROM chats WHERE id=$1`, chat.Id)
		}
	} else {
		defer result.Close()
		return err
	}
	db.store.Exec(`INSERT INTO chats (
        id,
        isgroup,
        unread,
        lastmessage
      )
      VALUES ($1,$2,$3,$4)`,
		chat.Id,
		chat.IsGroup,
		chat.Unread,
		chat.LastMessage,
	)
	return nil
}

// get an array of all chat ids
func (db *MessageDatabase) GetChatIds() []Chat {
	var ret []Chat
	if result, err := db.store.Query("SELECT id, isgroup, unread, lastmessage FROM chats ORDER by lastmessage DESC"); err == nil {
		defer result.Close()
		for result.Next() {
			chat := Chat{}
			result.Scan(&chat.Id, &chat.IsGroup, &chat.Unread, &chat.LastMessage)
			chat.Name = db.GetIdName(chat.Id)
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

//// gets a pretty name for a whatsapp id
//func (db *MessageDatabase) GetIdName(id string) string {
//  if result, err := db.store.Query("SELECT name, short FROM contacts WHERE id=$1", id); err == nil {
//    defer result.Close()
//    if result.Next() {
//      var name = ""
//      var short = ""
//      result.Scan(&name, &short)
//      //fmt.Printf("Found %s/%s", name, short)
//      if name != "" {
//        return name
//      }
//      if short != "" {
//        return short
//      }
//    }
//  } else {
//    defer result.Close()
//  }
//  return strings.TrimSuffix(strings.TrimSuffix(id, CONTACTSUFFIX), GROUPSUFFIX)
//}

//// gets a short name for a whatsapp id
//func (db *MessageDatabase) GetIdShort(id string) string {
//  if result, err := db.store.Query("SELECT name, short FROM contacts WHERE id=$1", id); err == nil {
//    defer result.Close()
//    if result.Next() {
//      var name = ""
//      var short = ""
//      result.Scan(&name, &short)
//      if short != "" {
//        return short
//      }
//      if name != "" {
//        return name
//      }
//    }
//  } else {
//    defer result.Close()
//  }
//  return strings.TrimSuffix(strings.TrimSuffix(id, CONTACTSUFFIX), GROUPSUFFIX)
//}

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
      medialink
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
