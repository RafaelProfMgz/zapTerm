package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"code.rocketnine.space/tslocum/cbind"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
	"github.com/rivo/tview"
	"github.com/skratchdot/open-golang/open"
	"github.com/zyedidia/clipboard"
)

var VERSION string = "v2.0.0"

var sndTxt string = ""
var currentReceiver messages.Chat = messages.Chat{}
var curRegions []messages.Message

// inlineImages caches rendered half-block art per message id, so images are
// embedded inside their (clickable) message region and survive chat switches.
var inlineImages = map[string]string{}

// lastTextClick marks the last mouse click on the message panel; it lets the
// highlight callback tell a mouse click apart from keyboard navigation.
var lastTextClick time.Time

var textView *tview.TextView
var treeView *tview.TreeView
var textInput *tview.InputField
var topBar *tview.TextView
var infoBar *tview.TextView
var hintBar *tview.TextView
var helpView *tview.TextView

var chatRoot *tview.TreeNode
var app *tview.Application
var pages *tview.Pages

var focusOrder []tview.Primitive
var prevFocus tview.Primitive
var helpVisible bool

var sessionManager *messages.SessionManager

var keyBindings *cbind.Configuration

var uiHandler messages.UiMessageHandler

func main() {
	config.InitConfig()
	// headless mode for the Ink frontend (ink-ui/): no tview, NDJSON on stdio
	for _, arg := range os.Args[1:] {
		if arg == "--ui=json" {
			runJsonUi()
			return
		}
	}
	// The clipboard backend must be probed once at startup; without this the
	// command-arg maps stay nil and the first copy/paste panics instead of
	// using xclip/xsel/wl-clipboard (or a safe in-memory fallback).
	clipboard.Initialize()
	uiHandler = UiHandler{}
	sessionManager = &messages.SessionManager{}
	sessionManager.Init(uiHandler)

	app = tview.NewApplication()

	bg := tcell.ColorNames[config.Config.Colors.Background]
	cmdPrefix := config.Config.General.CmdPrefix

	// top bar: branding (left), mock-terminal protocol header style
	topBar = tview.NewTextView()
	topBar.SetDynamicColors(true)
	topBar.SetScrollable(false)
	topBar.SetText(" [::b]ZAPTERM " + VERSION + "[-::-] [::d]— PROTOCOLO WHATSAPP · CONEXÃO CRIPTOGRAFADA[::-]")
	topBar.SetBackgroundColor(bg)

	// info bar: connection status (right)
	infoBar = tview.NewTextView()
	infoBar.SetDynamicColors(true)
	infoBar.SetTextAlign(tview.AlignRight)
	infoBar.SetBackgroundColor(bg)
	UpdateStatusBar(messages.SessionStatus{})

	// messages panel
	textView = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	textView.SetBackgroundColor(bg)
	textView.SetTextColor(tcell.ColorNames[config.Config.Colors.Text])
	textView.SetBorder(true)
	textView.SetTitle(" [ MENSAGENS ] ")
	textView.SetTitleAlign(tview.AlignLeft)
	// A mouse click on a message region highlights it (tview behavior); when the
	// highlighted message is an attachment, the click also opens it in the
	// system's native viewer. The capture below runs before the region highlight,
	// so lastTextClick distinguishes clicks from keyboard navigation.
	textView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			lastTextClick = time.Now()
		}
		return action, event
	})
	textView.SetHighlightedFunc(func(added, removed, remaining []string) {
		if len(added) == 0 || time.Since(lastTextClick) > 500*time.Millisecond {
			return
		}
		lastTextClick = time.Time{}
		for _, msg := range curRegions {
			if msg.Id != added[0] {
				continue
			}
			switch msg.Kind {
			case messages.MessageKindAudio:
				// clicking a voice message plays it right in the terminal
				sessionManager.CommandChannel <- messages.Command{"play", []string{msg.Id}}
			case messages.MessageKindImage, messages.MessageKindVideo,
				messages.MessageKindDocument:
				sessionManager.CommandChannel <- messages.Command{"open", []string{msg.Id}}
			}
			return
		}
	})

	PrintHelp()

	// input field: shell-prompt style ("você@zapterm:~$ "), like a command line
	textInput = tview.NewInputField()
	textInput.SetBackgroundColor(bg)
	textInput.SetFieldBackgroundColor(tcell.ColorNames[config.Config.Colors.InputBackground])
	textInput.SetFieldTextColor(tcell.ColorNames[config.Config.Colors.InputText])
	textInput.SetLabel("você@zapterm:~$ ")
	textInput.SetLabelColor(tcell.ColorNames[config.Config.Colors.Positive])
	textInput.SetPlaceholder("digite_mensagem_ou_comando…  (" + cmdPrefix + "comando · Enter envia)")
	textInput.SetPlaceholderTextColor(tcell.ColorNames[config.Config.Colors.Borders])
	textInput.SetBorder(true)
	textInput.SetTitleAlign(tview.AlignLeft)
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

	// chat list panel
	MakeTree()
	treeView.SetBorder(true)
	treeView.SetTitle(" [ CONVERSAS ] ")
	treeView.SetTitleAlign(tview.AlignLeft)

	// bottom hint bar (always-visible key guide)
	hintBar = tview.NewTextView()
	hintBar.SetDynamicColors(true)
	hintBar.SetScrollable(false)
	hintBar.SetBackgroundColor(bg)
	hintBar.SetText(hintText())

	// help overlay (toggled with F1 / ?)
	helpView = tview.NewTextView()
	helpView.SetDynamicColors(true).SetScrollable(true).SetWordWrap(true)
	helpView.SetBackgroundColor(bg)
	helpView.SetTextColor(tcell.ColorNames[config.Config.Colors.Text])
	helpView.SetBorder(true)
	helpView.SetTitle(" [ AJUDA ] teclas e comandos — Esc ou ? fecha ")
	helpView.SetTitleAlign(tview.AlignLeft)
	helpView.SetTitleColor(tcell.ColorNames[config.Config.Colors.ListHeader])
	helpView.SetBorderColor(tcell.ColorNames[config.Config.Colors.ListHeader])
	helpView.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape || ev.Rune() == 'q' {
			toggleHelp(nil)
			return nil
		}
		return ev
	})

	// layout: top bar | chats + messages | input | hint bar
	sideBarWidth := config.Config.Ui.ChatSidebarWidth
	gridLayout := tview.NewGrid()
	gridLayout.SetRows(1, 0, 3, 1)
	gridLayout.SetColumns(sideBarWidth, 0)
	gridLayout.SetBorders(false)
	gridLayout.SetBackgroundColor(bg)
	gridLayout.AddItem(topBar, 0, 0, 1, 1, 0, 0, false)
	gridLayout.AddItem(infoBar, 0, 1, 1, 1, 0, 0, false)
	gridLayout.AddItem(treeView, 1, 0, 1, 1, 0, 0, false)
	gridLayout.AddItem(textView, 1, 1, 1, 1, 0, 0, false)
	gridLayout.AddItem(textInput, 2, 0, 1, 2, 0, 0, false)
	gridLayout.AddItem(hintBar, 3, 0, 1, 2, 0, 0, false)

	pages = tview.NewPages()
	pages.AddPage("main", gridLayout, true, true)
	pages.AddPage("help", centeredHelp(helpView), true, false)
	buildFinder()

	focusOrder = []tview.Primitive{treeView, textView, textInput}

	app.SetRoot(pages, true)
	app.EnableMouse(true)
	// keep the focused panel's border/title highlighted on every frame
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		updateFocusBorders()
		return false
	})
	app.SetFocus(textInput)
	if err := sessionManager.StartManager(); err != nil {
		PrintError(err)
	}
	LoadShortcuts()
	app.Run()
}

