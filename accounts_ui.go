package main

import (
	"fmt"
	"io"

	"github.com/normen/whatscli/messages"
	"github.com/rivo/tview"
)

// tviewAccounts hosts several accounts in the tview UI, which only ever shows
// the active one: every account's session gets an accountFilter that drops
// what belongs to the others. Switching is done with /conta; the account
// sidebar exists only in the Ink frontend.
type tviewAccounts struct{}

func (tviewAccounts) ForAccount(id string) messages.UiMessageHandler {
	return accountFilter{id: id}
}

// SetAccounts is a no-op: there is no account list on screen (use /contas).
func (tviewAccounts) SetAccounts(_ []messages.AccountInfo, _ string) {}

// SetActiveAccount drops the open chat of the previous account; the new
// account's chat list and status follow as ordinary events.
func (tviewAccounts) SetActiveAccount(id string) {
	go app.QueueUpdateDraw(func() {
		currentReceiver = messages.Chat{}
		curRegions = nil
		textView.Clear()
		textView.SetTitle(" [ MENSAGENS ] ")
		PrintText("[::d]<<< conta ativa: " + tview.Escape(id) + " >>>[::-]")
	})
}

// PrintAccounts escapes the listing: "[ONLINE]" would otherwise be eaten as
// a color tag.
func (tviewAccounts) PrintAccounts(accounts []messages.AccountInfo, active string) {
	lines := messages.AccountLines(accounts, active)
	go app.QueueUpdateDraw(func() {
		for _, line := range lines {
			PrintText(tview.Escape(line))
		}
	})
}

// accountFilter forwards one account's events to UiHandler while that account
// is active. Errors of background accounts still show up, prefixed with the
// account id; everything else from them is dropped (their state is re-sent
// when the user switches to them).
type accountFilter struct {
	id string
}

func (f accountFilter) active() bool {
	return accountManager != nil && accountManager.Active() == f.id
}

func (f accountFilter) NewMessage(msg messages.Message) {
	if f.active() {
		uiHandler.NewMessage(msg)
	}
}

func (f accountFilter) NewScreen(msgs []messages.Message) {
	if f.active() {
		uiHandler.NewScreen(msgs)
	}
}

func (f accountFilter) SetChats(chats []messages.Chat) {
	if f.active() {
		uiHandler.SetChats(chats)
	}
}

func (f accountFilter) PrintError(err error) {
	if err == nil {
		return
	}
	if !f.active() {
		err = fmt.Errorf("%s: %v", tview.Escape("["+f.id+"]"), err)
	}
	uiHandler.PrintError(err)
}

func (f accountFilter) PrintText(msg string) {
	if f.active() {
		uiHandler.PrintText(msg)
	}
}

// files and audio were asked for by the user, so they pass through
func (f accountFilter) PrintFile(path string, msgId string) { uiHandler.PrintFile(path, msgId) }
func (f accountFilter) PlayFile(path string, msgId string)  { uiHandler.PlayFile(path, msgId) }
func (f accountFilter) OpenFile(path string)                { uiHandler.OpenFile(path) }

func (f accountFilter) SetStatus(status messages.SessionStatus) {
	if f.active() {
		uiHandler.SetStatus(status)
	}
}

func (f accountFilter) SetQRCode(qr messages.QRCode) {
	if f.active() {
		uiHandler.SetQRCode(qr)
	}
}

func (f accountFilter) SetStories(stories []messages.StatusUpdate) {
	if f.active() {
		uiHandler.SetStories(stories)
	}
}

func (f accountFilter) GetWriter() io.Writer {
	if f.active() {
		return uiHandler.GetWriter()
	}
	return io.Discard
}
