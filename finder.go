package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
	"github.com/rivo/tview"
)

// search scopes for the Telescope-like finder
const (
	scopeAll = iota
	scopeContacts
	scopeGroups
)

var scopeNames = []string{"Todos", "Contatos", "Grupos"}

// allChats holds the latest full chat list (contacts + groups), kept in sync by
// SetChats so the finder can search across everything without touching the tree.
var allChats []messages.Chat

var (
	finderInput     *tview.InputField
	finderList      *tview.List
	finderVisible   bool
	finderScope     int
	finderResults   []messages.Chat // parallel to the list items currently shown
	finderPrevFocus tview.Primitive
)

// buildFinder constructs the finder overlay (search field + results list) and
// registers it as a hidden page over the main layout.
func buildFinder() {
	bg := tcell.ColorNames[config.Config.Colors.Background]
	hi := tcell.ColorNames[config.Config.Colors.ListHeader]

	finderInput = tview.NewInputField()
	finderInput.SetBackgroundColor(bg)
	finderInput.SetFieldBackgroundColor(tcell.ColorNames[config.Config.Colors.InputBackground])
	finderInput.SetFieldTextColor(tcell.ColorNames[config.Config.Colors.InputText])
	finderInput.SetPlaceholder("digite nome, número ou id…")
	finderInput.SetPlaceholderTextColor(tcell.ColorNames[config.Config.Colors.Borders])
	finderInput.SetBorder(true)
	finderInput.SetTitleAlign(tview.AlignLeft)
	finderInput.SetBorderColor(hi)
	finderInput.SetTitleColor(hi)
	finderInput.SetChangedFunc(func(string) { refreshFinder() })
	finderInput.SetInputCapture(finderKeyCapture)

	finderList = tview.NewList()
	finderList.ShowSecondaryText(true)
	finderList.SetBackgroundColor(bg)
	finderList.SetMainTextColor(tcell.ColorNames[config.Config.Colors.Text])
	finderList.SetSecondaryTextColor(tcell.ColorNames[config.Config.Colors.Borders])
	finderList.SetSelectedTextColor(bg)
	finderList.SetSelectedBackgroundColor(hi)
	finderList.SetBorder(true)
	finderList.SetTitle(" Resultados ")
	finderList.SetTitleAlign(tview.AlignLeft)
	finderList.SetBorderColor(hi)
	finderList.SetTitleColor(hi)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(finderInput, 3, 0, true).
		AddItem(finderList, 0, 1, false)

	pages.AddPage("find", centeredFinder(flex), true, false)
}

// centeredFinder floats the finder roughly centered over the main layout.
func centeredFinder(p tview.Primitive) tview.Primitive {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 2, 0, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(p, 0, 4, true).
			AddItem(nil, 0, 1, false), 0, 3, true).
		AddItem(nil, 2, 0, false)
}

// finderKeyCapture routes navigation keys from the (focused) search field to the
// results list, since the input field keeps focus so the user can keep typing.
func finderKeyCapture(ev *tcell.EventKey) *tcell.EventKey {
	switch ev.Key() {
	case tcell.KeyEscape:
		closeFinder()
		return nil
	case tcell.KeyEnter:
		openFinderSelection()
		return nil
	case tcell.KeyUp, tcell.KeyCtrlP:
		moveFinderSelection(-1)
		return nil
	case tcell.KeyDown, tcell.KeyCtrlN:
		moveFinderSelection(1)
		return nil
	case tcell.KeyCtrlY:
		copyFinderSelection()
		return nil
	case tcell.KeyTab:
		finderScope = (finderScope + 1) % len(scopeNames)
		refreshFinder()
		return nil
	case tcell.KeyBacktab:
		finderScope = (finderScope - 1 + len(scopeNames)) % len(scopeNames)
		refreshFinder()
		return nil
	}
	return ev
}

// openFinder shows the finder overlay and focuses the search field.
func openFinder() {
	if finderVisible {
		return
	}
	finderVisible = true
	finderScope = scopeAll
	finderInput.SetText("") // also triggers refreshFinder via the changed func
	refreshFinder()
	finderPrevFocus = app.GetFocus()
	pages.ShowPage("find")
	app.SetFocus(finderInput)
}

// closeFinder hides the finder overlay and restores the previous focus.
func closeFinder() {
	if !finderVisible {
		return
	}
	finderVisible = false
	pages.HidePage("find")
	if finderPrevFocus != nil {
		app.SetFocus(finderPrevFocus)
	}
}