// centeredHelp wraps a primitive so it floats centered over the main layout.
func centeredHelp(p tview.Primitive) tview.Primitive {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(p, 0, 3, true).
			AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 1, 0, false)
}

// updateFocusBorders highlights the border/title of the currently focused panel.
func updateFocusBorders() {
	if treeView == nil || textView == nil || textInput == nil {
		return
	}
	hi := tcell.ColorNames[config.Config.Colors.Positive]
	lo := tcell.ColorNames[config.Config.Colors.Borders]
	// NOTE: do not call app.GetFocus() here — this runs inside draw() which holds
	// the application write-lock, and GetFocus() takes the read-lock → deadlock.
	// Each primitive's HasFocus() only touches its own state, so it is safe.
	style := func(box *tview.Box, focused bool) {
		if focused {
			box.SetBorderColor(hi)
			box.SetTitleColor(hi)
		} else {
			box.SetBorderColor(lo)
			box.SetTitleColor(lo)
		}
	}
	style(treeView.Box, treeView.HasFocus())
	style(textView.Box, textView.HasFocus())
	style(textInput.Box, textInput.HasFocus())
}

// cycleFocus moves focus across the chats/messages/input panels.
func cycleFocus(delta int) {
	if len(focusOrder) == 0 {
		return
	}
	cur := app.GetFocus()
	idx := 0
	for i, p := range focusOrder {
		if p == cur {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(focusOrder)) % len(focusOrder)
	target := focusOrder[idx]
	if target != textView {
		ResetMsgSelection()
	}
	app.SetFocus(target)
}

// toggleHelp shows or hides the help overlay, preserving the previous focus.
func toggleHelp(ev *tcell.EventKey) *tcell.EventKey {
	if helpVisible {
		helpVisible = false
		pages.HidePage("help")
		if prevFocus != nil {
			app.SetFocus(prevFocus)
		}
	} else {
		helpVisible = true
		helpView.SetText(buildHelpText())
		helpView.ScrollToBeginning()
		prevFocus = app.GetFocus()
		pages.ShowPage("help")
		app.SetFocus(helpView)
	}
	return nil
}

// showHelp opens the help overlay if it is not already open.
func showHelp() {
	if !helpVisible {
		toggleHelp(nil)
	}
}

// hintText builds the always-visible bottom key guide, taskbar style:
// "[CTRL+Q] SAIR  [TAB] PAINEL ..." (keys in brackets, like the terminal mock).
func hintText() string {
	k := config.Config.Keymap
	c := config.Config.Colors.ListHeader
	q := config.Config.Colors.Negative
	key := func(color, ks, label string) string {
		return "[" + color + "::b]" + tview.Escape("["+strings.ToUpper(ks)+"]") + "[-::-] " + label
	}
	// quit comes first, in the "negative" color, so closing the app is unmissable
	return " " + key(q, k.CommandQuit, "SAIR") + "  " +
		key(c, "TAB", "PAINEL") + "  " +
		key(c, "ENTER", "ENVIAR") + "  " +
		key(c, k.FindChats, "BUSCAR") + "  " +
		key(c, "1-4", "FILTRAR") + "  " +
		key(c, k.MessagePlay, "ÁUDIO") + "  " +
		key(c, "F1", "AJUDA") + " "
}

// chat list filters, WhatsApp-Web style: a flat recency-sorted list that can be
// narrowed down to unread chats, groups or 1:1 contacts.
const (
	chatFilterAll = iota
	chatFilterUnread
	chatFilterGroups
	chatFilterContacts
)

var chatFilterNames = []string{"Todas", "Não lidas", "Grupos", "Contatos"}
var chatFilter = chatFilterAll

// creates the TreeView for chats: a flat, recency-sorted list (the tree root is
// hidden and graphics are off, so it reads like a plain chat list).
func MakeTree() *tview.TreeView {
	chatRoot = tview.NewTreeNode("Conversas").
		SetColor(tcell.ColorNames[config.Config.Colors.ListHeader])
	treeView = tview.NewTreeView().
		SetRoot(chatRoot).
		SetTopLevel(1).
		SetCurrentNode(chatRoot)
	treeView.SetGraphics(false)
	treeView.SetBackgroundColor(tcell.ColorNames[config.Config.Colors.Background])

	// Moving onto a chat opens it.
	treeView.SetChangedFunc(func(node *tview.TreeNode) {
		if chat, ok := node.GetReference().(messages.Chat); ok {
			SetDisplayedChat(chat)
		}
	})
	// Enter on a chat jumps to the input field to start typing right away.
	treeView.SetSelectedFunc(func(node *tview.TreeNode) {
		if chat, ok := node.GetReference().(messages.Chat); ok {
			SetDisplayedChat(chat)
			app.SetFocus(textInput)
		}
	})
	return treeView
}

// setChatFilter switches the chat list filter and rebuilds the list.
func setChatFilter(filter int) {
	if filter < 0 || filter >= len(chatFilterNames) {
		return
	}
	chatFilter = filter
	rebuildChatTree()
}

func handleChatFilter(filter int) func(ev *tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		setChatFilter(filter)
		return nil
	}
}

