package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/adrg/xdg"
)

// DefaultAccountID is the account every install starts with; the pre-multi-
// account session is migrated into it.
const DefaultAccountID = "default"

// Account is one paired WhatsApp number. ID names the folder under
// accounts/, Label is user-editable and JID is filled in after pairing.
type Account struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	JID   string `json:"jid,omitempty"`
}

// AccountRegistry is the content of accounts.json.
type AccountRegistry struct {
	Active   string    `json:"active"`
	Accounts []Account `json:"accounts"`
}

// Find returns the account with the given id, or nil.
func (r *AccountRegistry) Find(id string) *Account {
	for i := range r.Accounts {
		if r.Accounts[i].ID == id {
			return &r.Accounts[i]
		}
	}
	return nil
}

// accountsLock serializes read-modify-write cycles of accounts.json: several
// session managers (one per account) may record their JID concurrently.
var accountsLock sync.Mutex

// baseDir is ~/.config/whatscli; a var so tests can point it at a temp dir.
var baseDir = func() string {
	return filepath.Join(xdg.ConfigHome, "whatscli")
}

func accountsFilePath() string {
	return filepath.Join(baseDir(), "accounts.json")
}

// AccountDir returns (and creates) ~/.config/whatscli/accounts/<id>/.
func AccountDir(id string) string {
	dir := filepath.Join(baseDir(), "accounts", id)
	os.MkdirAll(dir, 0o700)
	return dir
}

// GetSessionFilePathFor is the whatsmeow SQLite path of an account, without
// the ".db" suffix (callers append it, as with the old single session).
func GetSessionFilePathFor(id string) string {
	return filepath.Join(AccountDir(id), "session")
}

// GetCacheFilePathFor is the local conversation cache of an account.
func GetCacheFilePathFor(id string) string {
	return filepath.Join(AccountDir(id), "cache.json")
}

// GetQRFilePathFor is where the login QR image of an account is saved.
func GetQRFilePathFor(id string) string {
	return filepath.Join(AccountDir(id), "whatscli-qr.png")
}

// LoadAccounts reads accounts.json.
func LoadAccounts() (*AccountRegistry, error) {
	accountsLock.Lock()
	defer accountsLock.Unlock()
	return loadAccounts()
}

func loadAccounts() (*AccountRegistry, error) {
	data, err := os.ReadFile(accountsFilePath())
	if err != nil {
		return nil, err
	}
	reg := &AccountRegistry{}
	if err := json.Unmarshal(data, reg); err != nil {
		return nil, fmt.Errorf("accounts.json inválido: %v", err)
	}
	return reg, nil
}

// SaveAccounts writes accounts.json atomically (temp file + rename), so a
// crash mid-write never leaves a truncated registry behind.
func SaveAccounts(reg *AccountRegistry) error {
	accountsLock.Lock()
	defer accountsLock.Unlock()
	return saveAccounts(reg)
}

