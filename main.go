package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/Rhymen/go-whatsapp"
	"github.com/gdamore/tcell/v2"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
	"github.com/rivo/tview"
	"github.com/skratchdot/open-golang/open"
	"gitlab.com/tslocum/cbind"
)

var VERSION string = "v0.8.1"

var sndTxt string = ""
var currentReceiver string = ""
var curRegions []string
var textView *tview.TextView
var treeView *tview.TreeView
var textInput *tview.InputField
var topBar *tview.TextView

//var infoBar *tview.TextView
var sessionManager *messages.SessionManager
var keyBindings *cbind.Configuration

var contactRoot *tview.TreeNode
var app *tview.Application
var uiHandler messages.UiMessageHandler

func main() {
	config.InitConfig()
	uiHandler = UiHandler{}
	sessionManager = &messages.SessionManager{}
	sessionManager.Init(uiHandler)
	messages.LoadContacts()

	app = tview.NewApplication()

	sideBarWidth := config.GetIntSetting("ui", "contact_sidebar_width")
	gridLayout := tview.NewGrid()
	gridLayout.SetRows(1, 0, 1)
	gridLayout.SetColumns(sideBarWidth, 0, sideBarWidth)
	gridLayout.SetBorders(true)
	gridLayout.SetBackgroundColor(config.GetColor("background"))
	gridLayout.SetBordersColor(config.GetColor("borders"))

	topBar = tview.NewTextView()
	topBar.SetDynamicColors(true)
	topBar.SetScrollable(false)
	topBar.SetText("[::b] WhatsCLI " + VERSION + "  [-::d]Type /help for help")
	topBar.SetBackgroundColor(config.GetColor("background"))

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
	textView.SetBackgroundColor(config.GetColor("background"))
	textView.SetTextColor(config.GetColor("text"))

	PrintHelp()

	textInput = tview.NewInputField()
	textInput.SetBackgroundColor(config.GetColor("background"))
	textInput.SetFieldBackgroundColor(config.GetColor("input_background"))
	textInput.SetFieldTextColor(config.GetColor("input_text"))
	textInput.SetChangedFunc(func(change string) {
		sndTxt = change
	})
	textInput.SetDoneFunc(EnterCommand)
	textInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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
		if err := sessionManager.StartTextReceiver(); err != nil {
			PrintError(err)
		}
	}()
	LoadShortcuts()
	app.Run()
}

// creates the TreeView for contacts
func MakeTree() *tview.TreeView {
	rootDir := "Contacts"
	contactRoot = tview.NewTreeNode(rootDir).
		SetColor(config.GetColor("list_header"))
	treeView = tview.NewTreeView().
		SetRoot(contactRoot).
		SetCurrentNode(contactRoot)
	treeView.SetBackgroundColor(config.GetColor("background"))

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
	return treeView
}

func handleFocusMessage(ev *tcell.EventKey) *tcell.EventKey {
	if !textView.HasFocus() {
		app.SetFocus(textView)
		if curRegions != nil && len(curRegions) > 0 {
			textView.Highlight(curRegions[len(curRegions)-1])
		}
	}
	return nil
}

func handleFocusInput(ev *tcell.EventKey) *tcell.EventKey {
	ResetMsgSelection()
	if !textInput.HasFocus() {
		app.SetFocus(textInput)
	}
	return nil
}

func handleFocusContacts(ev *tcell.EventKey) *tcell.EventKey {
	ResetMsgSelection()
	if !treeView.HasFocus() {
		app.SetFocus(treeView)
	}
	return nil
}

func handleSwitchPanels(ev *tcell.EventKey) *tcell.EventKey {
	ResetMsgSelection()
	if !textInput.HasFocus() {
		app.SetFocus(textInput)
	} else {
		app.SetFocus(treeView)
	}
	return nil
}

func handleCommand(command string) func(ev *tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		sessionManager.CommandChannel <- messages.Command{command, nil}
		return nil
	}
}

func handleQuit(ev *tcell.EventKey) *tcell.EventKey {
	sessionManager.CommandChannel <- messages.Command{"disconnect", nil}
	app.Stop()
	return nil
}

func handleHelp(ev *tcell.EventKey) *tcell.EventKey {
	PrintHelp()
	return nil
}

func handleDownload(ev *tcell.EventKey) *tcell.EventKey {
	hls := textView.GetHighlights()
	if len(hls) > 0 {
		go DownloadMessageId(hls[0], false)
		ResetMsgSelection()
		app.SetFocus(textInput)
	}
	return nil
}

