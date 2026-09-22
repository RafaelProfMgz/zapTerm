package messages

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/normen/whatscli/config"
)

// AccountInfo is one row of the account list pushed to the UI.
type AccountInfo struct {
	ID     string
	Label  string
	JID    string
	Status SessionStatus
}

// UiAccountHandler is what a UI implements to host several accounts.
// ForAccount returns the UiMessageHandler a single account's SessionManager
// talks to: the JSON UI stamps the account id on every event, the tview UI
// drops what does not belong to the active account.
type UiAccountHandler interface {
	ForAccount(id string) UiMessageHandler
	// SetAccounts pushes the full account list (labels, numbers, state).
	SetAccounts(accounts []AccountInfo, active string)
	// SetActiveAccount tells the UI the active account changed; the new
	// account's chats/status/QR follow as ordinary events.
	SetActiveAccount(id string)
	// PrintAccounts shows the /contas listing. Plain text on purpose (see
	// AccountLines): the tview UI must escape the "[ONLINE]" tags itself.
	PrintAccounts(accounts []AccountInfo, active string)
}

// AccountManager runs one SessionManager per account (config/accounts.json)
// and routes UI commands to them. Its methods are called from the UI
// goroutine; status/QR updates arrive from the session goroutines, hence mu.
// accounts.json stays the single source of truth for labels and numbers.
type AccountManager struct {
	mu       sync.RWMutex
	managers map[string]*SessionManager
	handlers map[string]*accountHandler
	order    []string
	active   string
	status   map[string]SessionStatus
	lastQR   map[string]QRCode
	ui       UiAccountHandler
}

// accountHandler decorates the UI handler of one account: it records the
// account's last status and QR (so they can be replayed when the user switches
// to it) and otherwise passes everything through. The SessionManager itself
// never knows which account it is serving.
type accountHandler struct {
	UiMessageHandler
	am *AccountManager
	id string
}

func (h *accountHandler) SetStatus(status SessionStatus) {
	h.UiMessageHandler.SetStatus(status)
	if h.am.recordStatus(h.id, status) {
		h.am.publishAccounts()
	}
}

func (h *accountHandler) SetQRCode(qr QRCode) {
	h.am.mu.Lock()
	if _, ok := h.am.managers[h.id]; ok { // a removed account may still unwind
		h.am.lastQR[h.id] = qr
	}
	h.am.mu.Unlock()
	h.UiMessageHandler.SetQRCode(qr)
}

// NewAccountManager creates the managers for every registered account; call
// StartAll to connect them.
func NewAccountManager(ui UiAccountHandler) (*AccountManager, error) {
	reg, err := config.LoadAccounts()
	if err != nil {
		return nil, err
	}
	am := &AccountManager{
		managers: make(map[string]*SessionManager),
		handlers: make(map[string]*accountHandler),
		status:   make(map[string]SessionStatus),
		lastQR:   make(map[string]QRCode),
		ui:       ui,
		active:   reg.Active,
	}
	for _, acc := range reg.Accounts {
		am.addManager(acc.ID)
	}
	if _, ok := am.managers[am.active]; !ok && len(am.order) > 0 {
		am.active = am.order[0]
	}
	return am, nil
}

// addManager creates (but does not start) the SessionManager of an account.
func (am *AccountManager) addManager(id string) *SessionManager {
	h := &accountHandler{UiMessageHandler: am.ui.ForAccount(id), am: am, id: id}
	sm := &SessionManager{AccountID: id}
	sm.Init(h)
	sm.background = func() bool { return am.Active() != id }
	sm.notifyPrefix = func() string { return am.notifyPrefix(id) }
	sm.primaryAccount = func() bool {
		ids := am.ids()
		return len(ids) > 0 && ids[0] == id
	}
	am.mu.Lock()
	am.managers[id] = sm
	am.handlers[id] = h
	am.order = append(am.order, id)
	am.mu.Unlock()
	return sm
}

// StartAll starts every account's manager and announces the account list.
func (am *AccountManager) StartAll() error {
	am.ui.SetActiveAccount(am.Active())
	am.publishAccounts()
	var errs []error
	for _, id := range am.ids() {
		if err := am.session(id).StartManager(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", id, err))
		}
	}
	return errors.Join(errs...)
}

// Shutdown persists every cache and disconnects every account.
func (am *AccountManager) Shutdown() {
	for _, id := range am.ids() {
		sm := am.session(id)
		sm.FlushCache()
		sm.CommandChannel <- Command{"disconnect", nil}
	}
}

