package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rhymen/go-whatsapp"
	"github.com/gdamore/tcell/v2"
	"github.com/normen/whatscli/messages"
	"github.com/rivo/tview"
)

type waMsg struct {
	Wid  string
	Text string
}

var VERSION string = "v0.6.0"

var sendChannel chan waMsg
var textChannel chan whatsapp.TextMessage
var otherChannel chan interface{}
var contactChannel chan whatsapp.Contact

var sndTxt string = ""
var currentReceiver string = ""
var curRegions []string
var textView *tview.TextView
var treeView *tview.TreeView
var textInput *tview.InputField
var topBar *tview.TextView

//var infoBar *tview.TextView
var msgStore messages.MessageDatabase

var contactRoot *tview.TreeNode
var handler textHandler
var app *tview.Application

func main() {
	msgStore = messages.MessageDatabase{}
	msgStore.Init()
	messages.LoadContacts()
	app = tview.NewApplication()
	gridLayout := tview.NewGrid()
	gridLayout.SetRows(1, 0, 1)
	gridLayout.SetColumns(30, 0, 30)
	gridLayout.SetBorders(true)
	gridLayout.SetBackgroundColor(tcell.ColorBlack)

	topBar = tview.NewTextView()
	topBar.SetDynamicColors(true)
	topBar.SetScrollable(false)
	topBar.SetText("[::b] WhatsCLI " + VERSION + "  [-::d]Type /help for help")

	//infoBar = tview.NewTextView()
	//infoBar.SetDynamicColors(true)
	//infoBar.SetText("🔋: ??%")

	textView = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			app.SetFocus(textInput)
			return event
		}
		if curRegions == nil {
			return event
		}
		if event.Key() == tcell.KeyDown || event.Rune() == 'j' {
			return event
		}
		if event.Key() == tcell.KeyUp || event.Rune() == 'k' {
			return event
		}
		if event.Rune() == 'd' {
			hls := textView.GetHighlights()
			if len(hls) > 0 {
				DownloadMessageId(hls[0], false)
			}
			return nil
		}
		if event.Rune() == 'o' {
			hls := textView.GetHighlights()
			if len(hls) > 0 {
				DownloadMessageId(hls[0], true)
			}
			return nil
		}
		return event
	})

	// TODO: add better way
	messages.SetTextView(textView)
	PrintHelp()

	//textView.SetBorder(true)

	textInput = tview.NewInputField()
	textInput.SetChangedFunc(func(change string) {
		sndTxt = change
	})
	textInput.SetDoneFunc(EnterCommand)
	textInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlE {
			app.SetFocus(treeView)
			return nil
		}
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
	//gridLayout.AddItem(infoBar, 0, 0, 1, 1, 0, 0, false)
	gridLayout.AddItem(MakeTree(), 1, 0, 2, 1, 0, 0, false)
	gridLayout.AddItem(textView, 1, 1, 1, 3, 0, 0, false)
	gridLayout.AddItem(textInput, 2, 1, 1, 3, 0, 0, false)

	app.SetRoot(gridLayout, true)
	app.EnableMouse(true)
	app.SetFocus(textInput)
	go func() {
		if err := StartTextReceiver(); err != nil {
			fmt.Fprintln(textView, "[red]", err, "[-]")
		}
	}()
	app.Run()
}

// prints help to chat view
func PrintHelp() {
	fmt.Fprintln(textView, "[::b]WhatsCLI "+VERSION+"\n\n[-]")
	fmt.Fprintln(textView, "[-::u]Commands:[-::-]")
	fmt.Fprintln(textView, "/name NewName = name selected contact")
	fmt.Fprintln(textView, "/addname 1234567 NewName = add name for number")
	fmt.Fprintln(textView, "/connect = (re)connect in case the connection dropped")
	fmt.Fprintln(textView, "/load = reload contacts")
	fmt.Fprintln(textView, "/quit = exit app")
	fmt.Fprintln(textView, "/help = show this help\n")
	fmt.Fprintln(textView, "[-::u]Keys:[-::-]")
	fmt.Fprintln(textView, "<Tab> = switch input/contacts")
	fmt.Fprintln(textView, "<Up/Dn> = scroll history\n")
	fmt.Fprintln(textView, "[-::-]Highlighted Message:[-::-]")
	fmt.Fprintln(textView, "d = download attachment")
	fmt.Fprintln(textView, "o = open attachment\n")
}

// called when text is entered by the user
func EnterCommand(key tcell.Key) {
	if sndTxt == "" {
		return
	}
	if key == tcell.KeyEsc {
		textInput.SetText("")
		return
	}
	if sndTxt == "/connect" {
		//command
		messages.GetConnection()
		textInput.SetText("")
		return
	}
	if sndTxt == "/load" {
		//command
		LoadContacts()
		textInput.SetText("")
		return
	}
	if sndTxt == "/help" {
		//command
		PrintHelp()
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
			fmt.Fprintln(textView, "Use /addname 1234567 NewName")
			return
		}
		contact := whatsapp.Contact{
			Jid:  parts[1] + messages.CONTACTSUFFIX,
			Name: strings.TrimPrefix(sndTxt, "/addname "+parts[1]+" "),
		}
		contactChannel <- contact
		textInput.SetText("")
		return
	}
	if currentReceiver == "" {
		fmt.Fprintln(textView, "[red]no contact selected[-]")
		return
	}
	if strings.Index(sndTxt, "/name ") == 0 {
		//command
		contact := whatsapp.Contact{
			Jid:  currentReceiver,
			Name: strings.TrimPrefix(sndTxt, "/name "),
		}
		contactChannel <- contact
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

// creates the TreeView for contacts
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
		if event.Key() == tcell.KeyCtrlSpace {
			app.SetFocus(textInput)
			return nil
		}
		return event
	})
	return treeView
}