func handleOpen(ev *tcell.EventKey) *tcell.EventKey {
	hls := textView.GetHighlights()
	if len(hls) > 0 {
		go DownloadMessageId(hls[0], true)
		ResetMsgSelection()
		app.SetFocus(textInput)
	}
	return nil
}

func handleShow(ev *tcell.EventKey) *tcell.EventKey {
	hls := textView.GetHighlights()
	if len(hls) > 0 {
		go PrintImage(hls[0])
		ResetMsgSelection()
		app.SetFocus(textInput)
	}
	return nil
}

func handleInfo(ev *tcell.EventKey) *tcell.EventKey {
	hls := textView.GetHighlights()
	if len(hls) > 0 {
		//TODO: command msg info
		//PrintText(msgStore.GetMessageInfo(hls[0]))
		ResetMsgSelection()
		app.SetFocus(textInput)
	}
	return nil
}

func handleMessagesUp(ev *tcell.EventKey) *tcell.EventKey {
	if curRegions == nil || len(curRegions) == 0 {
		return nil
	}
	hls := textView.GetHighlights()
	if len(hls) > 0 {
		newId := GetOffsetMsgId(hls[0], -1)
		if newId != "" {
			textView.Highlight(newId)
		}
	} else {
		textView.Highlight(curRegions[len(curRegions)-1])
	}
	textView.ScrollToHighlight()
	return nil
}

func handleMessagesDown(ev *tcell.EventKey) *tcell.EventKey {
	if curRegions == nil || len(curRegions) == 0 {
		return nil
	}
	hls := textView.GetHighlights()
	if len(hls) > 0 {
		newId := GetOffsetMsgId(hls[0], 1)
		if newId != "" {
			textView.Highlight(newId)
		}
	} else {
		textView.Highlight(curRegions[0])
	}
	textView.ScrollToHighlight()
	return nil
}

func handleMessagesLast(ev *tcell.EventKey) *tcell.EventKey {
	if curRegions == nil || len(curRegions) == 0 {
		return nil
	}
	textView.Highlight(curRegions[len(curRegions)-1])
	textView.ScrollToHighlight()
	return nil
}

func handleMessagesFirst(ev *tcell.EventKey) *tcell.EventKey {
	if curRegions == nil || len(curRegions) == 0 {
		return nil
	}
	textView.Highlight(curRegions[0])
	textView.ScrollToHighlight()
	return nil
}

func handleExitMessages(ev *tcell.EventKey) *tcell.EventKey {
	if curRegions == nil || len(curRegions) == 0 {
		return nil
	}
	ResetMsgSelection()
	app.SetFocus(textInput)
	return nil
}

func LoadShortcuts() {
	keyBindings = cbind.NewConfiguration()
	if err := keyBindings.Set(config.GetKey("focus_messages"), handleFocusMessage); err != nil {
		PrintErrorMsg("focus_messages:", err)
	}
	if err := keyBindings.Set(config.GetKey("focus_input"), handleFocusInput); err != nil {
		PrintErrorMsg("focus_input:", err)
	}
	if err := keyBindings.Set(config.GetKey("focus_contacts"), handleFocusContacts); err != nil {
		PrintErrorMsg("focus_contacts:", err)
	}
	if err := keyBindings.Set(config.GetKey("switch_panels"), handleSwitchPanels); err != nil {
		PrintErrorMsg("switch_panels:", err)
	}
	if err := keyBindings.Set(config.GetKey("command_backlog"), handleCommand("backlog")); err != nil {
		PrintErrorMsg("command_backlog:", err)
	}
	if err := keyBindings.Set(config.GetKey("command_connect"), handleCommand("login")); err != nil {
		PrintErrorMsg("command_connect:", err)
	}
	if err := keyBindings.Set(config.GetKey("command_quit"), handleQuit); err != nil {
		PrintErrorMsg("command_quit:", err)
	}
	if err := keyBindings.Set(config.GetKey("command_help"), handleHelp); err != nil {
		PrintErrorMsg("command_help:", err)
	}
	app.SetInputCapture(keyBindings.Capture)
	keysMessages := cbind.NewConfiguration()
	if err := keysMessages.Set(config.GetKey("message_download"), handleDownload); err != nil {
		PrintErrorMsg("message_download:", err)
	}
	if err := keysMessages.Set(config.GetKey("message_open"), handleOpen); err != nil {
		PrintErrorMsg("message_open:", err)
	}
	if err := keysMessages.Set(config.GetKey("message_show"), handleShow); err != nil {
		PrintErrorMsg("message_show:", err)
	}
	if err := keysMessages.Set(config.GetKey("message_info"), handleInfo); err != nil {
		PrintErrorMsg("message_info:", err)
	}
	keysMessages.SetKey(tcell.ModNone, tcell.KeyEscape, handleExitMessages)
	keysMessages.SetKey(tcell.ModNone, tcell.KeyUp, handleMessagesUp)
	keysMessages.SetKey(tcell.ModNone, tcell.KeyDown, handleMessagesDown)
	keysMessages.SetRune(tcell.ModNone, 'k', handleMessagesUp)
	keysMessages.SetRune(tcell.ModNone, 'j', handleMessagesDown)
	keysMessages.SetRune(tcell.ModNone, 'g', handleMessagesFirst)
	keysMessages.SetRune(tcell.ModNone, 'G', handleMessagesLast)
	textView.SetInputCapture(keysMessages.Capture)
}

