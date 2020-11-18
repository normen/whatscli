package messages

import (
	"encoding/gob"
	"github.com/Rhymen/go-whatsapp"
	"os"
	"os/user"
	"strings"
)

var contacts map[string]string
var connection *whatsapp.Conn

func SetConnection(conn *whatsapp.Conn) {
	connection = conn
}

func LoadContacts() {
	contacts = make(map[string]string)
	file, err := os.Open(GetHomeDir() + ".whatscli.contacts")
	if err != nil {
		return
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&contacts)
	if err != nil {
		return
	}
}

func SaveContacts() {
	file, err := os.Create(GetHomeDir() + ".whatscli.contacts")
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

func SetIdName(id string, name string) {
	contacts[id] = name
	SaveContacts()
}

func GetIdName(id string) string {
	if _, ok := contacts[id]; ok {
		return contacts[id]
	}
	if val, ok := connection.Store.Contacts[id]; ok {
		if val.Short != "" {
			return val.Short
		} else if val.Name != "" {
			return val.Name
		} else if val.Notify != "" {
			return val.Notify
		}
	}
	return strings.TrimSuffix(id, CONTACTSUFFIX)
}

func GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
	}
	return usr.HomeDir + string(os.PathSeparator)
}
