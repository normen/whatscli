package messages

import (
	"encoding/gob"
	"fmt"
	"github.com/rivo/tview"
	"os"
	"time"

	"github.com/Rhymen/go-whatsapp"
	"github.com/normen/whatscli/qrcode"
)

var textView *tview.TextView

// TODO: remove this circular dependeny in favor of a better way
func SetTextView(tv *tview.TextView) {
	textView = tv
}

// gets an existing connection or creates one
func GetConnection() *whatsapp.Conn {
	var wac *whatsapp.Conn
	if connection == nil {
		wacc, err := whatsapp.NewConn(5 * time.Second)
		if err != nil {
			return nil
		}
		wac = wacc
		connection = wac
		//wac.SetClientVersion(2, 2021, 4)
	} else {
		wac = connection
	}
	return wac
}

// Login logs in the user. It ries to see if a session already exists. If not, tries to create a
// new one using qr scanned on the terminal.
func Login() error {
	return LoginWithConnection(GetConnection())
}

// LoginWithConnection logs in the user using a provided connection. It ries to see if a session already exists. If not, tries to create a
// new one using qr scanned on the terminal.
func LoginWithConnection(wac *whatsapp.Conn) error {
	//load saved session
	session, err := readSession()
	if err == nil {
		//restore session
		session, err = wac.RestoreWithSession(session)
		if err != nil {
			return fmt.Errorf("restoring failed: %v\n", err)
		}
	} else {
		//no saved session -> regular login
		qr := make(chan string)
		go func() {
			terminal := qrcode.New()
			terminal.SetOutput(tview.ANSIWriter(textView))
			terminal.Get(<-qr).Print()
		}()
		session, err = wac.Login(qr)
		if err != nil {
			return fmt.Errorf("error during login: %v\n", err)
		}
	}

	//save session
	err = writeSession(session)
	if err != nil {
		return fmt.Errorf("error saving session: %v\n", err)
	}
	//<-time.After(3 * time.Second)
	return nil
}

// Logout logs out the user.
func Logout() error {
	return removeSession()
}

// reads the session file from disk
func readSession() (whatsapp.Session, error) {
	session := whatsapp.Session{}
	file, err := os.Open(GetHomeDir() + ".whatscli.session")
	if err != nil {
		return session, err
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&session)
	if err != nil {
		return session, err
	}
	return session, nil
}

// saves the session file to disk
func writeSession(session whatsapp.Session) error {
	file, err := os.Create(GetHomeDir() + ".whatscli.session")
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(session)
	if err != nil {
		return err
	}
	return nil
}

// deletes the session file from disk
func removeSession() error {
	return os.Remove(GetHomeDir() + ".whatscli.session")
}