// prints help to chat view
func PrintHelp() {
	fmt.Fprintln(textView, "[::b]WhatsCLI "+VERSION+"[-]")
	fmt.Fprintln(textView, "")
	fmt.Fprintln(textView, "[-::u]Keys:[-::-]")
	fmt.Fprintln(textView, "[::b] Up/Down[::-] = scroll history/contacts")
	fmt.Fprintln(textView, "[::b]", config.GetKey("switch_panels"), "[::-] = switch input/contacts")
	fmt.Fprintln(textView, "[::b]", config.GetKey("focus_messages"), "[::-] = focus message panel")
	fmt.Fprintln(textView, "[::b]", config.GetKey("focus_contacts"), "[::-] = focus contacts panel")
	fmt.Fprintln(textView, "[::b]", config.GetKey("focus_input"), "[::-] = focus input")
	fmt.Fprintln(textView, "")
	fmt.Fprintln(textView, "[-::-]Message panel focused:[-::-]")
	fmt.Fprintln(textView, "[::b] Up/Down[::-] = select message")
	fmt.Fprintln(textView, "[::b]", config.GetKey("message_download"), "[::-] = download attachment -> ", config.GetSetting("download_path"))
	fmt.Fprintln(textView, "[::b]", config.GetKey("message_open"), "[::-] = download & open attachment -> ", config.GetSetting("preview_path"))
	fmt.Fprintln(textView, "[::b]", config.GetKey("message_show"), "[::-] = download & show image using jp2a -> ", config.GetSetting("preview_path"))
	fmt.Fprintln(textView, "[::b]", config.GetKey("message_info"), "[::-] = info about message")
	fmt.Fprintln(textView, "")
	fmt.Fprintln(textView, "[-::u]Commands:[-::-]")
	fmt.Fprintln(textView, "[::b] /backlog[::-] = load more messages for this chat ->[::b]", config.GetKey("command_backlog"), "[::-]")
	fmt.Fprintln(textView, "[::b] /connect[::-] = (re)connect in case the connection dropped ->[::b]", config.GetKey("command_connect"), "[::-]")
	fmt.Fprintln(textView, "[::b] /help[::-] = show this help ->[::b]", config.GetKey("command_help"), "[::-]")
	fmt.Fprintln(textView, "[::b] /quit[::-] = exit app ->[::b]", config.GetKey("command_quit"), "[::-]")
	fmt.Fprintln(textView, "[::b] /disconnect[::-] = close the connection")
	fmt.Fprintln(textView, "[::b] /logout[::-] = remove login data from computer (stays connected until app closes)")
	fmt.Fprintln(textView, "")
	fmt.Fprintln(textView, "Config file in \n-> ", config.GetConfigFilePath())
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
	switch sndTxt {
	}
	if sndTxt == "/backlog" {
		sessionManager.CommandChannel <- messages.Command{"backlog", nil}
		textInput.SetText("")
		return
	}
	if sndTxt == "/connect" {
		sessionManager.CommandChannel <- messages.Command{"login", nil}
		textInput.SetText("")
		return
	}
	if sndTxt == "/disconnect" {
		sessionManager.CommandChannel <- messages.Command{"disconnect", nil}
		textInput.SetText("")
		return
	}
	if sndTxt == "/logout" {
		sessionManager.CommandChannel <- messages.Command{"logout", nil}
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
		sessionManager.Disconnect()
		app.Stop()
		return
	}
	// send message
	msg := messages.Command{
		Name:   "send_message",
		Params: []string{currentReceiver, sndTxt},
	}
	sessionManager.CommandChannel <- msg
	textInput.SetText("")
}

