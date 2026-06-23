package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/normen/whatscli/messages"
	"github.com/skratchdot/open-golang/open"
)

// JsonUiHandler is the headless UI used by the Ink frontend (ink-ui/): events
// go out as NDJSON on stdout and commands come back as NDJSON on stdin. Go
// stays the brain — session, storage, commands and audio playback all live
// here; the frontend only renders.
type JsonUiHandler struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// jsonUi is non-nil when running with --ui=json; shared helpers (audio
// callbacks) use it to route updates to the frontend instead of tview.
var jsonUi *JsonUiHandler

func (j *JsonUiHandler) emit(v map[string]any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.enc.Encode(v)
}

// jsonMessageDto strips RawMessage (huge proto) and exposes lowercase keys.
func jsonMessageDto(m *messages.Message) map[string]any {
	return map[string]any{
		"id":           m.Id,
		"chatId":       m.ChatId,
		"contactName":  m.ContactName,
		"contactShort": m.ContactShort,
		"timestamp":    m.Timestamp,
		"fromMe":       m.FromMe,
		"forwarded":    m.Forwarded,
		"text":         m.Text,
		"kind":         string(m.Kind),
		"durationSecs": m.DurationSecs,
	}
}

func jsonChatDto(c messages.Chat) map[string]any {
	return map[string]any{
		"id":          c.Id,
		"isGroup":     c.IsGroup,
		"name":        c.Name,
		"unread":      c.Unread,
		"lastMessage": c.LastMessage,
	}
}

func (j *JsonUiHandler) NewMessage(msg messages.Message) {
	j.emit(map[string]any{"type": "message", "message": jsonMessageDto(&msg)})
}

func (j *JsonUiHandler) NewScreen(msgs []messages.Message) {
	dtos := make([]map[string]any, 0, len(msgs))
	for i := range msgs {
		dtos = append(dtos, jsonMessageDto(&msgs[i]))
	}
	j.emit(map[string]any{"type": "screen", "messages": dtos})
}

func (j *JsonUiHandler) SetChats(chats []messages.Chat) {
	dtos := make([]map[string]any, 0, len(chats))
	for _, c := range chats {
		dtos = append(dtos, jsonChatDto(c))
	}
	j.emit(map[string]any{"type": "chats", "chats": dtos})
}

func jsonStoryDto(s messages.StatusUpdate) map[string]any {
	msgs := make([]map[string]any, 0, len(s.Messages))
	for i := range s.Messages {
		msgs = append(msgs, jsonMessageDto(&s.Messages[i]))
	}
	return map[string]any{
		"senderId":    s.SenderId,
		"name":        s.Name,
		"short":       s.Short,
		"unread":      s.Unread,
		"lastMessage": s.LastMessage,
		"messages":    msgs,
	}
}

func (j *JsonUiHandler) SetStories(stories []messages.StatusUpdate) {
	dtos := make([]map[string]any, 0, len(stories))
	for _, s := range stories {
		dtos = append(dtos, jsonStoryDto(s))
	}
	j.emit(map[string]any{"type": "stories", "stories": dtos})
}

func (j *JsonUiHandler) PrintError(err error) {
	if err == nil {
		return
	}
	j.emit(map[string]any{"type": "error", "text": err.Error()})
}

func (j *JsonUiHandler) PrintText(msg string) {
	j.emit(map[string]any{"type": "text", "text": msg})
}

func (j *JsonUiHandler) PrintFile(path string, msgId string) {
	j.emit(map[string]any{"type": "file", "path": path, "msgId": msgId})
}

func (j *JsonUiHandler) OpenFile(path string) {
	open.Run(path) // the brain opens files itself; the frontend only renders
}

func (j *JsonUiHandler) PlayFile(path string, msgId string) {
	playAudioFile(path, msgId) // audio playback stays in Go
}

func (j *JsonUiHandler) SetStatus(status messages.SessionStatus) {
	j.emit(map[string]any{
		"type":      "status",
		"connected": status.Connected,
		"lastSeen":  status.LastSeen,
	})
}

// GetWriter adapts free-form writes (QR code login, shutdown notices) into
// one "text" event per line.
func (j *JsonUiHandler) GetWriter() io.Writer {
	return &jsonLineWriter{j: j}
}

type jsonLineWriter struct {
	j   *JsonUiHandler
	mu  sync.Mutex
	buf strings.Builder
}

func (w *jsonLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, b := range p {
		if b == '\n' {
			w.j.emit(map[string]any{"type": "text", "text": w.buf.String()})
			w.buf.Reset()
		} else {
			w.buf.WriteByte(b)
		}
	}
	return len(p), nil
}

// runJsonUi is the --ui=json entrypoint: starts the session manager headless
// and pumps stdin commands into the CommandChannel until the frontend closes.
func runJsonUi() {
	jsonUi = &JsonUiHandler{enc: json.NewEncoder(os.Stdout)}
	uiHandler = jsonUi
	sessionManager = &messages.SessionManager{}
	sessionManager.Init(uiHandler)
	if err := sessionManager.StartManager(); err != nil {
		jsonUi.PrintError(err)
	}
	jsonUi.emit(map[string]any{"type": "ready", "version": VERSION})

	dec := json.NewDecoder(os.Stdin)
	for {
		var cmd struct {
			Cmd    string   `json:"cmd"`
			Params []string `json:"params"`
		}
		if err := dec.Decode(&cmd); err != nil {
			break // EOF/parse failure: frontend went away
		}
		if cmd.Cmd == "" {
			continue
		}
		if cmd.Cmd == "quit" {
			break
		}
		sessionManager.CommandChannel <- messages.Command{cmd.Cmd, cmd.Params}
	}

	stopAudio()
	sessionManager.FlushCache() // persist the local cache before exiting
	sessionManager.CommandChannel <- messages.Command{"disconnect", nil}
	// give the manager a moment to flush the disconnect before exiting
	time.Sleep(200 * time.Millisecond)
}