// copyFinderSelection copies the highlighted chat's full JID to the clipboard,
// giving feedback in the search field title (the messages panel is hidden behind
// the overlay). The full JID is what goes into the bot's chat_id config.
func copyFinderSelection() {
	idx := finderList.GetCurrentItem()
	if idx < 0 || idx >= len(finderResults) {
		return
	}
	id := finderResults[idx].Id
	if err := safeWriteClipboard(id); err != nil {
		finderInput.SetTitle(" 🔭 clipboard indisponível — id: " + id + " ")
	} else {
		finderInput.SetTitle(" 🔭 ✓ copiado: " + id + " ")
	}
}

func moveFinderSelection(delta int) {
	n := finderList.GetItemCount()
	if n == 0 {
		return
	}
	cur := (finderList.GetCurrentItem() + delta + n) % n
	finderList.SetCurrentItem(cur)
}

// openFinderSelection opens the highlighted chat and closes the finder.
func openFinderSelection() {
	idx := finderList.GetCurrentItem()
	if idx < 0 || idx >= len(finderResults) {
		closeFinder()
		return
	}
	chat := finderResults[idx]
	closeFinder()
	SetDisplayedChat(chat)
	selectTreeNodeById(chat.Id)
	app.SetFocus(textInput)
}

// refreshFinder filters allChats by the current query + scope and repopulates the list.
func refreshFinder() {
	if finderInput == nil || finderList == nil {
		return
	}
	query := finderInput.GetText()

	type scored struct {
		chat  messages.Chat
		score int
	}
	matches := make([]scored, 0, len(allChats))
	for _, c := range allChats {
		switch finderScope {
		case scopeContacts:
			if c.IsGroup {
				continue
			}
		case scopeGroups:
			if !c.IsGroup {
				continue
			}
		}
		raw := rawJid(c.Id)
		// search across display name and the raw number/id
		score, ok := fuzzyMatch(query, c.Name+" "+raw)
		if !ok {
			continue
		}
		matches = append(matches, scored{c, score})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return strings.ToLower(matches[i].chat.Name) < strings.ToLower(matches[j].chat.Name)
	})

	finderList.Clear()
	finderResults = finderResults[:0]
	for _, m := range matches {
		c := m.chat
		raw := rawJid(c.Id)
		icon := "👤 "
		if c.IsGroup {
			icon = "👥 "
		}
		label := c.Name
		if label == "" {
			label = "+" + raw
		}
		if c.Unread > 0 {
			label += fmt.Sprintf("  (%d)", c.Unread)
		}
		finderList.AddItem(icon+label, raw, 0, nil)
		finderResults = append(finderResults, c)
	}

	finderInput.SetTitle(fmt.Sprintf(
		" 🔭 %s (Tab) · ↑/↓ · Enter abre · Ctrl+y copia id · Esc — %d resultado(s) ",
		scopeNames[finderScope], len(finderResults),
	))
}

// rawJid strips the WhatsApp suffix, leaving the bare number or group id.
func rawJid(id string) string {
	return strings.TrimSuffix(strings.TrimSuffix(id, messages.GROUPSUFFIX), messages.CONTACTSUFFIX)
}

// selectTreeNodeById moves the sidebar selection to the chat with the given id, if present.
func selectTreeNodeById(id string) {
	if chatRoot == nil {
		return
	}
	var walk func(n *tview.TreeNode) bool
	walk = func(n *tview.TreeNode) bool {
		for _, child := range n.GetChildren() {
			if ref, ok := child.GetReference().(messages.Chat); ok && ref.Id == id {
				treeView.SetCurrentNode(child)
				return true
			}
			if walk(child) {
				return true
			}
		}
		return false
	}
	walk(chatRoot)
}

// fuzzyMatch reports whether every rune of query appears in target in order
// (case-insensitive, subsequence match) and returns a score where higher is
// better. Contiguous runs and early matches score higher, so the most relevant
// chats float to the top. An empty query matches everything with score 0.
func fuzzyMatch(query, target string) (int, bool) {
	if strings.TrimSpace(query) == "" {
		return 0, true
	}
	q := []rune(strings.ToLower(query))
	t := []rune(strings.ToLower(target))
	qi, score, prev := 0, 0, -2
	for ti := 0; ti < len(t) && qi < len(q); ti++ {
		if t[ti] == q[qi] {
			if ti == prev+1 {
				score += 5 // contiguous match
			}
			if ti < 10 {
				score += 10 - ti // earlier match = better
			}
			prev = ti
			qi++
		}
	}
	if qi < len(q) {
		return 0, false
	}
	return score, true
}
