package messages

import (
	"encoding/json"
	"os"
)

// cacheMessageLimit caps how many messages per chat are written to the local
// cache — enough to show recent history offline without bloating the file.
const cacheMessageLimit = 100

// cacheSchemaVersion lets future formats reject/ignore older caches.
const cacheSchemaVersion = 1

// cacheSnapshot is the on-disk shape of the lightweight local cache: the chat
// list plus the last N messages of each chat (status@broadcast included so
// stories survive a restart). RawMessage protos are dropped via the `json:"-"`
// tag on Message.
type cacheSnapshot struct {
	Version  int                  `json:"version"`
	Chats    []Chat               `json:"chats"`
	Messages map[string][]Message `json:"messages"`
}

// snapshot builds a serializable view of the database under read locks.
func (md *MessageDatabase) snapshot(limit int) cacheSnapshot {
	md.chatLock.RLock()
	chats := make([]Chat, 0, len(md.chats))
	for _, c := range md.chats {
		chats = append(chats, c)
	}
	md.chatLock.RUnlock()

	md.messageLock.RLock()
	msgs := make(map[string][]Message, len(md.messages))
	for id, list := range md.messages {
		if len(list) == 0 {
			continue
		}
		if len(list) > limit {
			list = list[len(list)-limit:]
		}
		cp := make([]Message, len(list))
		copy(cp, list)
		msgs[id] = cp
	}
	md.messageLock.RUnlock()

	return cacheSnapshot{Version: cacheSchemaVersion, Chats: chats, Messages: msgs}
}

// SaveCache writes a lightweight snapshot to path as JSON, atomically (write to
// a temp file then rename) so a crash mid-write never corrupts the cache.
func (md *MessageDatabase) SaveCache(path string) error {
	snap := md.snapshot(cacheMessageLimit)
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadCache reads a snapshot from path into the database. A missing file is not
// an error (first run). Chats are loaded before messages because AddMessage may
// also create chats, and AddChat's merge keeps the richer name/unread state.
func (md *MessageDatabase) LoadCache(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var snap cacheSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Version != cacheSchemaVersion {
		return nil // unknown/older format: ignore rather than crash
	}
	for _, c := range snap.Chats {
		md.AddChat(c)
	}
	for _, list := range snap.Messages {
		for _, m := range list {
			md.AddMessage(m, false)
		}
	}
	return nil
}
