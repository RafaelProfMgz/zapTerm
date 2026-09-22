package config

import (
	"os"
	"path/filepath"
	"testing"
)

func useTempBase(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := baseDir
	baseDir = func() string { return dir }
	t.Cleanup(func() { baseDir = prev })
	return dir
}

func TestInitAccountsMigratesLegacySession(t *testing.T) {
	base := useTempBase(t)
	for name, content := range map[string]string{
		"session.db":      "db",
		"session.db-wal":  "wal",
		"cache.json":      "{}",
		"whatscli-qr.png": "png",
	} {
		if err := os.WriteFile(filepath.Join(base, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	reg, err := InitAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Active != DefaultAccountID || len(reg.Accounts) != 1 || reg.Accounts[0].Label != "Pessoal" {
		t.Fatalf("unexpected registry: %+v", reg)
	}

	accDir := filepath.Join(base, "accounts", DefaultAccountID)
	for name, want := range map[string]string{"session.db": "db", "session.db-wal": "wal", "cache.json": "{}"} {
		got, err := os.ReadFile(filepath.Join(accDir, name))
		if err != nil || string(got) != want {
			t.Errorf("%s not migrated: %q, %v", name, got, err)
		}
		if _, err := os.Stat(filepath.Join(base, name)); !os.IsNotExist(err) {
			t.Errorf("legacy %s still in place", name)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "whatscli-qr.png")); !os.IsNotExist(err) {
		t.Error("stale QR image not removed")
	}
	if GetSessionFilePathFor(DefaultAccountID)+".db" != filepath.Join(accDir, "session.db") {
		t.Error("session path does not point at the migrated db")
	}

	// second run: registry exists, nothing moves again
	if err := os.WriteFile(filepath.Join(base, "session.db"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InitAccounts(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(accDir, "session.db")); string(got) != "db" {
		t.Errorf("migrated db overwritten on second run: %q", got)
	}
}

func TestInitAccountsFreshInstall(t *testing.T) {
	base := useTempBase(t)
	reg, err := InitAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Active != DefaultAccountID {
		t.Fatalf("active = %q", reg.Active)
	}
	if _, err := os.Stat(filepath.Join(base, "accounts.json")); err != nil {
		t.Fatalf("registry not written: %v", err)
	}
}

func TestSetAccountJID(t *testing.T) {
	useTempBase(t)
	if _, err := InitAccounts(); err != nil {
		t.Fatal(err)
	}
	if err := SetAccountJID(DefaultAccountID, "5511999999999@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	reg, err := LoadAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Find(DefaultAccountID).JID != "5511999999999@s.whatsapp.net" {
		t.Fatalf("jid not saved: %+v", reg)
	}
	if err := SetAccountJID("missing", "x"); err != nil {
		t.Fatalf("unknown account should be a no-op, got %v", err)
	}
}

func TestInitAccountsFixesDanglingActive(t *testing.T) {
	useTempBase(t)
	if err := SaveAccounts(&AccountRegistry{Active: "gone", Accounts: []Account{{ID: "work", Label: "Trabalho"}}}); err != nil {
		t.Fatal(err)
	}
	reg, err := InitAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Active != "work" {
		t.Fatalf("active = %q, want work", reg.Active)
	}
}
