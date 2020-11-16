package main

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp"
	"github.com/gdamore/tcell/v2"
	"github.com/normen/whatscli/messages"
	"github.com/rivo/tview"
	"strings"
	"time"
)

type textHandler struct{}
type waMsg struct {
	Wid  string
	Text string
}

var sendChannel chan waMsg
var textChannel chan whatsapp.TextMessage

var sndTxt string = ""
var currentReceiver string = ""
var textView *tview.TextView
var treeView *tview.TreeView
var textInput *tview.InputField
var topBar *tview.TextView
var connection *whatsapp.Conn
var msgStore messages.MessageDatabase

var contactRoot *tview.TreeNode
var handler textHandler
var app *tview.Application

//var messages map[string]string

func main() {
	msgStore = messages.MessageDatabase{}
	msgStore.Init()
	messages.LoadContacts()
	app = tview.NewApplication()
	gridLayout := tview.NewGrid()
	gridLayout.SetRows(1, 0, 1)
	gridLayout.SetColumns(30, 0, 30)
	gridLayout.SetBorders(true)

	//list := tview.NewList()
	////list.SetTitle("Contacts")
	////list.AddItem("List Contacts", "get the contacts", 'a', func() {
	////  list.Clear()
	////  var ids = msgStore.GetContactIds()
	////  for _, element := range ids {
	////    //fmt.Fprint(textView, "\n"+element)
	////    var elem = element
	////    list.AddItem(messages.GetIdName(element), "", '-', func() {
	////      currentReceiver = elem
	////      textView.Clear()
	////      textView.SetText(msgStore.GetMessagesString(elem))
	////      fmt.Fprint(textView, "\nNeuer Empfänger: ", elem)
	////    })
	////  }
	////})
	//list.ShowSecondaryText(false)
	//list.AddItem("Load", "Load Contacts", 'l', LoadContacts)
	//list.AddItem("Quit", "Press to exit", 'q', func() {
	//  app.Stop()
	//})

	topBar = tview.NewTextView()
	topBar.SetDynamicColors(true)
	topBar.SetText("[::b] WhatsCLI v0.3.0  [-::d]Help: /name (NewName) | /addname (number) (NewName) | /quit | <Tab> = contacts/message | <Up/Dn> = scroll")

	textView = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	fmt.Fprint(textView, "[::b]WhatsCLI v0.3.0\n\n[-][-::u]Commands:[-::-]\n/name NewName = name current contact\n/addname number NewName = name by number\n/load = reload contacts\n/quit = exit app\n\n[-::u]Keys:[-::-]\n<Tab> = switch input/contacts\n<Up/Dn> = scroll history")

	//textView.SetBorder(true)

	textInput = tview.NewInputField()
	textInput.SetChangedFunc(func(change string) {
		sndTxt = change
	})
	textInput.SetDoneFunc(EnterCommand)
	textInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			app.SetFocus(treeView)
			return nil
		}
		if event.Key() == tcell.KeyDown {
			offset, _ := textView.GetScrollOffset()
			offset += 1
			textView.ScrollTo(offset, 0)
			return nil
		}
		if event.Key() == tcell.KeyUp {
			offset, _ := textView.GetScrollOffset()
			offset -= 1
			textView.ScrollTo(offset, 0)
			return nil
		}
		if event.Key() == tcell.KeyPgDn {
			offset, _ := textView.GetScrollOffset()
			offset += 10
			textView.ScrollTo(offset, 0)
			return nil
		}
		if event.Key() == tcell.KeyPgUp {
			offset, _ := textView.GetScrollOffset()
			offset -= 10
			textView.ScrollTo(offset, 0)
			return nil
		}
		return event
	})

	gridLayout.AddItem(topBar, 0, 0, 1, 4, 0, 0, false)
	gridLayout.AddItem(MakeTree(), 1, 0, 2, 1, 0, 0, false)
	gridLayout.AddItem(textView, 1, 1, 1, 3, 0, 0, false)
	gridLayout.AddItem(textInput, 2, 1, 1, 3, 0, 0, false)

	app.SetRoot(gridLayout, true)
	app.EnableMouse(true)
	app.SetFocus(textInput)
	go func() {
		if err := StartTextReceiver(); err != nil {
			fmt.Fprint(textView, err)
		}
	}()
	app.Run()
}

