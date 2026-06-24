package messages

import (
	"os"
	"path/filepath"
	"testing"
)

func sampleDB(t *testing.T) *MessageDatabase {
	t.Helper()
	db := &MessageDatabase{}
	db.Init()
	db.AddMessage(Message{
		Id:          "m1",
		ChatId:      "123@s.whatsapp.net",
		ContactId:   "123@s.whatsapp.net",
		ContactName: "Alice",
		Timestamp:   100,
		Text:        "oi",
		Kind:        MessageKindText,
	}, false)
	db.AddMessage(Message{
		Id:          "m2",
		ChatId:      "123@s.whatsapp.net",
		ContactId:   "123@s.whatsapp.net",
		ContactName: "Alice",
		Timestamp:   200,
		Text:        "tudo bem?",
		Kind:        MessageKindText,
	}, true)
	return db
}

func TestSaveAndLoadCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	db := sampleDB(t)

	if err := db.SaveCache(path); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}

	fresh := &MessageDatabase{}
	fresh.Init()
	if err := fresh.LoadCache(path); err != nil {
		t.Fatalf("LoadCache: %v", err)
	}

	chats := fresh.GetChatIds()
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat after load, got %d", len(chats))
	}
	if chats[0].Name != "Alice" {
		t.Fatalf("expected chat name Alice, got %q", chats[0].Name)
	}

	msgs := fresh.GetMessages("123@s.whatsapp.net")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after load, got %d", len(msgs))
	}
	if msgs[0].Id != "m1" || msgs[1].Id != "m2" {
		t.Fatalf("messages not preserved/sorted: %#v", msgs)
	}
}

func TestLoadCacheMissingFileIsNotError(t *testing.T) {
	db := &MessageDatabase{}
	db.Init()
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	if err := db.LoadCache(path); err != nil {
		t.Fatalf("expected nil for missing cache, got %v", err)
	}
}

func TestLoadCacheIgnoresUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte(`{"version":999,"chats":[{"Id":"x","Name":"X"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	db := &MessageDatabase{}
	db.Init()
	if err := db.LoadCache(path); err != nil {
		t.Fatalf("expected nil for unknown version, got %v", err)
	}
	if len(db.GetChatIds()) != 0 {
		t.Fatal("unknown-version cache should be ignored, not loaded")
	}
}

func TestSnapshotCapsMessagesPerChat(t *testing.T) {
	db := &MessageDatabase{}
	db.Init()
	for i := 0; i < cacheMessageLimit+50; i++ {
		db.AddMessage(Message{
			Id:        string(rune('a'+i%26)) + string(rune('a'+i/26)),
			ChatId:    "c@s.whatsapp.net",
			Timestamp: uint64(i + 1),
			Text:      "x",
			Kind:      MessageKindText,
		}, false)
	}
	snap := db.snapshot(cacheMessageLimit)
	if got := len(snap.Messages["c@s.whatsapp.net"]); got != cacheMessageLimit {
		t.Fatalf("expected snapshot capped at %d, got %d", cacheMessageLimit, got)
	}
}
