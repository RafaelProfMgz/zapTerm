package messages

import "testing"

func TestAddMessageAndMarkChatRead(t *testing.T) {
	db := &MessageDatabase{}
	db.Init()

	first := Message{
		Id:           "msg-1",
		ChatId:       "123@s.whatsapp.net",
		ContactId:    "123@s.whatsapp.net",
		ContactName:  "Alice",
		ContactShort: "Alice",
		Timestamp:    100,
		Text:         "hello",
		Kind:         MessageKindText,
	}
	second := Message{
		Id:           "msg-2",
		ChatId:       "123@s.whatsapp.net",
		ContactId:    "123@s.whatsapp.net",
		ContactName:  "Alice",
		ContactShort: "Alice",
		Timestamp:    101,
		Text:         "[IMAGE]",
		Kind:         MessageKindImage,
	}

	if !db.AddMessage(first, false) {
		t.Fatal("expected first message to be new")
	}
	if !db.AddMessage(second, true) {
		t.Fatal("expected second message to be new")
	}

	msgs := db.GetMessages(first.ChatId)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Id != "msg-1" || msgs[1].Id != "msg-2" {
		t.Fatalf("messages not sorted by timestamp: %#v", msgs)
	}

	chats := db.GetChatIds()
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}
	if chats[0].Unread != 1 {
		t.Fatalf("expected unread count 1, got %d", chats[0].Unread)
	}

	cleared := db.MarkChatRead(first.ChatId)
	if len(cleared) != 1 || cleared[0].Id != "msg-2" {
		t.Fatalf("expected msg-2 to be cleared, got %#v", cleared)
	}

	chats = db.GetChatIds()
	if chats[0].Unread != 0 {
		t.Fatalf("expected unread count 0 after mark read, got %d", chats[0].Unread)
	}
}

func TestUpdateChatUnreadMarksLatestIncomingMessages(t *testing.T) {
	db := &MessageDatabase{}
	db.Init()

	for idx := 0; idx < 4; idx++ {
		db.AddMessage(Message{
			Id:           string(rune('a' + idx)),
			ChatId:       "group@g.us",
			SenderId:     "user@s.whatsapp.net",
			ContactId:    "user@s.whatsapp.net",
			ContactName:  "User",
			ContactShort: "User",
			Timestamp:    uint64(idx + 1),
			FromMe:       idx == 0,
			Text:         "payload",
			Kind:         MessageKindText,
		}, false)
	}

	db.UpdateChatUnread("group@g.us", 2)

	msgs := db.GetMessages("group@g.us")
	unread := 0
	for _, msg := range msgs {
		if msg.Unread {
			unread++
		}
	}
	if unread != 2 {
		t.Fatalf("expected 2 unread messages, got %d", unread)
	}
}

func addStatus(db *MessageDatabase, id, sender, name string, ts uint64, unread bool) {
	db.AddMessage(Message{
		Id:           id,
		ChatId:       STATUSSUFFIX,
		SenderId:     sender,
		ContactId:    sender,
		ContactName:  name,
		ContactShort: name,
		Timestamp:    ts,
		Text:         "status",
		Kind:         MessageKindImage,
	}, unread)
}

func TestStatusBroadcastExcludedFromChatList(t *testing.T) {
	db := &MessageDatabase{}
	db.Init()

	db.AddMessage(Message{
		Id:          "real-1",
		ChatId:      "123@s.whatsapp.net",
		ContactId:   "123@s.whatsapp.net",
		ContactName: "Alice",
		Timestamp:   100,
		Text:        "oi",
		Kind:        MessageKindText,
	}, false)
	addStatus(db, "st-1", "555@s.whatsapp.net", "Bob", 200, false)

	chats := db.GetChatIds()
	if len(chats) != 1 {
		t.Fatalf("expected only the real chat, got %d: %#v", len(chats), chats)
	}
	if chats[0].Id != "123@s.whatsapp.net" {
		t.Fatalf("status@broadcast leaked into chat list: %#v", chats)
	}
}

func TestGetStatusUpdatesGroupsBySender(t *testing.T) {
	db := &MessageDatabase{}
	db.Init()

	addStatus(db, "a1", "111@s.whatsapp.net", "Alice", 100, true)
	addStatus(db, "a2", "111@s.whatsapp.net", "Alice", 300, false)
	addStatus(db, "b1", "222@s.whatsapp.net", "Bob", 200, false)

	stories := db.GetStatusUpdates()
	if len(stories) != 2 {
		t.Fatalf("expected 2 senders, got %d", len(stories))
	}
	// most recent sender first: Alice (last 300) before Bob (200)
	if stories[0].SenderId != "111@s.whatsapp.net" {
		t.Fatalf("expected Alice first (most recent), got %q", stories[0].SenderId)
	}
	if len(stories[0].Messages) != 2 {
		t.Fatalf("expected Alice to have 2 status posts, got %d", len(stories[0].Messages))
	}
	if stories[0].Unread != 1 {
		t.Fatalf("expected Alice unread 1, got %d", stories[0].Unread)
	}
	if stories[0].LastMessage != 300 {
		t.Fatalf("expected Alice lastMessage 300, got %d", stories[0].LastMessage)
	}
	if stories[1].SenderId != "222@s.whatsapp.net" {
		t.Fatalf("expected Bob second, got %q", stories[1].SenderId)
	}
}

func TestGetStatusUpdatesEmpty(t *testing.T) {
	db := &MessageDatabase{}
	db.Init()
	if got := db.GetStatusUpdates(); len(got) != 0 {
		t.Fatalf("expected no stories, got %d", len(got))
	}
}