// get the next message id to select (highlighted + offset)
func GetOffsetMsgId(curId string, offset int) string {
	if curRegions == nil || len(curRegions) == 0 {
		return ""
	}
	for idx, val := range curRegions {
		if val == curId {
			arrPos := idx + offset
			if len(curRegions) > arrPos && arrPos >= 0 {
				return curRegions[arrPos]
			}
		}
	}
	if offset > 0 {
		return curRegions[0]
	} else {
		return curRegions[len(curRegions)-1]
	}
}

// resets the selection in the textView and scrolls it down
func ResetMsgSelection() {
	if len(textView.GetHighlights()) > 0 {
		textView.Highlight("")
	}
	textView.ScrollToEnd()
}

// prints text to the TextView
func PrintText(txt string) {
	fmt.Fprintln(textView, txt)
}

// prints an error to the TextView
func PrintError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(textView, "[red]", err.Error(), "[-]")
}

// prints an error to the TextView
func PrintErrorMsg(text string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(textView, "[red]", text, err.Error(), "[-]")
}

// prints an image attachment to the TextView (by message id)
func PrintImage(id string) {
	var err error
	var path string
	PrintText("[::d]loading..[::-]")
	if path, err = sessionManager.DownloadMessage(id, true); err == nil {
		cmd := exec.Command("jp2a", "--color", path)
		var stdout io.ReadCloser
		if stdout, err = cmd.StdoutPipe(); err == nil {
			if err = cmd.Start(); err == nil {
				reader := bufio.NewReader(stdout)
				io.Copy(tview.ANSIWriter(textView), reader)
				return
			}
		}
	}
	PrintError(err)
}

// downloads a specific message attachment
func DownloadMessageId(id string, openIt bool) {
	PrintText("[::d]loading..[::-]")
	if result, err := sessionManager.DownloadMessage(id, openIt); err == nil {
		PrintText("[::d]Downloaded as [yellow]" + result + "[-::-]")
		if openIt {
			open.Run(result)
		}
	} else {
		PrintError(err)
	}
}

// notifies about a new message if its recent
func NotifyMsg(msg whatsapp.TextMessage) {
	if int64(msg.Info.Timestamp) > time.Now().Unix()-30 {
		//fmt.Print("\a")
		//err := beeep.Notify(messages.GetIdName(msg.Info.RemoteJid), msg.Text, "")
		//if err != nil {
		//  fmt.Fprintln(textView, "[red]error in notification[-]")
		//}
	}
}

// sets the current contact, loads text from storage to TextView
func SetDisplayedContact(wid string) {
	currentReceiver = wid
	textView.Clear()
	textView.SetTitle(messages.GetIdName(wid))
	sessionManager.CommandChannel <- messages.Command{"select_contact", []string{currentReceiver}}
}

type UiHandler struct{}

func (u UiHandler) NewMessage(msg string, id string) {
	//TODO: its stupid to "go" this as its supposed to run
	//on the ui thread anyway. But QueueUpdate blocks...?
	go app.QueueUpdateDraw(func() {
		curRegions = append(curRegions, id)
		PrintText(msg)
	})
}

func (u UiHandler) NewScreen(screen string, ids []string) {
	go app.QueueUpdateDraw(func() {
		textView.Clear()
		textView.SetText(screen)
		curRegions = ids
	})
}

// loads the contact data from storage to the TreeView
func (u UiHandler) SetContacts(ids []string) {
	go app.QueueUpdateDraw(func() {
		contactRoot.ClearChildren()
		for _, element := range ids {
			node := tview.NewTreeNode(messages.GetIdName(element)).
				SetReference(element).
				SetSelectable(true)
			if strings.Count(element, messages.CONTACTSUFFIX) > 0 {
				node.SetColor(config.GetColor("list_contact"))
			} else {
				node.SetColor(config.GetColor("list_group"))
			}
			contactRoot.AddChild(node)
			if element == currentReceiver {
				treeView.SetCurrentNode(node)
			}
		}
	})
}

func (u UiHandler) PrintError(err error) {
	PrintError(err)
}

func (u UiHandler) PrintText(msg string) {
	PrintText(msg)
}

func (u UiHandler) GetWriter() io.Writer {
	return textView
}