func handleChatFilterCycle(ev *tcell.EventKey) *tcell.EventKey {
	setChatFilter((chatFilter + 1) % len(chatFilterNames))
	return nil
}

// chatPassesFilter reports whether a chat is visible under the current filter.
// hasActivity tells whether any chat has messages at all: before the first
// history sync nothing has activity yet, and "Todas" then shows everything.
func chatPassesFilter(c messages.Chat, hasActivity bool) bool {
	switch chatFilter {
	case chatFilterUnread:
		return c.Unread > 0
	case chatFilterGroups:
		return c.IsGroup
	case chatFilterContacts:
		return !c.IsGroup
	default:
		return !hasActivity || c.LastMessage > 0
	}
}

// weekdaysPt maps time.Weekday to short Portuguese day names.
var weekdaysPt = [...]string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}

// shortChatTime renders a WhatsApp-Web style timestamp for the chat list:
// today → "15:04", last week → weekday, otherwise → "02/01".
func shortChatTime(ts int64) string {
	t := time.Unix(ts, 0)
	now := time.Now()
	ty, tm, td := t.Date()
	ny, nm, nd := now.Date()
	if ty == ny && tm == nm && td == nd {
		return t.Format("15:04")
	}
	if now.Sub(t) < 7*24*time.Hour {
		return weekdaysPt[t.Weekday()]
	}
	return t.Format("02/01")
}

// formatChatEntry renders one chat list line: "*" marks the open chat (like
// the mock), then the name, and on the right either the unread count ("[3]")
// or the time of the last message. Groups get a minimal "#" marker, no emojis.
func formatChatEntry(c messages.Chat, width int) string {
	name := c.Name
	if name == "" {
		name = "+" + rawJid(c.Id)
	}
	if c.IsGroup {
		name = "# " + name
	}
	marker := "  "
	if c.Id != "" && c.Id == currentReceiver.Id {
		marker = "* "
	}
	name = marker + name

	right := ""
	rightWidth := 0
	if c.Unread > 0 {
		badge := fmt.Sprintf("[%d]", c.Unread)
		rightWidth = runewidth.StringWidth(badge)
		right = "[" + config.Config.Colors.UnreadCount + "::b]" + tview.Escape(badge) + "[-::-]"
	} else if c.LastMessage > 0 {
		right = shortChatTime(c.LastMessage)
		rightWidth = runewidth.StringWidth(right)
		right = "[::d]" + right + "[::-]"
	}

	avail := width - rightWidth - 1
	if avail < 4 {
		avail = 4
	}
	name = runewidth.Truncate(name, avail, "…")
	pad := avail - runewidth.StringWidth(name) + 1
	if pad < 1 {
		pad = 1
	}
	out := tview.Escape(name)
	if c.Unread > 0 {
		out = "[::b]" + out + "[::-]"
	}
	return out + strings.Repeat(" ", pad) + right
}

// refreshChatMarkers re-renders the sidebar labels so the "*" marker follows
// the open chat without rebuilding the tree (which would move the selection).
func refreshChatMarkers() {
	if chatRoot == nil {
		return
	}
	width := config.Config.Ui.ChatSidebarWidth - 2
	for _, node := range chatRoot.GetChildren() {
		if c, ok := node.GetReference().(messages.Chat); ok {
			node.SetText(formatChatEntry(c, width))
		}
	}
}

// rebuildChatTree repopulates the sidebar from allChats honoring the active
// filter, keeping the selection on the current chat when possible.
func rebuildChatTree() {
	if chatRoot == nil {
		return
	}
	chatRoot.ClearChildren()
	oldId := currentReceiver.Id

	hasActivity := false
	for _, c := range allChats {
		if c.LastMessage > 0 {
			hasActivity = true
			break
		}
	}

	// 2 columns are lost to the panel borders
	width := config.Config.Ui.ChatSidebarWidth - 2
	var selectedNode *tview.TreeNode
	shown := 0
	for _, element := range allChats {
		// keep currentReceiver fresh (name/unread may have changed)
		if element.Id == oldId {
			currentReceiver = element
		}
		if !chatPassesFilter(element, hasActivity) {
			continue
		}
		node := tview.NewTreeNode(formatChatEntry(element, width)).
			SetReference(element).
			SetSelectable(true)
		if element.IsGroup {
			node.SetColor(tcell.ColorNames[config.Config.Colors.ListGroup])
		} else {
			node.SetColor(tcell.ColorNames[config.Config.Colors.ListContact])
		}
		if selectedNode == nil && element.Id == currentReceiver.Id {
			selectedNode = node
		}
		chatRoot.AddChild(node)
		shown++
	}
	treeView.SetTitle(fmt.Sprintf(" [ %s (%d) ] ", strings.ToUpper(chatFilterNames[chatFilter]), shown))
	if selectedNode != nil {
		treeView.SetCurrentNode(selectedNode)
	}
}

