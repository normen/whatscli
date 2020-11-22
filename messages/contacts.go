package messages

import (
	"encoding/gob"
	"os"
	"os/user"
	"strings"

	"github.com/Rhymen/go-whatsapp"
	"github.com/normen/whatscli/config"
)

var contacts map[string]string
var connection *whatsapp.Conn

// loads custom contacts from disk
func LoadContacts() {
	contacts = make(map[string]string)
	file, err := os.Open(config.GetContactsFilePath())
	if err != nil {
		// load old contacts file, re-save in new location if found
		file, err = os.Open(GetHomeDir() + ".whatscli.contacts")
		if err != nil {
			return
		} else {
			os.Remove(GetHomeDir() + ".whatscli.contacts")
			SaveContacts()
		}
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&contacts)
	if err != nil {
		return
	}
}

// saves custom contacts to disk
func SaveContacts() {
	file, err := os.Open(config.GetContactsFilePath())
	if err != nil {
		return
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(contacts)
	if err != nil {
		return
	}
	return
}

// sets a new name for a whatsapp id
func SetIdName(id string, name string) {
	contacts[id] = name
	SaveContacts()
}

// gets a pretty name for a whatsapp id
func GetIdName(id string) string {
	if _, ok := contacts[id]; ok {
		return contacts[id]
	}
	if val, ok := connection.Store.Contacts[id]; ok {
		if val.Name != "" {
			return val.Name
		} else if val.Short != "" {
			return val.Short
		} else if val.Notify != "" {
			return val.Notify
		}
	}
	return strings.TrimSuffix(id, CONTACTSUFFIX)
}

// gets a short name for a whatsapp id
func GetIdShort(id string) string {
	if val, ok := connection.Store.Contacts[id]; ok {
		if val.Short != "" {
			return val.Short
		} else if val.Name != "" {
			return val.Name
		} else if val.Notify != "" {
			return val.Notify
		}
	}
	if _, ok := contacts[id]; ok {
		return contacts[id]
	}
	return strings.TrimSuffix(id, CONTACTSUFFIX)
}

// gets the OS home dir with a path separator at the end
func GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
	}
	return usr.HomeDir + string(os.PathSeparator)
}
