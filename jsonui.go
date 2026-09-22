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
//
// The root handler (jsonUi) also implements messages.UiAccountHandler: each
// account gets a child handler from ForAccount that stamps "account" on every
// event and writes through the root's encoder.
type JsonUiHandler struct {
	mu      sync.Mutex
	enc     *json.Encoder
	parent  *JsonUiHandler // nil on the root handler
	account string
}

// jsonUi is non-nil when running with --ui=json; shared helpers (audio
// callbacks) use it to route updates to the frontend instead of tview.
var jsonUi *JsonUiHandler

func (j *JsonUiHandler) emit(v map[string]any) {
	if j.parent != nil {
		v["account"] = j.account
		j.parent.emit(v)
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.enc.Encode(v)
}

// ForAccount returns the handler of one account's SessionManager.
func (j *JsonUiHandler) ForAccount(id string) messages.UiMessageHandler {
	return &JsonUiHandler{parent: j, account: id}
}

func jsonAccountDto(a messages.AccountInfo) map[string]any {
	return map[string]any{
		"id":         a.ID,
		"label":      a.Label,
		"jid":        a.JID,
		"connected":  a.Status.Connected,
		"loggedIn":   a.Status.LoggedIn,
		"connecting": a.Status.Connecting,
		"needsLogin": a.Status.NeedsLogin,
	}
}

// SetAccounts emits the full account list: {"type":"accounts","active":id,...}.
func (j *JsonUiHandler) SetAccounts(accounts []messages.AccountInfo, active string) {
	dtos := make([]map[string]any, 0, len(accounts))
	for _, a := range accounts {
		dtos = append(dtos, jsonAccountDto(a))
	}
	j.emit(map[string]any{"type": "accounts", "active": active, "accounts": dtos})
}

// SetActiveAccount emits {"type":"account","id":...}: the frontend drops the
// state of the previous account; the new one's chats/status follow.
func (j *JsonUiHandler) SetActiveAccount(id string) {
	j.emit(map[string]any{"type": "account", "id": id})
}

func (j *JsonUiHandler) PrintAccounts(accounts []messages.AccountInfo, active string) {
	for _, line := range messages.AccountLines(accounts, active) {
		j.PrintText(line)
	}
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
		"type":       "status",
		"connected":  status.Connected,
		"lastSeen":   status.LastSeen,
		"loggedIn":   status.LoggedIn,
		"connecting": status.Connecting,
		"needsLogin": status.NeedsLogin,
	})
}

// SetQRCode forwards the login QR state; the frontend draws the matrix itself
// (one string per row, '1' = dark module) so the code keeps its quiet zone and
// stays scannable.
func (j *JsonUiHandler) SetQRCode(qr messages.QRCode) {
	j.emit(map[string]any{
		"type":    "qr",
		"event":   string(qr.Event),
		"code":    qr.Code,
		"matrix":  qr.Matrix,
		"png":     qr.PngPath,
		"message": qr.Message,
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
	messages.Headless = true // bell goes to stderr; stdout is the NDJSON protocol
	jsonUi = &JsonUiHandler{enc: json.NewEncoder(os.Stdout)}
	uiHandler = jsonUi
	var err error
	if accountManager, err = messages.NewAccountManager(jsonUi); err != nil {
		jsonUi.PrintError(err)
		jsonUi.emit(map[string]any{"type": "exit", "code": 1})
		return
	}
	jsonUi.emit(map[string]any{"type": "ready", "version": VERSION})
	if err := accountManager.StartAll(); err != nil {
		jsonUi.PrintError(err)
	}

	dec := json.NewDecoder(os.Stdin)
	for {
		// "account" is optional: without it the command goes to the active one
		var cmd struct {
			Cmd     string   `json:"cmd"`
			Account string   `json:"account"`
			Params  []string `json:"params"`
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
		accountManager.Send(cmd.Account, messages.Command{cmd.Cmd, cmd.Params})
	}

	stopAudio()
	accountManager.Shutdown() // persists every cache and disconnects
	// give the manager a moment to flush the disconnect before exiting
	time.Sleep(200 * time.Millisecond)
}
