package messages

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/normen/whatscli/config"
)

// nopHandler is a UiMessageHandler that records printed text.
type nopHandler struct {
	mu    sync.Mutex
	texts []string
}

func (h *nopHandler) NewMessage(Message)        {}
func (h *nopHandler) NewScreen([]Message)       {}
func (h *nopHandler) SetChats([]Chat)           {}
func (h *nopHandler) PrintError(err error)      { h.PrintText("ERR " + errString(err)) }
func (h *nopHandler) PrintFile(string, string)  {}
func (h *nopHandler) PlayFile(string, string)   {}
func (h *nopHandler) SetStatus(SessionStatus)   {}
func (h *nopHandler) SetQRCode(QRCode)          {}
func (h *nopHandler) SetStories([]StatusUpdate) {}
func (h *nopHandler) OpenFile(string)           {}
func (h *nopHandler) GetWriter() io.Writer      { return io.Discard }
func (h *nopHandler) PrintText(s string)        { h.mu.Lock(); h.texts = append(h.texts, s); h.mu.Unlock() }
func (h *nopHandler) lastText() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.texts[len(h.texts)-1]
}
func (h *nopHandler) hasText(want string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.texts {
		if s == want {
			return true
		}
	}
	return false
}
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// fakeAccountsUi records what the AccountManager pushes to the UI.
type fakeAccountsUi struct {
	handlers map[string]*nopHandler
	active   []string
	listed   [][]AccountInfo
}

func (u *fakeAccountsUi) ForAccount(id string) UiMessageHandler {
	h := &nopHandler{}
	u.handlers[id] = h
	return h
}
func (u *fakeAccountsUi) SetAccounts(list []AccountInfo, _ string) { u.listed = append(u.listed, list) }
func (u *fakeAccountsUi) SetActiveAccount(id string)               { u.active = append(u.active, id) }
func (u *fakeAccountsUi) PrintAccounts(list []AccountInfo, a string) {
	u.listed = append(u.listed, list)
}

// newTestAccounts builds an AccountManager over a temp config dir with two
// accounts. Managers are never started, so nothing touches the network and
// routed commands just sit in their CommandChannel.
func newTestAccounts(t *testing.T) (*AccountManager, *fakeAccountsUi, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	xdg.Reload()
	t.Cleanup(xdg.Reload)
	if err := config.SaveAccounts(&config.AccountRegistry{Active: "default", Accounts: []config.Account{
		{ID: "default", Label: "Pessoal"},
		{ID: "trabalho", Label: "Trabalho"},
	}}); err != nil {
		t.Fatal(err)
	}
	ui := &fakeAccountsUi{handlers: map[string]*nopHandler{}}
	am, err := NewAccountManager(ui)
	if err != nil {
		t.Fatal(err)
	}
	return am, ui, filepath.Join(dir, "whatscli")
}

func nextCommand(t *testing.T, sm *SessionManager) Command {
	t.Helper()
	select {
	case c := <-sm.CommandChannel:
		return c
	default:
		t.Fatal("no command routed to this account")
		return Command{}
	}
}

func TestAccountManagerRoutesCommands(t *testing.T) {
	am, _, _ := newTestAccounts(t)
	am.Send("", Command{"select", []string{"a@s.whatsapp.net"}})
	if c := nextCommand(t, am.session("default")); c.Name != "select" {
		t.Fatalf("active account got %v", c)
	}
	am.Send("trabalho", Command{"novoqr", nil})
	if c := nextCommand(t, am.session("trabalho")); c.Name != "novoqr" {
		t.Fatalf("explicit account got %v", c)
	}
	if len(am.session("default").CommandChannel) != 0 {
		t.Fatal("command with account leaked into the active account")
	}
}

func TestAccountManagerSwitchByNumber(t *testing.T) {
	am, ui, _ := newTestAccounts(t)
	am.Send("", Command{"conta", []string{"2"}})
	if am.Active() != "trabalho" {
		t.Fatalf("active = %q", am.Active())
	}
	if ui.active[len(ui.active)-1] != "trabalho" {
		t.Fatalf("UI not told about the switch: %v", ui.active)
	}
	if c := nextCommand(t, am.session("trabalho")); c.Name != resyncCommand {
		t.Fatalf("new active account not resynced: %v", c)
	}
	if reg, _ := config.LoadAccounts(); reg.Active != "trabalho" {
		t.Fatalf("active account not persisted: %+v", reg)
	}
	am.Send("", Command{"conta", []string{"nope"}})
	if !strings.Contains(ui.handlers["trabalho"].lastText(), "conta desconhecida") {
		t.Fatalf("unknown account not reported: %q", ui.handlers["trabalho"].lastText())
	}
}

func TestAccountManagerRename(t *testing.T) {
	am, _, _ := newTestAccounts(t)
	am.Send("", Command{"conta", []string{"renomear", "trabalho", "Loja", "Centro"}})
	if got := am.Accounts()[1].Label; got != "Loja Centro" {
		t.Fatalf("label = %q", got)
	}
}

func TestAccountManagerRemove(t *testing.T) {
	am, ui, base := newTestAccounts(t)
	accDir := config.AccountDir("default")
	if err := am.Remove("default"); err != nil {
		t.Fatal(err)
	}
	if am.Active() != "trabalho" || len(am.ids()) != 1 {
		t.Fatalf("after remove: active=%q ids=%v", am.Active(), am.ids())
	}
	// the folder goes away in the background once the manager has stopped
	for i := 0; ; i++ {
		if ui.handlers["trabalho"].hasText("conta removida: default") {
			break
		}
		if i == 100 {
			t.Fatal("background removal did not finish")
		}
		sleepTick()
	}
	if _, err := os.Stat(accDir); !os.IsNotExist(err) {
		t.Fatalf("account folder not removed: %s", accDir)
	}
	if err := am.Remove("trabalho"); err == nil {
		t.Fatal("removing the last account must fail")
	}
	if _, err := os.Stat(filepath.Join(base, "accounts.json")); err != nil {
		t.Fatal(err)
	}
}

func TestAccountLines(t *testing.T) {
	lines := AccountLines([]AccountInfo{
		{ID: "default", Label: "Pessoal", JID: "5511@s.whatsapp.net", Status: SessionStatus{Connected: true, LoggedIn: true}},
		{ID: "loja", Label: "Loja"},
	}, "default")
	if lines[1] != "* 1 Pessoal (default) 5511 [ONLINE]" || lines[2] != "  2 Loja (loja) [SEM SESSÃO]" {
		t.Fatalf("unexpected listing: %q", lines)
	}
}

func sleepTick() { time.Sleep(20 * time.Millisecond) }