func EnterCommand(key tcell.Key) {
	if sndTxt == "" {
		return
	}
	if sndTxt == "/load" {
		//command
		LoadContacts()
		textInput.SetText("")
		return
	}
	if sndTxt == "/quit" {
		//command
		app.Stop()
		return
	}
	if strings.Index(sndTxt, "/addname ") == 0 {
		//command
		parts := strings.Split(sndTxt, " ")
		if len(parts) < 3 {
			fmt.Fprint(textView, "\nUse /addname 1234567 NewName")
			return
		}
		messages.SetIdName(parts[1]+messages.CONTACTSUFFIX, strings.TrimPrefix(sndTxt, "/addname "+parts[1]+" "))
		SetDisplayedContact(currentReceiver)
		LoadContacts()
		textInput.SetText("")
		return
	}
	if currentReceiver == "" {
		fmt.Fprint(textView, "\nNo recipient set")
		return
	}
	if strings.Index(sndTxt, "/name ") == 0 {
		//command
		messages.SetIdName(currentReceiver, strings.TrimPrefix(sndTxt, "/name "))
		SetDisplayedContact(currentReceiver)
		LoadContacts()
		textInput.SetText("")
		return
	}
	// send message
	msg := waMsg{
		Wid:  currentReceiver,
		Text: sndTxt,
	}
	sendChannel <- msg
	textInput.SetText("")
}

func MakeTree() *tview.TreeView {
	rootDir := "Contacts"
	contactRoot = tview.NewTreeNode(rootDir).
		SetColor(tcell.ColorYellow)
	treeView = tview.NewTreeView().
		SetRoot(contactRoot).
		SetCurrentNode(contactRoot)

	// If a contact was selected, open it.
	treeView.SetChangedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference == nil {
			return // Selecting the root node does nothing.
		}
		children := node.GetChildren()
		if len(children) == 0 {
			// Load and show files in this directory.
			recv := reference.(string)
			SetDisplayedContact(recv)
		} else {
			// Collapse if visible, expand if collapsed.
			node.SetExpanded(!node.IsExpanded())
		}
	})
	treeView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			app.SetFocus(textInput)
			return nil
		}
		return event
	})
	return treeView
}

func LoadContacts() {
	var ids = msgStore.GetContactIds()
	contactRoot.ClearChildren()
	for _, element := range ids {
		node := tview.NewTreeNode(messages.GetIdName(element)).
			SetReference(element).
			SetSelectable(true)
		if strings.Count(element, messages.CONTACTSUFFIX) > 0 {
			node.SetColor(tcell.ColorGreen)
		} else {
			node.SetColor(tcell.ColorBlue)
		}
		contactRoot.AddChild(node)
		if element == currentReceiver {
			treeView.SetCurrentNode(node)
		}
	}
}

func SetDisplayedContact(wid string) {
	currentReceiver = wid
	textView.Clear()
	textView.SetTitle(messages.GetIdName(wid))
	textView.SetText(msgStore.GetMessagesString(wid))
}

// HandleError implements the handler interface for go-whatsapp
func (t textHandler) HandleError(err error) {
	// TODO : handle go routine here
	fmt.Fprint(textView, "\nerror in textHandler : %v", err)
	return
}

// HandleTextMessage implements the text message handler interface for go-whatsapp
func (t textHandler) HandleTextMessage(msg whatsapp.TextMessage) {
	textChannel <- msg
	if msg.Info.RemoteJid != currentReceiver {
		//fmt.Print("\a")
		return
	}
	PrintTextMessage(msg)
}

func PrintTextMessage(msg whatsapp.TextMessage) {
	fmt.Fprint(textView, messages.GetTextMessageString(&msg))
}

// StartTextReceiver starts the handler for the text messages received
func StartTextReceiver() error {
	var wac = GetConnection()
	err := LoginWithConnection(wac)
	if err != nil {
		return fmt.Errorf("%v\n", err)
	}
	handler = textHandler{}
	wac.AddHandler(handler)
	sendChannel = make(chan waMsg)
	textChannel = make(chan whatsapp.TextMessage)
	for {
		select {
		case msg := <-sendChannel:
			SendText(msg.Wid, msg.Text)
		case rcvd := <-textChannel:
			if msgStore.AddTextMessage(rcvd) {
				app.QueueUpdateDraw(LoadContacts)
			}
		}
	}
	fmt.Fprint(textView, "\n"+"closing the receiver")
	wac.Disconnect()
	return nil
}

func SendText(wid string, text string) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: wid,
			FromMe:    true,
			Timestamp: uint64(time.Now().Unix()),
		},
		Text: text,
	}

	PrintTextMessage(msg)
	//TODO: workaround for error when receiving&sending
	connection.RemoveHandlers()
	_, err := connection.Send(msg)
	msgStore.AddTextMessage(msg)
	connection.AddHandler(handler)
	if err != nil {
		fmt.Fprint(textView, "\nerror sending message: %v", err)
	} else {
		//fmt.Fprint(textView, "\nSent msg with ID: %v", msgID)
	}
}