// Active returns the id of the active account.
func (am *AccountManager) Active() string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.active
}

// ActiveSession returns the manager of the active account.
func (am *AccountManager) ActiveSession() *SessionManager {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.managers[am.active]
}

func (am *AccountManager) session(id string) *SessionManager {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.managers[id]
}

func (am *AccountManager) ids() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return append([]string(nil), am.order...)
}

// activeHandler is where account-level feedback (lists, errors) is printed.
func (am *AccountManager) activeHandler() UiMessageHandler {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.handlers[am.active]
}

// recordStatus stores an account's status; false when the account is gone or
// nothing the account list shows has changed (battery-only updates).
func (am *AccountManager) recordStatus(id string, st SessionStatus) bool {
	am.mu.Lock()
	defer am.mu.Unlock()
	if _, ok := am.managers[id]; !ok {
		return false
	}
	prev, seen := am.status[id]
	am.status[id] = st
	return !seen || prev.Connected != st.Connected || prev.LoggedIn != st.LoggedIn ||
		prev.Connecting != st.Connecting || prev.NeedsLogin != st.NeedsLogin
}

// Accounts returns the account list in registry order.
func (am *AccountManager) Accounts() []AccountInfo {
	reg, err := config.LoadAccounts()
	am.mu.RLock()
	defer am.mu.RUnlock()
	list := make([]AccountInfo, 0, len(am.order))
	for _, id := range am.order {
		info := AccountInfo{ID: id, Label: id, Status: am.status[id]}
		if err == nil {
			if acc := reg.Find(id); acc != nil {
				info.Label = acc.Label
				info.JID = acc.JID
			}
		}
		list = append(list, info)
	}
	return list
}

// notifyPrefix labels desktop notifications with the account ("[Trabalho] ")
// once more than one account is connected; with a single one it stays empty.
func (am *AccountManager) notifyPrefix(id string) string {
	am.mu.RLock()
	connected := 0
	for _, st := range am.status {
		if st.Connected {
			connected++
		}
	}
	am.mu.RUnlock()
	if connected < 2 {
		return ""
	}
	for _, acc := range am.Accounts() {
		if acc.ID == id {
			return "[" + acc.Label + "] "
		}
	}
	return ""
}

func (am *AccountManager) publishAccounts() {
	am.ui.SetAccounts(am.Accounts(), am.Active())
}

// Send routes a command to an account (empty id = active account). The
// account commands themselves (/contas, /conta ...) are handled here.
func (am *AccountManager) Send(accountID string, cmd Command) {
	switch cmd.Name {
	case "contas", "accounts":
		am.printAccounts()
		return
	case "conta", "account":
		am.accountCommand(cmd.Params)
		return
	}
	if accountID == "" {
		accountID = am.Active()
	}
	sm := am.session(accountID)
	if sm == nil {
		am.activeHandler().PrintError(fmt.Errorf("conta desconhecida: %s", accountID))
		return
	}
	sm.CommandChannel <- cmd
}

// resolve accepts an account id or its 1-based position in the list.
func (am *AccountManager) resolve(ref string) (string, error) {
	ids := am.ids()
	if n, err := strconv.Atoi(ref); err == nil && n >= 1 && n <= len(ids) {
		return ids[n-1], nil
	}
	for _, id := range ids {
		if id == ref {
			return id, nil
		}
	}
	return "", fmt.Errorf("conta desconhecida: %s — veja %scontas", ref, config.Config.General.CmdPrefix)
}

func (am *AccountManager) accountCommand(params []string) {
	out := am.activeHandler()
	p := config.Config.General.CmdPrefix
	if len(params) == 0 {
		out.PrintText("uso: " + p + "conta <id|número> · " + p + "conta nova [nome] · " +
			p + "conta remover <id> · " + p + "conta renomear <id> <nome>")
		return
	}
	switch params[0] {
	case "nova", "new", "add":
		id, err := am.Add(strings.Join(params[1:], " "))
		if err != nil {
			out.PrintError(err)
			return
		}
		am.activeHandler().PrintText("conta nova criada: " + id + " — leia o QR code para parear")
	case "remover", "remove", "rm":
		if len(params) < 2 {
			out.PrintText("uso: " + p + "conta remover <id|número>")
			return
		}
		id, err := am.resolve(params[1])
		if err == nil {
			err = am.Remove(id)
		}
		out.PrintError(err)
	case "renomear", "rename":
		if len(params) < 3 {
			out.PrintText("uso: " + p + "conta renomear <id|número> <nome>")
			return
		}
		id, err := am.resolve(params[1])
		if err == nil {
			err = am.Rename(id, strings.Join(params[2:], " "))
		}
		out.PrintError(err)
	default:
		id, err := am.resolve(params[0])
		if err == nil {
			err = am.SetActive(id)
		}
		out.PrintError(err)
	}
}

