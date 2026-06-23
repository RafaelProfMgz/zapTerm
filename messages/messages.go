// this package manages the messages
package messages

import (
	"io"

	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// TODO: move these funcs/interface to channels
type UiMessageHandler interface {
	NewMessage(Message)
	NewScreen([]Message)
	SetChats([]Chat)
	PrintError(error)
	PrintText(string)
	// PrintFile displays a downloaded image in the message panel; msgId ties the
	// rendered image to its message (empty when there is no originating message).
	PrintFile(path string, msgId string)
	// PlayFile plays a downloaded audio file; msgId ties the playback state to
	// its message so playing the same message again stops it (toggle).
	PlayFile(path string, msgId string)
	SetStatus(SessionStatus)
	// SetStories pushes the grouped status@broadcast feed (stories), kept
	// separate from the conversation list.
	SetStories([]StatusUpdate)
	OpenFile(string)
	GetWriter() io.Writer
}

// data struct for current session status
type SessionStatus struct {
	BatteryCharge    int
	BatteryLoading   bool
	BatteryPowersave bool
	Connected        bool
	LastSeen         string
}

// message struct for battery messages
type BatteryMsg struct {
	charge    int
	loading   bool
	powersave bool
}

// message struct for status messages
type StatusMsg struct {
	connected bool
	err       error
}

// message object for commands
type Command struct {
	Name   string
	Params []string
}

type MessageKind string

const (
	MessageKindText     MessageKind = "text"
	MessageKindImage    MessageKind = "image"
	MessageKindVideo    MessageKind = "video"
	MessageKindAudio    MessageKind = "audio"
	MessageKindDocument MessageKind = "document"
	MessageKindUnknown  MessageKind = "unknown"
)

// internal message representation to abstract from message lib
type Message struct {
	Id           string
	ChatId       string // the source of the message (group id or contact id)
	SenderId     string
	ContactId    string
	ContactName  string
	ContactShort string
	Timestamp    uint64
	FromMe       bool
	Forwarded    bool
	Text         string
	Kind         MessageKind
	MimeType     string
	FileName     string
	// DurationSecs is the media length in seconds (audio/video), 0 if unknown.
	DurationSecs uint32
	Unread       bool
	// RawMessage is the original proto; it is never persisted to the local cache
	// (huge, and re-download only works while online anyway).
	RawMessage *waProto.Message `json:"-"`
}

// StatusUpdate groups one contact's status posts (stories). status@broadcast
// messages are kept out of the normal chat list and surfaced here instead.
type StatusUpdate struct {
	SenderId    string
	Name        string
	Short       string
	Unread      int
	LastMessage int64
	Messages    []Message
}

// internal contact representation to abstract from message lib
type Chat struct {
	Id      string
	IsGroup bool
	Name    string
	Unread  int
	//TODO: convert to uint64
	LastMessage int64
}

type Contact struct {
	Id    string
	Name  string
	Short string
}

const GROUPSUFFIX = "@g.us"
const CONTACTSUFFIX = "@s.whatsapp.net"
const STATUSSUFFIX = "status@broadcast"