// loads the contact data from storage to the TreeView
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

// sets the current contact, loads text from storage to TextView
func SetDisplayedContact(wid string) {
	currentReceiver = wid
	textView.Clear()
	textView.SetTitle(messages.GetIdName(wid))
	msgTxt, regIds := msgStore.GetMessagesString(wid)
	textView.SetText(msgTxt)
	curRegions = regIds
}

// starts the receiver and message handling thread
func StartTextReceiver() error {
	var wac = messages.GetConnection()
	err := messages.LoginWithConnection(wac)
	if err != nil {
		return fmt.Errorf("%v\n", err)
	}
	handler = textHandler{}
	wac.AddHandler(handler)
	sendChannel = make(chan waMsg)
	textChannel = make(chan whatsapp.TextMessage)
	otherChannel = make(chan interface{})
	contactChannel = make(chan whatsapp.Contact)
	for {
		select {
		case msg := <-sendChannel:
			SendText(msg.Wid, msg.Text)
		case rcvd := <-textChannel:
			if msgStore.AddTextMessage(rcvd) {
				app.QueueUpdateDraw(LoadContacts)
			}
		case other := <-otherChannel:
			msgStore.AddOtherMessage(&other)
		case contact := <-contactChannel:
			messages.SetIdName(contact.Jid, contact.Name)
			app.QueueUpdateDraw(LoadContacts)
		}
	}
	fmt.Fprintln(textView, "closing the receiver")
	wac.Disconnect()
	return nil
}

// sends text to whatsapp id
func SendText(wid string, text string) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: wid,
			FromMe:    true,
			Timestamp: uint64(time.Now().Unix()),
		},
		Text: text,
	}

	_, err := messages.GetConnection().Send(msg)
	if err != nil {
		fmt.Fprintln(textView, "[red]error sending message: ", err, "[-]")
	} else {
		msgStore.AddTextMessage(msg)
		PrintTextMessage(msg)
	}
}

func DownloadMessageId(id string, open bool) {
	fmt.Fprintln(textView, "[::d]..attempt download of #", id, "[::-]")
	go func() {
		if result, err := msgStore.DownloadMessage(id, open); err == nil {
			fmt.Fprintln(textView, "[::d]Downloaded as [yellow]", result, "[-::-]")
		} else {
			fmt.Fprintln(textView, "[red::d]", err.Error(), "[-::-]")
		}
	}()
}

func NotifyMsg(msg whatsapp.TextMessage) {
	if int64(msg.Info.Timestamp) > time.Now().Unix()-30 {
		//fmt.Print("\a")
		//err := beeep.Notify(messages.GetIdName(msg.Info.RemoteJid), msg.Text, "")
		//if err != nil {
		//  fmt.Fprintln(textView, "[red]error in notification[-]")
		//}
	}
}

// prints a text message to the TextView
func PrintTextMessage(msg whatsapp.TextMessage) {
	fmt.Fprintln(textView, messages.GetTextMessageString(&msg))
}

// handler struct for whatsapp callbacks
type textHandler struct{}

// HandleError implements the error handler interface for go-whatsapp
func (t textHandler) HandleError(err error) {
	// TODO : handle go routine here
	fmt.Fprintln(textView, "[red]error in textHandler : ", err, "[-]")
	return
}

// HandleTextMessage implements the text message handler interface for go-whatsapp
func (t textHandler) HandleTextMessage(msg whatsapp.TextMessage) {
	textChannel <- msg
	if msg.Info.RemoteJid != currentReceiver {
		NotifyMsg(msg)
		return
	}
	PrintTextMessage(msg)
}

// methods to convert messages to TextMessage
func (t textHandler) HandleImageMessage(message whatsapp.ImageMessage) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			SenderJid: message.Info.SenderJid,
			FromMe:    message.Info.FromMe,
			Timestamp: message.Info.Timestamp,
			Id:        message.Info.Id,
		},
		Text: "[IMAGE] " + message.Caption,
	}
	t.HandleTextMessage(msg)
	otherChannel <- message
}

func (t textHandler) HandleDocumentMessage(message whatsapp.DocumentMessage) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			SenderJid: message.Info.SenderJid,
			FromMe:    message.Info.FromMe,
			Timestamp: message.Info.Timestamp,
			Id:        message.Info.Id,
		},
		Text: "[DOCUMENT] " + message.Title,
	}
	t.HandleTextMessage(msg)
	otherChannel <- message
}

func (t textHandler) HandleVideoMessage(message whatsapp.VideoMessage) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			SenderJid: message.Info.SenderJid,
			FromMe:    message.Info.FromMe,
			Timestamp: message.Info.Timestamp,
			Id:        message.Info.Id,
		},
		Text: "[VIDEO] " + message.Caption,
	}
	t.HandleTextMessage(msg)
	otherChannel <- message
}

func (t textHandler) HandleAudioMessage(message whatsapp.AudioMessage) {
	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			SenderJid: message.Info.SenderJid,
			FromMe:    message.Info.FromMe,
			Timestamp: message.Info.Timestamp,
			Id:        message.Info.Id,
		},
		Text: "[AUDIO]",
	}
	t.HandleTextMessage(msg)
	otherChannel <- message
}

// add contact info to database TODO: when are these sent??
func (t textHandler) HandleNewContact(contact whatsapp.Contact) {
	// redundant, wac has contacts
	//contactChannel <- contact
}

//func (t textHandler) HandleBatteryMessage(msg whatsapp.BatteryMessage) {
//  app.QueueUpdate(func() {
//    infoBar.SetText("🔋: " + string(msg.Percentage) + "%")
//  })
//}