func saveAccounts(reg *AccountRegistry) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	path := accountsFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SetAccountJID records the paired number of an account (no-op when it is
// unchanged or the account is unknown).
func SetAccountJID(id, jid string) error {
	accountsLock.Lock()
	defer accountsLock.Unlock()
	reg, err := loadAccounts()
	if err != nil {
		return err
	}
	acc := reg.Find(id)
	if acc == nil || acc.JID == jid {
		return nil
	}
	acc.JID = jid
	return saveAccounts(reg)
}

// InitAccounts loads the account registry, creating it on first run. When the
// registry does not exist yet, the single-account files of older versions
// (session.db, cache.json in ~/.config/whatscli/) are moved into
// accounts/default/, so upgrading never asks for a new QR code.
func InitAccounts() (*AccountRegistry, error) {
	accountsLock.Lock()
	defer accountsLock.Unlock()
	reg, err := loadAccounts()
	if err == nil {
		if len(reg.Accounts) == 0 {
			reg.Accounts = []Account{{ID: DefaultAccountID, Label: "Pessoal"}}
		}
		if reg.Find(reg.Active) == nil {
			reg.Active = reg.Accounts[0].ID
		}
		return reg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := migrateLegacyFiles(); err != nil {
		return nil, fmt.Errorf("falha ao migrar a sessão antiga: %v", err)
	}
	reg = &AccountRegistry{
		Active:   DefaultAccountID,
		Accounts: []Account{{ID: DefaultAccountID, Label: "Pessoal"}},
	}
	return reg, saveAccounts(reg)
}

// migrateLegacyFiles moves the pre-multi-account files into the default
// account folder. SQLite journal files (-wal/-shm) move along with the db so
// uncheckpointed writes are not lost. Existing targets are never overwritten,
// which keeps a half-finished migration safe to re-run.
func migrateLegacyFiles() error {
	base := baseDir()
	dst := AccountDir(DefaultAccountID)
	moves := []struct{ from, to string }{
		{"session.db", "session.db"},
		{"session.db-wal", "session.db-wal"},
		{"session.db-shm", "session.db-shm"},
		{"session.db-journal", "session.db-journal"},
		{"cache.json", "cache.json"},
	}
	for _, m := range moves {
		from := filepath.Join(base, m.from)
		to := filepath.Join(dst, m.to)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if _, err := os.Stat(to); err == nil {
			continue
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	// the old QR image is stale by definition
	os.Remove(filepath.Join(base, "whatscli-qr.png"))
	return nil
}

// MaxAccounts caps how many accounts run at once: each one is a websocket, a
// goroutine and a full in-memory MessageDatabase fed by history sync.
const MaxAccounts = 5

// AddAccount registers a new account with a unique id derived from the label.
// The id also avoids folders still on disk (an account being removed), so a
// new account never inherits or loses someone else's files.
func AddAccount(label string) (Account, error) {
	accountsLock.Lock()
	defer accountsLock.Unlock()
	reg, err := loadAccounts()
	if err != nil {
		return Account{}, err
	}
	if len(reg.Accounts) >= MaxAccounts {
		return Account{}, fmt.Errorf("limite de %d contas atingido", MaxAccounts)
	}
	if label == "" {
		label = fmt.Sprintf("Conta %d", len(reg.Accounts)+1)
	}
	base := slugify(label)
	if base == "" {
		base = "conta"
	}
	id := base
	for n := 2; reg.Find(id) != nil || dirExists(filepath.Join(baseDir(), "accounts", id)); n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	acc := Account{ID: id, Label: label}
	reg.Accounts = append(reg.Accounts, acc)
	return acc, saveAccounts(reg)
}

// RemoveAccount drops an account from the registry (its folder is deleted by
// the caller once its session manager has stopped). The last account cannot
// be removed; when the active one goes, the first remaining becomes active.
func RemoveAccount(id string) error {
	accountsLock.Lock()
	defer accountsLock.Unlock()
	reg, err := loadAccounts()
	if err != nil {
		return err
	}
	if reg.Find(id) == nil {
		return fmt.Errorf("conta desconhecida: %s", id)
	}
	if len(reg.Accounts) == 1 {
		return errors.New("não é possível remover a única conta")
	}
	kept := reg.Accounts[:0]
	for _, a := range reg.Accounts {
		if a.ID != id {
			kept = append(kept, a)
		}
	}
	reg.Accounts = kept
	if reg.Active == id {
		reg.Active = kept[0].ID
	}
	return saveAccounts(reg)
}

// RenameAccount changes the user-facing label of an account.
func RenameAccount(id, label string) error {
	accountsLock.Lock()
	defer accountsLock.Unlock()
	reg, err := loadAccounts()
	if err != nil {
		return err
	}
	acc := reg.Find(id)
	if acc == nil {
		return fmt.Errorf("conta desconhecida: %s", id)
	}
	if label == "" {
		return errors.New("o nome da conta não pode ser vazio")
	}
	acc.Label = label
	return saveAccounts(reg)
}

// SetActiveAccount persists which account opens by default.
func SetActiveAccount(id string) error {
	accountsLock.Lock()
	defer accountsLock.Unlock()
	reg, err := loadAccounts()
	if err != nil {
		return err
	}
	if reg.Find(id) == nil {
		return fmt.Errorf("conta desconhecida: %s", id)
	}
	reg.Active = id
	return saveAccounts(reg)
}

// RemoveAccountDir deletes the folder of an account (session, cache, QR).
func RemoveAccountDir(id string) error {
	if id == "" {
		return errors.New("id de conta vazio")
	}
	return os.RemoveAll(filepath.Join(baseDir(), "accounts", id))
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// slugify turns a label into a short folder-safe id: lowercase ASCII letters
// and digits, accents folded, anything else collapsed into single dashes.
func slugify(label string) string {
	fold := map[rune]rune{
		'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
		'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
		'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
		'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
		'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
		'ç': 'c', 'ñ': 'n',
	}
	out := make([]rune, 0, 16)
	dash := false
	for _, r := range strings.ToLower(label) {
		if f, ok := fold[r]; ok {
			r = f
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, r)
			dash = false
		} else if len(out) > 0 && !dash {
			out = append(out, '-')
			dash = true
		}
		if len(out) >= 16 {
			break
		}
	}
	return strings.Trim(string(out), "-")
}