func (am *AccountManager) printAccounts() {
	am.ui.PrintAccounts(am.Accounts(), am.Active())
}

// AccountLines renders the /contas listing as plain text, one account per
// line: "* 1 Pessoal (default) 5511… [ONLINE]", "*" marking the active one.
func AccountLines(accounts []AccountInfo, active string) []string {
	lines := []string{"contas:"}
	for i, acc := range accounts {
		mark := " "
		if acc.ID == active {
			mark = "*"
		}
		line := fmt.Sprintf("%s %d %s (%s)", mark, i+1, acc.Label, acc.ID)
		if acc.JID != "" {
			line += " " + strings.SplitN(acc.JID, "@", 2)[0]
		}
		lines = append(lines, line+" "+StatusTag(acc.Status))
	}
	return lines
}

// StatusTag is the "[ONLINE]"-style state of an account (raw brackets).
func StatusTag(st SessionStatus) string {
	switch {
	case st.Connected:
		return "[ONLINE]"
	case st.Connecting:
		return "[CONECTANDO]"
	case !st.LoggedIn:
		return "[SEM SESSÃO]"
	default:
		return "[OFFLINE]"
	}
}

// Add registers, starts and activates a new account; it has no session, so
// its manager goes straight to the QR code.
func (am *AccountManager) Add(label string) (string, error) {
	acc, err := config.AddAccount(label)
	if err != nil {
		return "", err
	}
	sm := am.addManager(acc.ID)
	if err := sm.StartManager(); err != nil {
		return "", err
	}
	return acc.ID, am.SetActive(acc.ID)
}

// Remove unpairs an account on the phone, stops its manager and deletes its
// folder. The slow part (logout over the network) runs in the background so
// the UI stays responsive.
func (am *AccountManager) Remove(id string) error {
	if err := config.RemoveAccount(id); err != nil {
		return err
	}
	am.mu.Lock()
	sm := am.managers[id]
	delete(am.managers, id)
	delete(am.handlers, id)
	delete(am.status, id)
	delete(am.lastQR, id)
	for i, o := range am.order {
		if o == id {
			am.order = append(am.order[:i], am.order[i+1:]...)
			break
		}
	}
	wasActive := am.active == id
	am.mu.Unlock()

	if wasActive {
		// the registry already moved "active" to the first remaining account
		am.SetActive(am.ids()[0])
	} else {
		am.publishAccounts()
	}
	out := am.activeHandler()
	go func() {
		sm.CommandChannel <- Command{"logout", nil}
		if err := sm.Stop(30 * time.Second); err != nil {
			out.PrintError(err)
			return
		}
		if err := config.RemoveAccountDir(id); err != nil {
			out.PrintError(err)
			return
		}
		out.PrintText("conta removida: " + id)
	}()
	return nil
}

// Rename changes an account's label.
func (am *AccountManager) Rename(id, label string) error {
	if err := config.RenameAccount(id, label); err != nil {
		return err
	}
	am.publishAccounts()
	return nil
}

// SetActive switches the active account: the UI is told first (so it can
// clear its state), then the account re-publishes chats/status and, if a
// pairing is in progress, its QR code.
func (am *AccountManager) SetActive(id string) error {
	am.mu.Lock()
	sm, ok := am.managers[id]
	if !ok {
		am.mu.Unlock()
		return fmt.Errorf("conta desconhecida: %s", id)
	}
	am.active = id
	h := am.handlers[id]
	qr, hasQR := am.lastQR[id]
	am.mu.Unlock()

	config.SetActiveAccount(id)
	am.ui.SetActiveAccount(id)
	am.publishAccounts()
	sm.CommandChannel <- Command{resyncCommand, nil}
	if hasQR && qr.Event == QRCodeShow {
		h.UiMessageHandler.SetQRCode(qr)
	}
	return nil
}