func handleFocusMessage(ev *tcell.EventKey) *tcell.EventKey {
	if !textView.HasFocus() {
		app.SetFocus(textView)
		if curRegions != nil && len(curRegions) > 0 {
			textView.Highlight(curRegions[len(curRegions)-1].Id)
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
	cycleFocus(1)
	return nil
}

func handleOpenFinder(ev *tcell.EventKey) *tcell.EventKey {
	openFinder()
	return nil
}

func handleSwitchPanelsBack(ev *tcell.EventKey) *tcell.EventKey {
	cycleFocus(-1)
	return nil
}

func handleCommand(command string) func(ev *tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		sessionManager.CommandChannel <- messages.Command{command, nil}
		return nil
	}
}

func handleCopyUser(ev *tcell.EventKey) *tcell.EventKey {
	if hls := textView.GetHighlights(); len(hls) > 0 {
		for _, val := range curRegions {
			if val.Id == hls[0] {
				copyId(val.ContactId, val.ContactName)
			}
		}
		ResetMsgSelection()
	} else if currentReceiver.Id != "" {
		copyId(currentReceiver.Id, currentReceiver.Name)
	} else {
		PrintText("nenhuma conversa selecionada — escolha uma conversa primeiro")
	}
	return nil
}

// copyId always prints the id (so it is readable even without a clipboard tool)
// and additionally tries to copy it to the system clipboard.
func copyId(id, name string) {
	if id == "" {
		return
	}
	PrintText("[" + config.Config.Colors.ListHeader + "::b]id de " + name + ":[-::-] " + id)
	if err := safeWriteClipboard(id); err != nil {
		PrintText("[::d](clipboard indisponível — instale xclip/xsel/wl-clipboard ou selecione o id acima)[::-]")
	} else {
		PrintText("[" + config.Config.Colors.Positive + "]✓ copiado para a área de transferência[-]")
	}
}

func safeWriteClipboard(text string) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("clipboard unavailable: %v", rec)
		}
	}()
	return clipboard.WriteAll(text, "clipboard")
}

func handlePasteUser(ev *tcell.EventKey) *tcell.EventKey {
	if clip, err := safeReadClipboard(); err == nil {
		textInput.SetText(textInput.GetText() + " " + clip)
	} else {
		PrintError(err)
	}
	return nil
}

func safeReadClipboard() (clip string, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("clipboard paste is unavailable: %v", rec)
		}
	}()
	return clipboard.ReadAll("clipboard")
}

func handleQuit(ev *tcell.EventKey) *tcell.EventKey {
	stopAudio()
	sessionManager.CommandChannel <- messages.Command{"disconnect", nil}
	app.Stop()
	return nil
}

func handleHelp(ev *tcell.EventKey) *tcell.EventKey {
	return toggleHelp(ev)
}

// handleHelpRune toggles help on '?', but lets '?' be typed normally in the input field.
func handleHelpRune(ev *tcell.EventKey) *tcell.EventKey {
	if textInput.HasFocus() && !helpVisible {
		return ev
	}
	return toggleHelp(ev)
}

func handleMessageCommand(command string) func(ev *tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		hls := textView.GetHighlights()
		if len(hls) > 0 {
			sessionManager.CommandChannel <- messages.Command{command, []string{hls[0]}}
			ResetMsgSelection()
			app.SetFocus(textInput)
		}
		return nil
	}
}

func handleMessagesMove(amount int) func(ev *tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		if curRegions == nil || len(curRegions) == 0 {
			return nil
		}
		hls := textView.GetHighlights()
		if len(hls) > 0 {
			newId := GetOffsetMsgId(hls[0], amount)
			if newId != "" {
				textView.Highlight(newId)
			}
		} else {
			if amount < 0 {
				textView.Highlight(curRegions[0].Id)
			} else {
				textView.Highlight(curRegions[len(curRegions)-1].Id)
			}
		}
		textView.ScrollToHighlight()
		return nil
	}
}

func handleChatPanelUp(ev *tcell.EventKey) *tcell.EventKey {
	//TODO: scroll selection in treeView? or chatRoot? How?
	return ev
}

func handleChatPanelDown(ev *tcell.EventKey) *tcell.EventKey {
	return ev
}

// handlePlayMessage plays/stops the highlighted audio message. Unlike the other
// message commands it keeps the highlight, so pressing the key again stops it.
func handlePlayMessage(ev *tcell.EventKey) *tcell.EventKey {
	if hls := textView.GetHighlights(); len(hls) > 0 {
		sessionManager.CommandChannel <- messages.Command{"play", []string{hls[0]}}
	}
	return nil
}

func handleMessagesLast(ev *tcell.EventKey) *tcell.EventKey {
	if curRegions == nil || len(curRegions) == 0 {
		return nil
	}
	textView.Highlight(curRegions[len(curRegions)-1].Id)
	textView.ScrollToHighlight()
	return nil
}

func handleMessagesFirst(ev *tcell.EventKey) *tcell.EventKey {
	if curRegions == nil || len(curRegions) == 0 {
		return nil
	}
	textView.Highlight(curRegions[0].Id)
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

// load the key map
func LoadShortcuts() {
	// global bindings for app
	keyBindings = cbind.NewConfiguration()
	if err := keyBindings.Set(config.Config.Keymap.FocusMessages, handleFocusMessage); err != nil {
		PrintErrorMsg("focus_messages:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.FocusInput, handleFocusInput); err != nil {
		PrintErrorMsg("focus_input:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.FocusChats, handleFocusContacts); err != nil {
		PrintErrorMsg("focus_contacts:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.FindChats, handleOpenFinder); err != nil {
		PrintErrorMsg("find_chats:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.SwitchPanels, handleSwitchPanels); err != nil {
		PrintErrorMsg("switch_panels:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.CommandRead, handleCommand("read")); err != nil {
		PrintErrorMsg("command_read:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.Copyuser, handleCopyUser); err != nil {
		PrintErrorMsg("copyuser:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.Pasteuser, handlePasteUser); err != nil {
		PrintErrorMsg("pasteuser:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.CommandBacklog, handleCommand("backlog")); err != nil {
		PrintErrorMsg("command_backlog:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.CommandConnect, handleCommand("login")); err != nil {
		PrintErrorMsg("command_connect:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.CommandQuit, handleQuit); err != nil {
		PrintErrorMsg("command_quit:", err)
	}
	if err := keyBindings.Set(config.Config.Keymap.CommandHelp, handleHelp); err != nil {
		PrintErrorMsg("command_help:", err)
	}
	// always-available copy-id on Ctrl+y, so it works even if a saved config
	// still maps copy to Ctrl+c (which most terminals eat as SIGINT / quit).
	keyBindings.SetKey(tcell.ModNone, tcell.KeyCtrlY, handleCopyUser)
	// easy, always-available help + reverse panel cycling
	keyBindings.SetKey(tcell.ModNone, tcell.KeyF1, handleHelp)
	keyBindings.SetRune(tcell.ModNone, '?', handleHelpRune)
	keyBindings.SetKey(tcell.ModNone, tcell.KeyBacktab, handleSwitchPanelsBack)
	// while the finder overlay is open it owns every keystroke; otherwise the
	// global shortcuts (Tab, Ctrl+f, …) apply.
	app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if finderVisible {
			return ev
		}
		return keyBindings.Capture(ev)
	})
	// bindings for chat message text view
	keysMessages := cbind.NewConfiguration()
	if err := keysMessages.Set(config.Config.Keymap.MessageDownload, handleMessageCommand("download")); err != nil {
		PrintErrorMsg("message_download:", err)
	}
	if err := keysMessages.Set(config.Config.Keymap.MessageOpen, handleMessageCommand("open")); err != nil {
		PrintErrorMsg("message_open:", err)
	}
	if err := keysMessages.Set(config.Config.Keymap.Copyuser, handleCopyUser); err != nil {
		PrintErrorMsg("copyuser:", err)
	}
	if err := keysMessages.Set(config.Config.Keymap.Pasteuser, handlePasteUser); err != nil {
		PrintErrorMsg("pasteuser:", err)
	}
	if err := keysMessages.Set(config.Config.Keymap.MessageShow, handleMessageCommand("show")); err != nil {
		PrintErrorMsg("message_show:", err)
	}
	if err := keysMessages.Set(config.Config.Keymap.MessagePlay, handlePlayMessage); err != nil {
		PrintErrorMsg("message_play:", err)
	}
	if err := keysMessages.Set(config.Config.Keymap.MessageUrl, handleMessageCommand("url")); err != nil {
		PrintErrorMsg("message_url:", err)
	}
	if err := keysMessages.Set(config.Config.Keymap.MessageInfo, handleMessageCommand("info")); err != nil {
		PrintErrorMsg("message_info:", err)
	}
	if err := keysMessages.Set(config.Config.Keymap.MessageRevoke, handleMessageCommand("revoke")); err != nil {
		PrintErrorMsg("message_revoke:", err)
	}
	keysMessages.SetKey(tcell.ModNone, tcell.KeyEscape, handleExitMessages)
	keysMessages.SetKey(tcell.ModNone, tcell.KeyUp, handleMessagesMove(-1))
	keysMessages.SetKey(tcell.ModNone, tcell.KeyDown, handleMessagesMove(1))
	keysMessages.SetKey(tcell.ModNone, tcell.KeyPgUp, handleMessagesMove(-10))
	keysMessages.SetKey(tcell.ModNone, tcell.KeyPgDn, handleMessagesMove(10))
	keysMessages.SetRune(tcell.ModNone, 'k', handleMessagesMove(-1))
	keysMessages.SetRune(tcell.ModNone, 'j', handleMessagesMove(1))
	keysMessages.SetRune(tcell.ModNone, 'g', handleMessagesFirst)
	keysMessages.SetRune(tcell.ModNone, 'G', handleMessagesLast)
	keysMessages.SetRune(tcell.ModCtrl, 'u', handleMessagesMove(-10))
	keysMessages.SetRune(tcell.ModCtrl, 'd', handleMessagesMove(10))
	textView.SetInputCapture(keysMessages.Capture)
	keysChatPanel := cbind.NewConfiguration()
	keysChatPanel.SetRune(tcell.ModCtrl, 'u', handleChatPanelUp)
	keysChatPanel.SetRune(tcell.ModCtrl, 'd', handleChatPanelDown)
	// chat list filters, WhatsApp-Web style tabs: 1 Todas · 2 Não lidas ·
	// 3 Grupos · 4 Contatos; f cycles through them.
	for i := range chatFilterNames {
		keysChatPanel.SetRune(tcell.ModNone, rune('1'+i), handleChatFilter(i))
	}
	keysChatPanel.SetRune(tcell.ModNone, 'f', handleChatFilterCycle)
	treeView.SetInputCapture(keysChatPanel.Capture)
}

// PrintHelp prints a short welcome into the message panel, boot-log style.
func PrintHelp() {
	cmdPrefix := config.Config.General.CmdPrefix
	hdr := config.Config.Colors.ListHeader
	fmt.Fprintln(textView, "[-:-:b]ZAPTERM "+VERSION+" — PROTOCOLO WHATSAPP[-:-:-]")
	fmt.Fprintln(textView, "[::d]<<< terminal seguro · criptografia de ponta a ponta >>>[::-]")
	fmt.Fprintln(textView, "")
	fmt.Fprintln(textView, " > [::b]Tab[::-] alterna os painéis: [::b]Conversas[::-] → [::b]Mensagens[::-] → [::b]digitação[::-].")
	fmt.Fprintln(textView, " > Escolha uma conversa à esquerda (↑/↓ e Enter) e digite embaixo para responder.")
	fmt.Fprintln(textView, " > Na lista: [::b]1-4[::-] filtram (Todas · Não lidas · Grupos · Contatos).")
	fmt.Fprintln(textView, " > Áudios: clique ou selecione e pressione ["+hdr+"::b]"+config.Config.Keymap.MessagePlay+"[-::-] para tocar no terminal.")
	fmt.Fprintln(textView, " > Pressione ["+hdr+"::b]F1[-::-] ou ["+hdr+"::b]?[-::-] a qualquer momento para o guia completo.")
	fmt.Fprintln(textView, " > "+cmdPrefix+"connect conecta · "+cmdPrefix+"quit (ou "+config.Config.Keymap.CommandQuit+") sai.")
	fmt.Fprintln(textView, "")
}

// buildHelpText returns the full keys + commands guide shown in the help overlay.
func buildHelpText() string {
	cmdPrefix := config.Config.General.CmdPrefix
	k := config.Config.Keymap
	hdr := config.Config.Colors.ListHeader
	var b strings.Builder
	sec := func(title string) { fmt.Fprintf(&b, "\n["+hdr+"::b]%s[-::-]\n", title) }
	row := func(keys, desc string) { fmt.Fprintf(&b, "  [::b]%-18s[::-] %s\n", keys, desc) }

	fmt.Fprintln(&b, "["+hdr+"::bu]ZapTerm "+VERSION+" — Guia rápido[-:-:-]")

	sec("Navegação")
	row("Tab / Shift+Tab", "trocar de painel (Conversas/Mensagens/Digitação)")
	row(k.FindChats, "buscar conversa por nome, número ou id (Tab muda escopo)")
	row(k.FocusChats, "ir para a lista de Conversas")
	row(k.FocusMessages, "ir para o painel de Mensagens")
	row(k.FocusInput, "ir para o campo de digitação")
	row("↑ / ↓", "navegar (conversas, mensagens ou rolar histórico)")
	row("F1 ou ?", "abrir/fechar esta ajuda")
	row(k.CommandQuit, "sair do app")

	sec("Lista de conversas (com a lista focada)")
	row("1 / 2 / 3 / 4", "filtrar: Todas · Não lidas · Grupos · Contatos")
	row("f", "alternar entre os filtros")
	row("Enter", "abrir a conversa e ir direto para a digitação")

	sec("Conversa")
	row("Enter", "enviar a mensagem digitada")
	row(cmdPrefix+"backlog / "+k.CommandBacklog, fmt.Sprintf("carregar %d mensagens anteriores", config.Config.General.BacklogMsgQuantity))
	row(cmdPrefix+"read / "+k.CommandRead, "marcar a conversa como lida")
	row(cmdPrefix+"upload <arquivo>", "enviar arquivo como documento")
	row(cmdPrefix+"sendimage <arquivo>", "enviar imagem")
	row(cmdPrefix+"sendvideo <arquivo>", "enviar vídeo")
	row(cmdPrefix+"sendaudio <arquivo>", "enviar áudio")

	sec("Painel de mensagens (selecione uma mensagem com ↑/↓)")
	row("clique (mouse)", "áudio: toca no terminal · outros anexos: abre no sistema")
	row(k.MessagePlay, "tocar/parar áudio ou vídeo (player: mpv/ffplay/sox/vlc)")
	row(k.MessageDownload, "baixar anexo")
	row(k.MessageOpen, "baixar e abrir anexo")
	if canRenderInlineImages() {
		row(k.MessageShow, "baixar e exibir a imagem dentro do terminal")
	} else {
		row(k.MessageShow, "baixar e exibir imagem ("+config.Config.General.ShowCommand+")")
	}
	row(k.MessageUrl, "abrir URL encontrada na mensagem")
	row(k.MessageInfo, "informações da mensagem")
	row(k.MessageRevoke, "apagar/revogar mensagem")
	row("Esc", "sair do modo de seleção de mensagem")

	sec("Conexão")
	row(cmdPrefix+"connect / "+k.CommandConnect, "(re)conectar ao WhatsApp")
	row(cmdPrefix+"disconnect", "encerrar a conexão")
	row(cmdPrefix+"logout", "remover o login deste computador")
	row(cmdPrefix+"reset", "limpar a sessão e reconectar do zero")

	sec("Grupos")
	row(cmdPrefix+"create <ids> Assunto", "criar grupo com os usuários")
	row(cmdPrefix+"subject <texto>", "mudar o assunto do grupo")
	row(cmdPrefix+"add <id>", "adicionar usuário")
	row(cmdPrefix+"remove <id>", "remover usuário")
	row(cmdPrefix+"admin <id>", "tornar admin")
	row(cmdPrefix+"removeadmin <id>", "remover admin")
	row(cmdPrefix+"leave", "sair do grupo")

	sec("IDs")
	row(k.Copyuser+" ou "+cmdPrefix+"id", "copiar/mostrar o id da conversa atual (ex.: para o bot)")
	row(k.Pasteuser, "colar o id no campo de digitação")
	row("Ctrl+y (na busca)", "copiar o id da conversa destacada na busca")

	sec("Bot de IA (opcional)")
	row("/ai", "(no chat) iniciar conversa com a IA — responde cada mensagem (streaming)")
	row("/end", "(no chat) encerrar a conversa com a IA")
	row("/key sk-...", "(no chat) usar sua própria conta OpenAI — contexto maior")
	row("/key reset", "(no chat) voltar ao bot padrão")

	fmt.Fprintf(&b, "\n[::d]Config: %s[-::-]\n", config.GetConfigFilePath())
	return b.String()
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
	cmdPrefix := config.Config.General.CmdPrefix
	if sndTxt == cmdPrefix+"help" || sndTxt == cmdPrefix+"commands" {
		showHelp()
		textInput.SetText("")
		return
	}
	if sndTxt == cmdPrefix+"quit" {
		stopAudio()
		sessionManager.CommandChannel <- messages.Command{"disconnect", nil}
		app.Stop()
		return
	}
	if sndTxt == cmdPrefix+"id" {
		handleCopyUser(nil)
		textInput.SetText("")
		return
	}
	if strings.HasPrefix(sndTxt, cmdPrefix) {
		cmd := strings.TrimPrefix(sndTxt, cmdPrefix)
		var params []string
		if strings.Index(cmd, " ") >= 0 {
			cmdParts := strings.Split(cmd, " ")
			cmd = cmdParts[0]
			params = cmdParts[1:]
		}
		sessionManager.CommandChannel <- messages.Command{cmd, params}
		textInput.SetText("")
		return
	}
	if currentReceiver.Id == "" {
		PrintText("no receiver")
		textInput.SetText("")
		return
	}
	// no command, send as message
	msg := messages.Command{
		Name:   "send",
		Params: []string{currentReceiver.Id, sndTxt},
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
		if val.Id == curId {
			arrPos := idx + offset
			if len(curRegions) > arrPos && arrPos >= 0 {
				return curRegions[arrPos].Id
			}
		}
	}
	if offset > 0 {
		return curRegions[0].Id
	} else {
		return curRegions[len(curRegions)-1].Id
	}
}

// redrawChat re-renders the open chat (e.g. when playback state changes).
func redrawChat() {
	textView.SetText(getMessagesString(curRegions))
	if len(textView.GetHighlights()) > 0 {
		textView.ScrollToHighlight()
	} else {
		textView.ScrollToEnd()
	}
}

// queueRefreshChat re-renders the open chat from the UI goroutine; safe to call
// from any goroutine (audio playback callbacks, session manager, …). In
// headless mode there is no tview app — the playback state goes to the
// frontend as an event instead.
func queueRefreshChat() {
	if jsonUi != nil {
		jsonUi.emit(map[string]any{"type": "playing", "msgId": currentPlayingMsgId()})
		return
	}
	go app.QueueUpdateDraw(redrawChat)
}

// queuePrintError prints an error from any goroutine.
func queuePrintError(err error) {
	if jsonUi != nil {
		jsonUi.PrintError(err)
		return
	}
	go app.QueueUpdateDraw(func() {
		PrintError(err)
	})
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
	fmt.Fprintln(textView, "["+config.Config.Colors.Negative+"]", err.Error(), "[-]")
}

// prints an error to the TextView
func PrintErrorMsg(text string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(textView, "["+config.Config.Colors.Negative+"]", text, err.Error(), "[-]")
}

// prints an image attachment to the TextView (by message id)
func PrintImage(path string) {
	var err error
	cmdParts := strings.Split(config.Config.General.ShowCommand, " ")
	cmdParts = append(cmdParts, path)
	var cmd *exec.Cmd
	size := len(cmdParts)
	if size > 1 {
		cmd = exec.Command(cmdParts[0], cmdParts[1:]...)
	} else if size > 0 {
		cmd = exec.Command(cmdParts[0])
	}
	var stdout io.ReadCloser
	if stdout, err = cmd.StdoutPipe(); err == nil {
		if err = cmd.Start(); err == nil {
			reader := bufio.NewReader(stdout)
			io.Copy(tview.ANSIWriter(textView), reader)
			return
		}
	}
	PrintError(err)
	PrintText("[::d]este terminal não exibe imagens — clique na mensagem para abrir no visualizador do sistema[::-]")
}

// showInlineImage renders a downloaded image inside its message region and
// repaints the chat. Returns false if the terminal/format is not supported.
func showInlineImage(path, msgId string) bool {
	if msgId == "" || !canRenderInlineImages() {
		return false
	}
	_, _, width, _ := textView.GetInnerRect()
	cols := width - 2
	if cols > 100 {
		cols = 100
	}
	block, err := renderInlineImage(path, cols, config.Config.General.InlineImageLines)
	if err != nil {
		return false // e.g. webp sticker — fall back to the external viewer flow
	}
	inlineImages[msgId] = block
	textView.SetText(getMessagesString(curRegions))
	if len(textView.GetHighlights()) > 0 {
		textView.ScrollToHighlight()
	} else {
		textView.ScrollToEnd()
	}
	return true
}

// maybeAutoShowImage downloads and renders incoming images of the open chat
// automatically when the terminal can display them inline.
func maybeAutoShowImage(msg messages.Message) {
	if msg.Kind != messages.MessageKindImage || !canRenderInlineImages() {
		return
	}
	if _, ok := inlineImages[msg.Id]; ok {
		return
	}
	select {
	case sessionManager.CommandChannel <- messages.Command{"show", []string{msg.Id}}:
	default: // channel full — the user can still press 's' or click the message
	}
}

// updates the status bar: "[ONLINE]"/"[OFFLINE]" tag, terminal-mock style.
// (battery info was dropped — the multi-device API doesn't report it anymore)
func UpdateStatusBar(statusInfo messages.SessionStatus) {
	out := ""
	if statusInfo.Connected {
		out += "[" + config.Config.Colors.Positive + "::b]" + tview.Escape("[ONLINE]") + "[-::-]"
	} else {
		out += "[" + config.Config.Colors.Negative + "::b]" + tview.Escape("[OFFLINE]") + "[-::-]"
	}
	if statusInfo.LastSeen != "" {
		out += " [::d]" + tview.Escape(statusInfo.LastSeen) + "[::-]"
	}
	infoBar.SetText(out + " ")
}

// sets the current chat, loads text from storage to TextView
func SetDisplayedChat(wid messages.Chat) {
	//TODO: how to get chat to set
	currentReceiver = wid
	textView.Clear()
	if wid.Name != "" {
		textView.SetTitle(" [ " + wid.Name + " ] ")
	} else {
		textView.SetTitle(" [ MENSAGENS ] ")
	}
	refreshChatMarkers()
	sessionManager.CommandChannel <- messages.Command{"select", []string{currentReceiver.Id}}
}

// dateSeparator renders a dim day marker between messages, terminal-mock style.
func dateSeparator(t time.Time) string {
	return fmt.Sprintf("[::d]<<< %s, %s >>>[::-]",
		weekdaysPt[t.Weekday()], t.Format("02/01/2006"))
}

// msgDay returns the calendar day of a message, used to place date separators.
func msgDay(msg *messages.Message) string {
	return time.Unix(int64(msg.Timestamp), 0).Format("2006-01-02")
}

// get a string representation of all messages for chat, with a date separator
// every time the calendar day changes.
func getMessagesString(msgs []messages.Message) string {
	out := ""
	lastDay := ""
	for _, msg := range msgs {
		if day := msgDay(&msg); day != lastDay {
			out += dateSeparator(time.Unix(int64(msg.Timestamp), 0)) + "\n"
			lastDay = day
		}
		out += getTextMessageString(&msg)
		out += "\n"
	}
	return out
}

// formatDuration renders seconds as m:ss for audio/video lengths.
func formatDuration(secs uint32) string {
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

// mediaHint returns the usage hint appended to attachment messages, with the
// keys in brackets like the mock ("0:23 · [p] tocar · [o] abrir"). The caller
// escapes it, so brackets here are literal.
func mediaHint(msg *messages.Message) string {
	k := config.Config.Keymap
	switch msg.Kind {
	case messages.MessageKindAudio:
		hint := "[" + k.MessagePlay + "/clique] tocar · [" + k.MessageOpen + "] abrir"
		if msg.DurationSecs > 0 {
			hint = formatDuration(msg.DurationSecs) + " · " + hint
		}
		return hint
	case messages.MessageKindVideo:
		hint := "[" + k.MessagePlay + "] tocar · [" + k.MessageOpen + "/clique] abrir"
		if msg.DurationSecs > 0 {
			hint = formatDuration(msg.DurationSecs) + " · " + hint
		}
		return hint
	case messages.MessageKindImage:
		return "[" + k.MessageShow + "] exibir · [" + k.MessageOpen + "/clique] abrir"
	case messages.MessageKindDocument:
		return "[" + k.MessageOpen + "/clique] abrir"
	}
	return ""
}

// create a formatted string with regions based on message ID from a text
// message, IRC style: "[15:04:12] <nome> texto".
func getTextMessageString(msg *messages.Message) string {
	colorMe := config.Config.Colors.ChatMe
	colorContact := config.Config.Colors.ChatContact
	out := ""
	text := tview.Escape(msg.Text)
	if msg.Forwarded {
		text = "[" + config.Config.Colors.ForwardedText + "]" + text + "[-]"
	}
	stamp := time.Unix(int64(msg.Timestamp), 0).Format("15:04:05")
	out += "[\""
	out += msg.Id
	out += "\"]"
	out += "[-::d]" + tview.Escape("["+stamp+"]") + "[-::-] "
	if msg.FromMe { //msg from me
		out += "[" + colorMe + "::b]<eu>[-::-] " + text
	} else { // message from others
		out += "[" + colorContact + "::b]<" + tview.Escape(msg.ContactShort) + ">[-::-] " + text
	}
	if hint := mediaHint(msg); hint != "" {
		out += " [::d]" + tview.Escape(hint) + "[::-]"
	}
	if msg.Id != "" && msg.Id == currentPlayingMsgId() {
		out += " [" + config.Config.Colors.Positive + "::b]▶ TOCANDO[-::-] [::d]" +
			tview.Escape("["+config.Config.Keymap.MessagePlay+"] parar") + "[::-]"
	}
	// an already-rendered image lives inside the message region, so clicking
	// the picture itself also opens the native viewer
	if block, ok := inlineImages[msg.Id]; ok {
		out += "\n" + block
	}
	out += "[\"\"]"
	return out
}

type UiHandler struct{}

func (u UiHandler) NewMessage(msg messages.Message) {
	//TODO: its stupid to "go" this as its supposed to run
	//on the ui thread anyway. But QueueUpdate blocks...?
	go app.QueueUpdateDraw(func() {
		// emit a date separator when the incoming message starts a new day
		if len(curRegions) == 0 || msgDay(&curRegions[len(curRegions)-1]) != msgDay(&msg) {
			PrintText(dateSeparator(time.Unix(int64(msg.Timestamp), 0)))
		}
		curRegions = append(curRegions, msg)
		PrintText(getTextMessageString(&msg))
		maybeAutoShowImage(msg)
	})
}

func (u UiHandler) NewScreen(msgs []messages.Message) {
	go app.QueueUpdateDraw(func() {
		textView.Clear()
		screen := getMessagesString(msgs)
		textView.SetText(screen)
		curRegions = msgs
		if screen == "" {
			if currentReceiver.Id == "" {
				PrintHelp()
			} else {
				PrintText("[::d]<<< sem mensagens — " + tview.Escape("["+config.Config.Keymap.CommandBacklog+"]") + " carrega o histórico >>>[::-]")
			}
		}
	})
}

// loads the chat data from storage to the chat list; ids already come sorted
// by most recent message first.
func (u UiHandler) SetChats(ids []messages.Chat) {
	go app.QueueUpdateDraw(func() {
		allChats = ids // keep the full list for the finder (Ctrl+f) and filters
		rebuildChatTree()
	})
}

func (u UiHandler) PrintError(err error) {
	PrintError(err)
}

func (u UiHandler) PrintText(msg string) {
	PrintText(msg)
}

func (u UiHandler) PrintFile(path string, msgId string) {
	go app.QueueUpdateDraw(func() {
		if showInlineImage(path, msgId) {
			return
		}
		PrintImage(path)
	})
}

func (u UiHandler) PlayFile(path string, msgId string) {
	playAudioFile(path, msgId)
}

func (u UiHandler) OpenFile(path string) {
	open.Run(path)
}

func (u UiHandler) SetStatus(status messages.SessionStatus) {
	go app.QueueUpdateDraw(func() {
		UpdateStatusBar(status)
	})
}

// SetStories is a no-op in the tview UI for now — the stories view lives in the
// experimental Ink frontend (--ui=json). The method exists to satisfy
// messages.UiMessageHandler.
func (u UiHandler) SetStories(_ []messages.StatusUpdate) {}

func (u UiHandler) GetWriter() io.Writer {
	return textView
}
