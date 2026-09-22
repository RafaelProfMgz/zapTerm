package messages

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gen2brain/beeep"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var urlPattern = regexp.MustCompile(`https?://[^\s]+`)

// loggedOutCommand is queued by the whatsmeow event handler when the phone
// unpairs this device; the "__" prefix keeps it out of reach of typed commands.
const loggedOutCommand = "__loggedout"

// resyncCommand re-publishes chats, stories and status (and clears the open
// chat) so a UI that just switched to this account can redraw from scratch.
const resyncCommand = "__resync"

// stopCommand ends runManager; sent by Stop when an account is removed.
const stopCommand = "__stop"

// qrPNGPath is where the login QR is saved as an image. A real pairing code is
// ~277 characters, which draws a 65x65 module matrix — about 33 terminal rows
// even at half height — so on smaller windows the image is the only way to
// scan it. The path is deterministic so any goroutine can derive it.
func (sm *SessionManager) qrPNGPath() string {
	return config.GetQRFilePathFor(sm.AccountID)
}

// sessionDBPath is the whatsmeow SQLite file of this manager's account.
func (sm *SessionManager) sessionDBPath() string {
	return config.GetSessionFilePathFor(sm.AccountID) + ".db"
}

// cachePath is the local conversation cache of this manager's account.
func (sm *SessionManager) cachePath() string {
	return config.GetCacheFilePathFor(sm.AccountID)
}

// SessionManager deals with the connection and receives commands from the UI.
type SessionManager struct {
	// AccountID selects the account folder (session, cache, QR) under
	// ~/.config/whatscli/accounts/. Set it before Init; empty means the
	// default account. Never changes afterwards, so any goroutine may read it.
	AccountID       string
	db              *MessageDatabase
	currentReceiver string
	uiHandler       UiMessageHandler
	client          *whatsmeow.Client
	container       *sqlstore.Container
	BatteryChannel  chan BatteryMsg
	StatusChannel   chan StatusMsg
	CommandChannel  chan Command
	LoginChannel    chan loginResult
	ChatChannel     chan Chat
	ContactChannel  chan Contact
	TextChannel     chan *waProto.Message
	statusInfo      SessionStatus
	lastSent        time.Time
	started         bool
	eventHandler    *eventHandler
	userKeys        map[string]string // per-chat OpenAI keys supplied by users (in memory only)
	userKeysLock    sync.RWMutex
	aiChats         map[string]*aiChatState // per-chat "/ai" conversation state (in memory only)
	aiChatsLock     sync.Mutex
	botMsgIDs       map[string]bool // ids of messages the bot itself generated (this session)
	botMsgIDsLock   sync.Mutex
	currentRecvLock sync.RWMutex // guards currentReceiver against the streaming-bot goroutine
	cacheTimer      *time.Timer  // debounces local cache writes
	cacheLock       sync.Mutex   // guards cacheTimer
	loggingIn       atomic.Bool  // a login/reconnect attempt is in flight
	qrCancel        context.CancelFunc
	pendingLogin    *bool         // login requested while another one was unwinding
	done            chan struct{} // closed when runManager returns
	// set by AccountManager when several accounts run side by side (nil when
	// the manager runs alone)
	background   func() bool   // this account is not the one on screen
	notifyPrefix func() string // "[Trabalho] " when more than one account is connected
	// primaryAccount reports whether this is the first account of the list:
	// the one an unqualified bot chat_id runs on
	primaryAccount func() bool
}

func (sm *SessionManager) inBackground() bool {
	return sm.background != nil && sm.background()
}

// notifyTitle is the desktop notification title for a message from contact.
func (sm *SessionManager) notifyTitle(contact string) string {
	if sm.notifyPrefix == nil {
		return contact
	}
	return sm.notifyPrefix() + contact
}

// loginResult is what the async login goroutine reports back to the manager
// goroutine. Only the manager goroutine may touch sm.client, so the goroutine
// reports and the manager acts.
type loginResult struct {
	err error
	// retryQR means the stored session was rejected: drop it and ask for a new
	// QR code.
	retryQR bool
}

// Init initializes the SessionManager.
func (sm *SessionManager) Init(handler UiMessageHandler) {
	if sm.AccountID == "" {
		sm.AccountID = config.DefaultAccountID
	}
	sm.db = &MessageDatabase{}
	sm.db.Init()
	sm.uiHandler = handler
	sm.BatteryChannel = make(chan BatteryMsg, 10)
	sm.StatusChannel = make(chan StatusMsg, 10)
	sm.CommandChannel = make(chan Command, 10)
	sm.LoginChannel = make(chan loginResult, 4)
	sm.ChatChannel = make(chan Chat, 10)
	sm.ContactChannel = make(chan Contact, 10)
	sm.TextChannel = make(chan *waProto.Message, 10)
	sm.eventHandler = &eventHandler{sm: sm}
	sm.userKeys = make(map[string]string)
	sm.aiChats = make(map[string]*aiChatState)
	sm.done = make(chan struct{})
}

// StartManager starts the receiver and message handling goroutine.
func (sm *SessionManager) StartManager() error {
	if sm.started {
		return errors.New("session manager running, send commands to control")
	}
	sm.started = true
	go sm.runManager()
	return nil
}

// publishChats pushes the conversation list and the stories feed to the UI and
// schedules a debounced local-cache write. Use this instead of calling SetChats
// directly so stories stay in sync and the cache reflects new state.
func (sm *SessionManager) publishChats() {
	sm.uiHandler.SetChats(sm.db.GetChatIds())
	sm.uiHandler.SetStories(sm.db.GetStatusUpdates())
	sm.scheduleCacheSave()
}

// scheduleCacheSave writes the local cache 2s after the last change, coalescing
// bursts (history sync, rapid messages) into a single write.
func (sm *SessionManager) scheduleCacheSave() {
	sm.cacheLock.Lock()
	defer sm.cacheLock.Unlock()
	if sm.cacheTimer != nil {
		sm.cacheTimer.Stop()
	}
	sm.cacheTimer = time.AfterFunc(2*time.Second, func() {
		if err := sm.db.SaveCache(sm.cachePath()); err != nil {
			sm.uiHandler.PrintError(err)
		}
	})
}

// flushCache cancels any pending debounced write and persists immediately —
// used on shutdown so the latest state is never lost.
func (sm *SessionManager) flushCache() {
	sm.cacheLock.Lock()
	if sm.cacheTimer != nil {
		sm.cacheTimer.Stop()
		sm.cacheTimer = nil
	}
	sm.cacheLock.Unlock()
	if err := sm.db.SaveCache(sm.cachePath()); err != nil {
		sm.uiHandler.PrintError(err)
	}
}

// FlushCache persists the local cache immediately, cancelling any pending
// debounced write. The UI calls this on shutdown so the last changes survive.
func (sm *SessionManager) FlushCache() {
	sm.flushCache()
}

// Stop ends the manager goroutine (flushing the cache and disconnecting) and
// waits for it, so the account folder can be deleted safely afterwards.
// Commands queued before it (e.g. "logout") still run first.
func (sm *SessionManager) Stop(timeout time.Duration) error {
	if !sm.started {
		return nil
	}
	sm.CommandChannel <- Command{stopCommand, nil}
	select {
	case <-sm.done:
		return nil
	case <-time.After(timeout):
		return errors.New("o gerenciador da conta não parou a tempo")
	}
}

func (sm *SessionManager) runManager() error {
	defer close(sm.done)
	// show cached conversations immediately, before WhatsApp connects
	if err := sm.db.LoadCache(sm.cachePath()); err != nil {
		sm.uiHandler.PrintError(err)
	}
	sm.uiHandler.SetChats(sm.db.GetChatIds())
	sm.uiHandler.SetStories(sm.db.GetStatusUpdates())

	// the login runs in its own goroutine (the QR wait can take minutes), so the
	// command loop below is live from the first frame — that is what makes the
	// reconnect command/button usable while a QR is on screen.
	sm.startLogin(false)

	for sm.started {
		select {
		case command := <-sm.CommandChannel:
			sm.execCommand(command)
		case res := <-sm.LoginChannel:
			sm.handleLoginResult(res)
		case batteryMsg := <-sm.BatteryChannel:
			sm.statusInfo.BatteryLoading = batteryMsg.loading
			sm.statusInfo.BatteryPowersave = batteryMsg.powersave
			sm.statusInfo.BatteryCharge = batteryMsg.charge
			sm.uiHandler.SetStatus(sm.statusInfo)
		case statusMsg := <-sm.StatusChannel:
			prevStatus := sm.statusInfo.Connected
			if statusMsg.err == nil {
				sm.statusInfo.Connected = statusMsg.connected
			}
			sm.refreshStatus()
			if prevStatus != sm.statusInfo.Connected {
				if sm.statusInfo.Connected {
					sm.uiHandler.PrintText("conectado")
				} else {
					sm.uiHandler.PrintText("desconectado")
				}
			}
		}
	}

	sm.flushCache()
	fmt.Fprintln(sm.uiHandler.GetWriter(), "closing the receiver")
	if sm.client != nil {
		sm.client.Disconnect()
	}
	return nil
}

func (sm *SessionManager) setCurrentReceiver(id string) {
	sm.currentRecvLock.Lock()
	sm.currentReceiver = id
	sm.currentRecvLock.Unlock()
	sm.uiHandler.NewScreen(sm.getMessages(id))
}

// getCurrentReceiver returns the currently selected chat id. It is safe to call
// from the bot's streaming goroutine (which runs off the manager goroutine).
func (sm *SessionManager) getCurrentReceiver() string {
	sm.currentRecvLock.RLock()
	defer sm.currentRecvLock.RUnlock()
	return sm.currentReceiver
}

func (sm *SessionManager) getConnection() (*whatsmeow.Client, error) {
	if sm.client == nil {
		dbPath := sm.sessionDBPath()
		container, err := sqlstore.New(context.Background(), "sqlite3", "file:"+dbPath+"?_foreign_keys=on", waLogger("db"))
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %v", err)
		}
		deviceStore, err := container.GetFirstDevice(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get device: %v", err)
		}
		client := whatsmeow.NewClient(deviceStore, waLogger("wa"))
		client.AddEventHandler(sm.eventHandler.Handle)
		sm.client = client
		sm.container = container
	}
	return sm.client, nil
}

// startLogin kicks off a (re)connection attempt. It returns immediately: the
// work runs in a goroutine (the QR wait blocks for as long as the code is on
// screen) and reports back on LoginChannel. forceQR drops the stored session
// first, so a brand new QR code is generated.
func (sm *SessionManager) startLogin(forceQR bool) {
	if sm.loggingIn.Load() {
		// an attempt is already running: cancel it and queue this one, which the
		// manager starts as soon as the goroutine unwinds
		sm.uiHandler.PrintText("cancelando a tentativa de conexão anterior…")
		sm.pendingLogin = &forceQR
		sm.cancelLogin()
		return
	}
	if forceQR {
		sm.clearStoredSession()
	}
	client, err := sm.getConnection()
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("falha ao criar a conexão com o WhatsApp: %v", err))
		sm.refreshStatus()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	sm.qrCancel = cancel
	sm.loggingIn.Store(true)
	sm.refreshStatus()
	go sm.runLogin(ctx, client)
}

// cancelLogin aborts the QR wait of the running attempt (if any). Runs on the
// manager goroutine.
func (sm *SessionManager) cancelLogin() {
	if sm.qrCancel != nil {
		sm.qrCancel()
		sm.qrCancel = nil
	}
}

// clearStoredSession forgets the paired device so the next login shows a QR
// code. Runs on the manager goroutine (it writes sm.client).
func (sm *SessionManager) clearStoredSession() {
	if sm.client != nil {
		if sm.client.IsConnected() {
			sm.client.Disconnect()
		}
		if sm.client.Store != nil && sm.client.Store.ID != nil {
			if err := sm.client.Store.Delete(context.Background()); err != nil {
				sm.uiHandler.PrintText("Aviso: não foi possível remover a sessão anterior: " + err.Error())
			}
		}
	}
	// reconnecting opens a new container every time, so close this one instead
	// of leaking a SQLite handle per attempt
	if sm.container != nil {
		sm.container.Close()
	}
	sm.client = nil
	sm.container = nil
}

// handleLoginResult closes an attempt started by startLogin. Runs on the
// manager goroutine.
func (sm *SessionManager) handleLoginResult(res loginResult) {
	sm.loggingIn.Store(false)
	sm.qrCancel = nil
	if pending := sm.pendingLogin; pending != nil {
		sm.pendingLogin = nil
		sm.startLogin(*pending)
		return
	}
	if res.retryQR {
		sm.startLogin(true) // stored session rejected: start over with a new QR
		return
	}
	if res.err != nil {
		p := config.Config.General.CmdPrefix
		sm.uiHandler.PrintError(fmt.Errorf("falha ao conectar no WhatsApp: %v", res.err))
		sm.uiHandler.PrintText("use " + p + "reconectar para tentar de novo ou " + p + "novoqr para ler um novo QR code")
	} else if sm.client != nil && sm.client.Store != nil && sm.client.Store.ID != nil {
		// remember which number this account is paired with (shown in the
		// account list); failing to write it is not worth bothering the user
		config.SetAccountJID(sm.AccountID, sm.client.Store.ID.ToNonAD().String())
	}
	sm.refreshStatus()
}

// refreshStatus recomputes the session state from the client and pushes it to
// the UI. Runs on the manager goroutine.
func (sm *SessionManager) refreshStatus() {
	connected := sm.client != nil && sm.client.IsConnected()
	loggedIn := sm.client != nil && sm.client.Store != nil && sm.client.Store.ID != nil
	connecting := sm.loggingIn.Load()
	sm.statusInfo.Connected = connected
	sm.statusInfo.LoggedIn = loggedIn
	sm.statusInfo.Connecting = connecting
	sm.statusInfo.NeedsLogin = !loggedIn && !connecting
	sm.uiHandler.SetStatus(sm.statusInfo)
}

// runLogin performs the connection off the manager goroutine. It only reads the
// client pointer it was handed — every write to sm.client stays on the manager
// goroutine — and always reports exactly one result.
func (sm *SessionManager) runLogin(ctx context.Context, client *whatsmeow.Client) {
	res := loginResult{}
	defer func() { sm.LoginChannel <- res }()

	sm.uiHandler.PrintText("conectando…")
	if client.IsConnected() {
		client.Disconnect()
		sm.StatusChannel <- StatusMsg{false, nil}
		time.Sleep(500 * time.Millisecond)
	}

	if client.Store.ID == nil {
		res.err = sm.waitForQRCode(ctx, client)
		return
	}

	if err := client.Connect(); err != nil {
		if errors.Is(err, whatsmeow.ErrNotConnected) || errors.Is(err, whatsmeow.ErrNotLoggedIn) {
			sm.uiHandler.PrintText("sessão expirada — é preciso ler um novo QR code")
			res.retryQR = true
			return
		}
		res.err = fmt.Errorf("conexão falhou: %v", err)
		return
	}

	sm.uiHandler.PrintText("sessão restaurada")
	sm.StatusChannel <- StatusMsg{true, nil}
	go sm.loadRecentChats()
}

// waitForQRCode streams pairing codes to the UI until the phone scans one, the
// code expires or ctx is cancelled (reconnect/disconnect asked for it).
func (sm *SessionManager) waitForQRCode(ctx context.Context, client *whatsmeow.Client) error {
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("falha ao iniciar o canal de QR: %v", err)
	}
	if err = client.Connect(); err != nil {
		return fmt.Errorf("erro ao conectar no WhatsApp: %v", err)
	}

	pngPath := sm.qrPNGPath()
	for evt := range qrChan {
		switch evt.Event {
		case "code":
			qr := QRCode{
				Event:   QRCodeShow,
				Code:    evt.Code,
				Message: "leia o QR code no celular: WhatsApp > Aparelhos conectados > Conectar aparelho",
			}
			// the drawn code can be unscannable on narrow panels, so a clean PNG
			// goes along as a fallback
			if matrix, mErr := qrcode.Matrix(evt.Code); mErr == nil {
				qr.Matrix = matrix
			}
			if pngErr := qrcode.SavePNG(evt.Code, pngPath, 512); pngErr == nil {
				qr.PngPath = pngPath
			}
			sm.uiHandler.SetQRCode(qr)
		case "success":
			sm.uiHandler.SetQRCode(QRCode{Event: QRCodeSuccess, Message: "aparelho conectado com sucesso"})
			sm.StatusChannel <- StatusMsg{true, nil}
			go sm.loadRecentChats()
			return nil
		case "timeout":
			p := config.Config.General.CmdPrefix
			sm.uiHandler.SetQRCode(QRCode{Event: QRCodeDone, Message: "o QR code expirou — use " + p + "novoqr para gerar outro"})
			return errors.New("o QR code expirou sem ser lido")
		default:
			sm.uiHandler.PrintText("QR: " + evt.Event)
		}
	}

	if ctx.Err() != nil { // cancelled on purpose: not a failure
		sm.uiHandler.SetQRCode(QRCode{Event: QRCodeDone, Message: "leitura do QR code cancelada"})
		return nil
	}
	sm.uiHandler.SetQRCode(QRCode{Event: QRCodeDone, Message: "o canal do QR code fechou sem conexão"})
	return errors.New("canal do QR code fechado sem sucesso")
}

func (sm *SessionManager) loadRecentChats() {
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	sm.loadContacts()
	addedChats := 0

	if sm.client.Store != nil && sm.client.Store.Contacts != nil {
		contacts, err := sm.client.Store.Contacts.GetAllContacts(context.Background())
		if err == nil {
			for jid, contact := range contacts {
				if jid.Server != types.DefaultUserServer {
					continue
				}
				name := contact.FullName
				if name == "" {
					name = contact.PushName
				}
				if name == "" {
					name = jid.User
				}
				sm.db.AddChat(Chat{
					Id:      jid.String(),
					IsGroup: false,
					Name:    name,
				})
				addedChats++
			}
		}
	}

	groups, err := sm.client.GetJoinedGroups(context.Background())
	if err == nil {
		for _, group := range groups {
			sm.db.AddChat(Chat{
				Id:      group.JID.String(),
				IsGroup: true,
				Name:    group.Name,
			})
			addedChats++
		}
	}

	sm.publishChats()
	if addedChats > 0 {
		sm.uiHandler.PrintText(fmt.Sprintf("Loaded %d chats", addedChats))
	}
}

func (sm *SessionManager) loadContacts() {
	if sm.client == nil || sm.client.Store == nil || sm.client.Store.Contacts == nil {
		return
	}

	contacts, err := sm.client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("failed to load contacts: %v", err))
		return
	}

	contactCount := 0
	for jid, contact := range contacts {
		name := contact.FullName
		if name == "" {
			name = contact.PushName
		}
		if name == "" {
			name = jid.User
		}
		sm.db.AddContact(Contact{
			Id:    jid.String(),
			Name:  name,
			Short: contact.PushName,
		})
		contactCount++
	}
	if contactCount > 0 {
		sm.uiHandler.PrintText(fmt.Sprintf("Loaded %d contacts", contactCount))
	}
}

func (sm *SessionManager) getChatName(jid types.JID) string {
	if jid.Server == types.GroupServer {
		groupInfo, err := sm.client.GetGroupInfo(context.Background(), jid)
		if err == nil && groupInfo.Name != "" {
			return groupInfo.Name
		}
	}
	if sm.client != nil && sm.client.Store != nil && sm.client.Store.Contacts != nil {
		contact, err := sm.client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.Found {
			if contact.FullName != "" {
				return contact.FullName
			}
			if contact.PushName != "" {
				return contact.PushName
			}
		}
	}
	return sm.db.GetIdName(jid.String())
}

func (sm *SessionManager) disconnect() error {
	sm.cancelLogin() // drops a QR wait still on screen
	if sm.client != nil && sm.client.IsConnected() {
		sm.client.Disconnect()
		sm.StatusChannel <- StatusMsg{false, nil}
	}
	sm.refreshStatus()
	return nil
}

func (sm *SessionManager) logout() error {
	sm.cancelLogin()
	if sm.client == nil {
		sm.StatusChannel <- StatusMsg{false, nil}
		sm.uiHandler.PrintText("já desconectado da conta")
		return nil
	}

	if sm.client.Store != nil && sm.client.Store.ID != nil {
		if err := sm.client.Logout(context.Background()); err != nil && !errors.Is(err, whatsmeow.ErrNotConnected) {
			sm.uiHandler.PrintText("Aviso: não foi possível sair completamente: " + err.Error())
		}
	}
	sm.clearStoredSession()
	sm.StatusChannel <- StatusMsg{false, nil}
	p := config.Config.General.CmdPrefix
	sm.uiHandler.PrintText("desconectado da conta — use " + p + "novoqr para entrar com outro QR code")
	return nil
}

func (sm *SessionManager) execCommand(command Command) {
	switch command.Name {
	default:
		sm.uiHandler.PrintText("[" + config.Config.Colors.Negative + "]Unknown command: [-]" + command.Name)
	case "backlog":
		sm.loadBacklog()
	case "login", "connect", "reconnect", "reconectar", "re-conect", "re-connect", "conectar":
		// reconnect with the stored session; falls back to a QR code when there
		// is no session or the server rejects it
		sm.startLogin(false)
	case "newqr", "novoqr", "qr", "relogin", "re-login", "novo-qr":
		// forget the stored session and show a fresh QR code right away
		sm.startLogin(true)
	case "cancelqr", "cancelar":
		sm.cancelLogin()
	case "openqr", "abrirqr":
		// abre a imagem do QR no visualizador do sistema — saída para quando a
		// janela do terminal é pequena demais para desenhar o código
		path := sm.qrPNGPath()
		if _, err := os.Stat(path); err != nil {
			sm.uiHandler.PrintError(errors.New("nenhum QR code salvo ainda — use " + config.Config.General.CmdPrefix + "novoqr"))
			return
		}
		sm.uiHandler.OpenFile(path)
	case loggedOutCommand:
		sm.handlePhoneLogout()
	case resyncCommand:
		sm.currentRecvLock.Lock()
		sm.currentReceiver = ""
		sm.currentRecvLock.Unlock()
		sm.uiHandler.SetChats(sm.db.GetChatIds())
		sm.uiHandler.SetStories(sm.db.GetStatusUpdates())
		sm.uiHandler.NewScreen(nil)
		sm.refreshStatus()
	case stopCommand:
		sm.cancelLogin()
		sm.started = false // runManager leaves its loop after this command
	case "reset":
		sm.resetSession()
	case "disconnect":
		sm.uiHandler.PrintError(sm.disconnect())
	case "logout":
		sm.uiHandler.PrintError(sm.logout())
	case "send":
		if checkParam(command.Params, 2) {
			sm.sendText(command.Params[0], strings.Join(command.Params[1:], " "))
		} else {
			sm.printCommandUsage("send", "[chat-id[] [message text[]")
		}
	case "select":
		if checkParam(command.Params, 1) {
			sm.setCurrentReceiver(command.Params[0])
		} else {
			sm.printCommandUsage("select", "[chat-id[]")
		}
	case "read":
		sm.markCurrentChatRead()
	case "info":
		if checkParam(command.Params, 1) {
			sm.uiHandler.PrintText(sm.db.GetMessageInfo(command.Params[0]))
		} else {
			sm.printCommandUsage("info", "[message-id[]")
		}
	case "download":
		sm.downloadCommand(command.Params, false, false)
	case "open":
		sm.downloadCommand(command.Params, true, false)
	case "show":
		sm.downloadCommand(command.Params, true, true)
	case "play":
		sm.playCommand(command.Params)
	case "url":
		sm.openMessageURL(command.Params)
	case "upload":
		sm.sendMediaCommand(command.Params, MessageKindDocument)
	case "sendimage":
		sm.sendMediaCommand(command.Params, MessageKindImage)
	case "sendvideo":
		sm.sendMediaCommand(command.Params, MessageKindVideo)
	case "sendaudio":
		sm.sendMediaCommand(command.Params, MessageKindAudio)
	case "revoke":
		sm.revokeMessage(command.Params)
	case "leave":
		sm.leaveCurrentGroup()
	case "create":
		sm.createGroup(command.Params)
	case "add":
		sm.updateCurrentGroupParticipants(command.Params, whatsmeow.ParticipantChangeAdd, "add", "added new members")
	case "remove":
		sm.updateCurrentGroupParticipants(command.Params, whatsmeow.ParticipantChangeRemove, "remove", "removed members")
	case "admin":
		sm.updateCurrentGroupParticipants(command.Params, whatsmeow.ParticipantChangePromote, "admin", "promoted members")
	case "removeadmin":
		sm.updateCurrentGroupParticipants(command.Params, whatsmeow.ParticipantChangeDemote, "removeadmin", "demoted members")
	case "subject":
		sm.updateCurrentGroupSubject(command.Params)
	case "colorlist":
		out := ""
		for idx := range tcell.ColorNames {
			out += "[" + idx + "]" + idx + "[-]\n"
		}
		sm.uiHandler.PrintText(out)
	case "more":
		sm.loadBacklog()
	}
}

func (sm *SessionManager) loadBacklog() {
	if sm.currentReceiver == "" {
		sm.printCommandUsage("backlog", "-> only works in a chat")
		return
	}
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	jid, err := types.ParseJID(sm.currentReceiver)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("invalid JID: %v", err))
		return
	}

	existingMessages := sm.db.GetMessages(sm.currentReceiver)
	sm.uiHandler.PrintText("Retrieving message history...")

	oldest, ok := sm.db.GetOldestMessage(sm.currentReceiver)
	if !ok {
		sm.uiHandler.PrintText("No local message anchor found yet. Open the chat after WhatsApp sync delivers some history, then try /backlog again.")
		sm.uiHandler.NewScreen(existingMessages)
		return
	}

	senderJID := types.EmptyJID
	if oldest.SenderId != "" {
		if parsedSender, parseErr := types.ParseJID(oldest.SenderId); parseErr == nil {
			senderJID = parsedSender
		}
	}
	req := sm.client.BuildHistorySyncRequest(&types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     jid,
			Sender:   senderJID,
			IsFromMe: oldest.FromMe,
			IsGroup:  strings.Contains(sm.currentReceiver, GROUPSUFFIX),
		},
		ID:        types.MessageID(oldest.Id),
		Timestamp: time.Unix(int64(oldest.Timestamp), 0),
	}, config.Config.General.BacklogMsgQuantity)
	if _, err = sm.client.SendPeerMessage(context.Background(), req); err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("failed to request message history: %v", err))
		sm.uiHandler.NewScreen(existingMessages)
		return
	}

	deadline := time.Now().Add(5 * time.Second)
	for len(sm.db.GetMessages(sm.currentReceiver)) == len(existingMessages) && time.Now().Before(deadline) {
		time.Sleep(250 * time.Millisecond)
	}

	if len(sm.db.GetMessages(sm.currentReceiver)) == len(existingMessages) {
		sm.uiHandler.PrintText("Requested older messages from WhatsApp. Waiting for sync response.")
	}

	updated := sm.db.GetMessages(sm.currentReceiver)
	if len(updated) > len(existingMessages) {
		sm.uiHandler.PrintText(fmt.Sprintf("Loaded %d additional messages", len(updated)-len(existingMessages)))
	} else {
		sm.uiHandler.PrintText("No additional messages found. WhatsApp may limit history access.")
	}
	sm.uiHandler.NewScreen(updated)
}

func (sm *SessionManager) resetSession() {
	sm.cancelLogin()
	sm.clearStoredSession() // desconecta, apaga o pareamento e fecha o SQLite
	dbPath := sm.sessionDBPath()
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		sm.uiHandler.PrintText("Aviso: não foi possível remover o arquivo da sessão: " + err.Error())
	}
	sm.StatusChannel <- StatusMsg{false, nil}
	p := config.Config.General.CmdPrefix
	sm.uiHandler.PrintText("sessão apagada — use " + p + "reconectar para ler um novo QR code")
	sm.refreshStatus()
}

// handlePhoneLogout reacts to the device being unpaired from the phone. It runs
// on the manager goroutine (the whatsmeow event handler only queues the
// command) so it may drop the stored session and start a new login.
func (sm *SessionManager) handlePhoneLogout() {
	p := config.Config.General.CmdPrefix
	sm.uiHandler.PrintText("a sessão foi encerrada no celular")
	sm.clearStoredSession()
	sm.refreshStatus()
	if config.Config.General.AutoReconnect {
		sm.uiHandler.PrintText("gerando um novo QR code…")
		sm.startLogin(true)
		return
	}
	sm.uiHandler.PrintText("use " + p + "reconectar para ler um novo QR code")
}

func (sm *SessionManager) markCurrentChatRead() {
	if sm.currentReceiver == "" {
		sm.printCommandUsage("read", "-> only works in a chat")
		return
	}
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	chatJID, err := types.ParseJID(sm.currentReceiver)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("invalid JID: %v", err))
		return
	}

	unreadMessages := sm.db.MarkChatRead(sm.currentReceiver)
	if len(unreadMessages) == 0 {
		sm.publishChats()
		sm.uiHandler.PrintText("No unread messages in current chat")
		return
	}

	type senderBatch struct {
		sender    types.JID
		ids       []types.MessageID
		timestamp time.Time
	}
	batches := make(map[string]*senderBatch)
	for _, msg := range unreadMessages {
		sender := chatJID
		if strings.Contains(sm.currentReceiver, GROUPSUFFIX) && msg.SenderId != "" {
			sender, err = types.ParseJID(msg.SenderId)
			if err != nil {
				continue
			}
		}
		key := sender.String()
		if _, ok := batches[key]; !ok {
			batches[key] = &senderBatch{sender: sender}
		}
		batches[key].ids = append(batches[key].ids, types.MessageID(msg.Id))
		ts := time.Unix(int64(msg.Timestamp), 0)
		if ts.After(batches[key].timestamp) {
			batches[key].timestamp = ts
		}
	}

	for _, batch := range batches {
		if batch.timestamp.IsZero() {
			batch.timestamp = time.Now()
		}
		if err := sm.client.MarkRead(context.Background(), batch.ids, batch.timestamp, chatJID, batch.sender); err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("failed to mark messages as read: %v", err))
		}
	}

	sm.publishChats()
}

func (sm *SessionManager) downloadCommand(params []string, preview, show bool) {
	if !checkParam(params, 1) {
		name := "download"
		if preview && !show {
			name = "open"
		} else if show {
			name = "show"
		}
		sm.printCommandUsage(name, "[message-id[]")
		return
	}

	msg, ok := sm.db.GetMessage(params[0])
	if !ok {
		sm.uiHandler.PrintError(errors.New("message not found"))
		return
	}
	if show && msg.Kind != MessageKindImage {
		sm.uiHandler.PrintError(errors.New("show only works for image messages"))
		return
	}

	path, err := sm.downloadMessage(msg, preview)
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	if show {
		sm.uiHandler.PrintFile(path, msg.Id)
		return
	}
	if preview {
		sm.uiHandler.OpenFile(path)
		return
	}
	sm.uiHandler.PrintText("[::d] -> " + path + "[::-]")
}

// playCommand downloads an audio (or video) message and hands it to the UI's
// audio player; non-playable attachments fall back to the system viewer.
func (sm *SessionManager) playCommand(params []string) {
	if !checkParam(params, 1) {
		sm.printCommandUsage("play", "[message-id[]")
		return
	}
	msg, ok := sm.db.GetMessage(params[0])
	if !ok {
		sm.uiHandler.PrintError(errors.New("message not found"))
		return
	}
	switch msg.Kind {
	case MessageKindAudio, MessageKindVideo:
	default:
		sm.uiHandler.PrintError(errors.New("play only works for audio/video messages"))
		return
	}
	path, err := sm.downloadMessage(msg, true)
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	sm.uiHandler.PlayFile(path, msg.Id)
}

func (sm *SessionManager) openMessageURL(params []string) {
	if !checkParam(params, 1) {
		sm.printCommandUsage("url", "[message-id[]")
		return
	}
	msg, ok := sm.db.GetMessage(params[0])
	if !ok {
		sm.uiHandler.PrintError(errors.New("message not found"))
		return
	}
	url := urlPattern.FindString(msg.Text)
	if url == "" {
		sm.uiHandler.PrintText("No URL found in message")
		return
	}
	sm.uiHandler.OpenFile(url)
}

func (sm *SessionManager) sendMediaCommand(params []string, kind MessageKind) {
	if sm.currentReceiver == "" {
		sm.printCommandUsage(commandNameForKind(kind), "-> only works in a chat")
		return
	}
	if !checkParam(params, 1) {
		sm.printCommandUsage(commandNameForKind(kind), "/path/to/file")
		return
	}
	path := strings.Join(params, " ")
	sm.uiHandler.PrintError(sm.sendMedia(sm.currentReceiver, path, kind))
}

func (sm *SessionManager) revokeMessage(params []string) {
	if !checkParam(params, 1) {
		sm.printCommandUsage("revoke", "[message-id[]")
		return
	}
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	msg, ok := sm.db.GetMessage(params[0])
	if !ok {
		sm.uiHandler.PrintError(errors.New("message not found"))
		return
	}
	chatJID, err := types.ParseJID(msg.ChatId)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("invalid chat JID: %v", err))
		return
	}
	if _, err = sm.client.RevokeMessage(context.Background(), chatJID, types.MessageID(msg.Id)); err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	sm.db.MarkMessageRevoked(msg.Id)
	if sm.currentReceiver == msg.ChatId {
		sm.uiHandler.NewScreen(sm.getMessages(msg.ChatId))
	}
	sm.uiHandler.PrintText("revoked: " + msg.Id)
}

func (sm *SessionManager) leaveCurrentGroup() {
	groupJID, err := sm.currentGroupJID()
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	if err = sm.client.LeaveGroup(context.Background(), groupJID); err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	sm.uiHandler.PrintText("left group " + groupJID.String())
}

func (sm *SessionManager) createGroup(params []string) {
	if !checkParam(params, 1) {
		sm.printCommandUsage("create", "[user-id[] [user-id[] New Group Subject")
		sm.printCommandUsage("create", "New Group Subject")
		return
	}

	participants := make([]types.JID, 0)
	idx := 0
	for idx < len(params) && strings.Contains(params[idx], CONTACTSUFFIX) {
		participant, err := types.ParseJID(params[idx])
		if err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("invalid user id %q: %v", params[idx], err))
			return
		}
		participants = append(participants, participant)
		idx++
	}

	name := strings.Join(params[idx:], " ")
	if name == "" {
		name = strings.Join(params, " ")
		participants = nil
	}

	groupInfo, err := sm.client.CreateGroup(context.Background(), whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: participants,
	})
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}

	sm.db.AddChat(Chat{
		Id:          groupInfo.JID.String(),
		IsGroup:     true,
		Name:        groupInfo.Name,
		LastMessage: time.Now().Unix(),
	})
	sm.publishChats()
	sm.uiHandler.PrintText("created new group " + groupInfo.JID.String())
}

func (sm *SessionManager) updateCurrentGroupParticipants(params []string, action whatsmeow.ParticipantChange, command, success string) {
	groupJID, err := sm.currentGroupJID()
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	if !checkParam(params, 1) {
		sm.printCommandUsage(command, "[user-id[]")
		return
	}

	participants := make([]types.JID, 0, len(params))
	for _, raw := range params {
		jid, err := types.ParseJID(raw)
		if err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("invalid user id %q: %v", raw, err))
			return
		}
		participants = append(participants, jid)
	}

	if _, err = sm.client.UpdateGroupParticipants(context.Background(), groupJID, participants, action); err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	sm.uiHandler.PrintText(success + " for " + groupJID.String())
}

func (sm *SessionManager) updateCurrentGroupSubject(params []string) {
	groupJID, err := sm.currentGroupJID()
	if err != nil {
		sm.uiHandler.PrintError(err)
		return
	}
	if !checkParam(params, 1) {
		sm.printCommandUsage("subject", "new-subject -> in group chat")
		return
	}

	name := strings.Join(params, " ")
	if err = sm.client.SetGroupName(context.Background(), groupJID, name); err != nil {
		sm.uiHandler.PrintError(err)
		return
	}

	sm.db.AddChat(Chat{
		Id:      groupJID.String(),
		IsGroup: true,
		Name:    name,
	})
	sm.publishChats()
	sm.uiHandler.PrintText("updated subject for " + groupJID.String())
}

func (sm *SessionManager) currentGroupJID() (types.JID, error) {
	if sm.currentReceiver == "" || !strings.Contains(sm.currentReceiver, GROUPSUFFIX) {
		return types.JID{}, errors.New("not a group")
	}
	return types.ParseJID(sm.currentReceiver)
}

func (sm *SessionManager) printCommandUsage(command, usage string) {
	sm.uiHandler.PrintText("[" + config.Config.Colors.Negative + "]Usage:[-] " + command + " " + usage)
}

func checkParam(arr []string, length int) bool {
	return arr != nil && len(arr) >= length
}

func (sm *SessionManager) getMessages(wid string) []Message {
	return sm.db.GetMessages(wid)
}

func (sm *SessionManager) sendText(wid, text string) {
	if sm.client == nil || !sm.client.IsConnected() {
		sm.uiHandler.PrintError(errors.New("not connected to WhatsApp"))
		return
	}

	receiver, err := types.ParseJID(wid)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("invalid JID: %v", err))
		return
	}

	raw := &waProto.Message{Conversation: proto.String(text)}
	sm.lastSent = time.Now()
	resp, err := sm.client.SendMessage(context.Background(), receiver, raw)
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("failed to send message: %v", err))
		return
	}

	newMsg := sm.outgoingMessageFromSendResponse(resp, wid, raw, MessageKindText, text, "", "")
	sm.db.AddMessage(newMsg, false)
	if sm.currentReceiver == wid {
		sm.uiHandler.NewMessage(newMsg)
	}
	sm.publishChats()
}

func (sm *SessionManager) sendMedia(chatID, path string, kind MessageKind) error {
	if sm.client == nil || !sm.client.IsConnected() {
		return errors.New("not connected to WhatsApp")
	}

	data, mimeType, fileName, err := readUploadFile(path)
	if err != nil {
		return err
	}

	receiver, err := types.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid JID: %v", err)
	}

	uploadResp, err := sm.client.Upload(context.Background(), data, uploadMediaType(kind))
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	fileLength := uploadResp.FileLength
	raw := &waProto.Message{}
	switch kind {
	case MessageKindImage:
		raw.ImageMessage = &waProto.ImageMessage{
			Mimetype:      proto.String(mimeType),
			URL:           &uploadResp.URL,
			DirectPath:    &uploadResp.DirectPath,
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    &fileLength,
		}
	case MessageKindVideo:
		raw.VideoMessage = &waProto.VideoMessage{
			Mimetype:      proto.String(mimeType),
			URL:           &uploadResp.URL,
			DirectPath:    &uploadResp.DirectPath,
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    &fileLength,
		}
	case MessageKindAudio:
		raw.AudioMessage = &waProto.AudioMessage{
			Mimetype:      proto.String(mimeType),
			URL:           &uploadResp.URL,
			DirectPath:    &uploadResp.DirectPath,
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    &fileLength,
			PTT:           proto.Bool(false),
		}
	case MessageKindDocument:
		raw.DocumentMessage = &waProto.DocumentMessage{
			Mimetype:      proto.String(mimeType),
			Title:         proto.String(fileName),
			FileName:      proto.String(fileName),
			URL:           &uploadResp.URL,
			DirectPath:    &uploadResp.DirectPath,
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    &fileLength,
		}
	default:
		return errors.New("unsupported media type")
	}

	sm.lastSent = time.Now()
	resp, err := sm.client.SendMessage(context.Background(), receiver, raw)
	if err != nil {
		return fmt.Errorf("failed to send media message: %v", err)
	}

	text := mediaDisplayText(kind, fileName, "")
	newMsg := sm.outgoingMessageFromSendResponse(resp, chatID, raw, kind, text, mimeType, fileName)
	sm.db.AddMessage(newMsg, false)
	if sm.currentReceiver == chatID {
		sm.uiHandler.NewMessage(newMsg)
	}
	sm.publishChats()
	return nil
}

func (sm *SessionManager) outgoingMessageFromSendResponse(resp whatsmeow.SendResponse, chatID string, raw *waProto.Message, kind MessageKind, text, mimeType, fileName string) Message {
	selfID := ""
	if sm.client != nil && sm.client.Store != nil && sm.client.Store.ID != nil {
		selfID = sm.client.Store.ID.String()
	}

	contactID := chatID
	if strings.Contains(chatID, GROUPSUFFIX) {
		contactID = selfID
	}

	return Message{
		Id:           string(resp.ID),
		ChatId:       chatID,
		SenderId:     selfID,
		ContactId:    contactID,
		ContactName:  sm.db.GetIdName(contactID),
		ContactShort: sm.db.GetIdShort(contactID),
		Timestamp:    uint64(resp.Timestamp.Unix()),
		FromMe:       true,
		Text:         text,
		Kind:         kind,
		MimeType:     mimeType,
		FileName:     fileName,
		RawMessage:   raw,
	}
}

// Headless is set by the --ui=json entrypoint (Ink frontend). When true,
// stdout carries the NDJSON protocol, so the terminal bell must not be written
// there — it goes to stderr (inherited by the launcher) instead.
var Headless bool

func notify(title, message string) error {
	if !config.Config.General.EnableNotifications {
		return nil
	}
	// Audible terminal bell (opt-in). In headless mode the bell is routed to
	// stderr so it reaches the real terminal without corrupting the NDJSON
	// protocol that flows over stdout.
	if config.Config.General.UseTerminalBell {
		out := os.Stdout
		if Headless {
			out = os.Stderr
		}
		fmt.Fprint(out, "\a")
	}
	// Desktop notification (popup + the system's own notification sound).
	// beeep shells out to notify-send/dbus on Linux and toast on Windows, so it
	// works the same whether running under tview or the headless Ink core.
	if config.Config.General.EnableSystemNotification {
		return beeep.Notify(title, message, "")
	}
	return nil
}

type eventHandler struct {
	sm *SessionManager
}

func (eh *eventHandler) Handle(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		eh.handleLiveMessage(v)
	case *events.HistorySync:
		eh.handleHistorySync(v)
	case *events.Connected:
		eh.sm.StatusChannel <- StatusMsg{true, nil}
	case *events.Disconnected:
		eh.sm.StatusChannel <- StatusMsg{false, nil}
	case *events.LoggedOut:
		eh.sm.StatusChannel <- StatusMsg{false, nil}
		eh.sm.uiHandler.PrintText("sessão encerrada: " + fmt.Sprintf("%v", v.Reason))
		// mutating the client is the manager goroutine's job, so queue it
		eh.sm.CommandChannel <- Command{loggedOutCommand, nil}
	}
}

func (eh *eventHandler) handleLiveMessage(evt *events.Message) {
	msg, action, ok := eh.normalizeEventMessage(evt)
	if !ok {
		return
	}

	switch action {
	case "revoke":
		if eh.sm.db.MarkMessageRevoked(msg.Id) && eh.sm.currentReceiver == msg.ChatId {
			eh.sm.uiHandler.NewScreen(eh.sm.getMessages(msg.ChatId))
		}
		eh.sm.publishChats()
		return
	case "ignore":
		return
	}

	markUnread := !msg.FromMe && msg.ChatId != eh.sm.currentReceiver
	isNew := eh.sm.db.AddMessage(msg, markUnread)
	if isNew {
		eh.sm.maybeReplyWithBot(msg)
	}
	isCurrent := msg.ChatId == eh.sm.currentReceiver
	if isCurrent {
		if isNew {
			eh.sm.uiHandler.NewMessage(msg)
		} else {
			eh.sm.uiHandler.NewScreen(eh.sm.getMessages(msg.ChatId))
		}
	}
	// the open chat of an account in the background is not on screen, so it
	// still notifies
	if markUnread && msg.Timestamp > uint64(time.Now().Unix()-30) && (!isCurrent || eh.sm.inBackground()) {
		if err := notify(eh.sm.notifyTitle(msg.ContactShort), msg.Text); err != nil {
			eh.sm.uiHandler.PrintError(err)
		}
	}
	eh.sm.publishChats()
}

func (eh *eventHandler) handleHistorySync(evt *events.HistorySync) {
	if evt == nil || evt.Data == nil {
		return
	}

	for _, conv := range evt.Data.GetConversations() {
		chatID := conv.GetID()
		if chatID == "" {
			chatID = conv.GetNewJID()
		}
		if chatID == "" {
			continue
		}

		chatJID, err := types.ParseJID(chatID)
		if err != nil {
			continue
		}

		chatName := conv.GetName()
		if chatName == "" {
			chatName = conv.GetDisplayName()
		}
		if chatName == "" {
			chatName = eh.sm.getChatName(chatJID)
		}

		lastMessage := int64(conv.GetLastMsgTimestamp())
		if lastMessage == 0 {
			lastMessage = int64(conv.GetConversationTimestamp())
		}
		eh.sm.db.AddChat(Chat{
			Id:          chatID,
			IsGroup:     chatJID.Server == types.GroupServer,
			Name:        chatName,
			Unread:      int(conv.GetUnreadCount()),
			LastMessage: lastMessage,
		})

		for _, histMsg := range conv.GetMessages() {
			webMsg := histMsg.GetMessage()
			if webMsg == nil {
				continue
			}
			parsed, err := eh.sm.client.ParseWebMessage(chatJID, webMsg)
			if err != nil {
				continue
			}
			msg, action, ok := eh.normalizeEventMessage(parsed)
			if !ok || action != "" {
				continue
			}
			eh.sm.db.AddMessage(msg, false)
		}
		eh.sm.db.UpdateChatUnread(chatID, int(conv.GetUnreadCount()))
	}

	eh.sm.publishChats()
	if eh.sm.currentReceiver != "" {
		eh.sm.uiHandler.NewScreen(eh.sm.getMessages(eh.sm.currentReceiver))
	}
}

func (eh *eventHandler) normalizeEventMessage(evt *events.Message) (Message, string, bool) {
	if evt == nil || evt.Message == nil {
		return Message{}, "ignore", false
	}

	if protocol := evt.Message.GetProtocolMessage(); protocol != nil {
		if protocol.GetType() == waProto.ProtocolMessage_REVOKE && protocol.GetKey() != nil {
			return Message{
				Id:     protocol.GetKey().GetID(),
				ChatId: evt.Info.Chat.String(),
			}, "revoke", true
		}
		return Message{}, "ignore", false
	}

	msg, ok := eh.messageFromInfo(evt.Info, evt.Message)
	return msg, "", ok
}

func (eh *eventHandler) messageFromInfo(info types.MessageInfo, raw *waProto.Message) (Message, bool) {
	if raw == nil {
		return Message{}, false
	}

	chatID := info.Chat.String()
	if chatID == "" {
		return Message{}, false
	}

	contactID, contactName, contactShort := eh.contactForMessage(info)
	msg := Message{
		Id:           string(info.ID),
		ChatId:       chatID,
		SenderId:     info.Sender.String(),
		ContactId:    contactID,
		ContactName:  contactName,
		ContactShort: contactShort,
		Timestamp:    uint64(info.Timestamp.Unix()),
		FromMe:       info.IsFromMe,
		RawMessage:   raw,
	}

	switch {
	case raw.GetConversation() != "":
		msg.Kind = MessageKindText
		msg.Text = raw.GetConversation()
		return msg, true
	case raw.GetExtendedTextMessage() != nil:
		ext := raw.GetExtendedTextMessage()
		msg.Kind = MessageKindText
		msg.Text = ext.GetText()
		msg.Forwarded = ext.GetContextInfo().GetIsForwarded()
		return msg, true
	case raw.GetImageMessage() != nil:
		image := raw.GetImageMessage()
		msg.Kind = MessageKindImage
		msg.MimeType = image.GetMimetype()
		msg.Text = mediaDisplayText(MessageKindImage, "", image.GetCaption())
		msg.Forwarded = image.GetContextInfo().GetIsForwarded()
		return msg, true
	case raw.GetVideoMessage() != nil:
		video := raw.GetVideoMessage()
		msg.Kind = MessageKindVideo
		msg.MimeType = video.GetMimetype()
		msg.DurationSecs = video.GetSeconds()
		msg.Text = mediaDisplayText(MessageKindVideo, "", video.GetCaption())
		msg.Forwarded = video.GetContextInfo().GetIsForwarded()
		return msg, true
	case raw.GetAudioMessage() != nil:
		audio := raw.GetAudioMessage()
		msg.Kind = MessageKindAudio
		msg.MimeType = audio.GetMimetype()
		msg.DurationSecs = audio.GetSeconds()
		msg.Text = mediaDisplayText(MessageKindAudio, "", "")
		msg.Forwarded = audio.GetContextInfo().GetIsForwarded()
		return msg, true
	case raw.GetDocumentMessage() != nil:
		doc := raw.GetDocumentMessage()
		msg.Kind = MessageKindDocument
		msg.MimeType = doc.GetMimetype()
		msg.FileName = doc.GetFileName()
		msg.Text = mediaDisplayText(MessageKindDocument, doc.GetFileName(), doc.GetCaption())
		msg.Forwarded = doc.GetContextInfo().GetIsForwarded()
		return msg, true
	default:
		return Message{}, false
	}
}

func (eh *eventHandler) contactForMessage(info types.MessageInfo) (string, string, string) {
	// status@broadcast carries everyone's stories under one chat; attribute each
	// post to its real sender (like a group) so stories can be grouped by author.
	if info.IsGroup || info.Chat.String() == STATUSSUFFIX {
		id := info.Sender.String()
		return id, eh.getContactName(info.Sender), eh.getContactShort(info.Sender)
	}
	id := info.Chat.String()
	chat := info.Chat
	return id, eh.getContactName(chat), eh.getContactShort(chat)
}

func (eh *eventHandler) getContactName(jid types.JID) string {
	if eh.sm.client != nil && eh.sm.client.Store != nil && eh.sm.client.Store.Contacts != nil {
		contact, err := eh.sm.client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.Found {
			if contact.FullName != "" {
				return contact.FullName
			}
			if contact.PushName != "" {
				return contact.PushName
			}
		}
	}
	return eh.sm.db.GetIdName(jid.String())
}

func (eh *eventHandler) getContactShort(jid types.JID) string {
	if eh.sm.client != nil && eh.sm.client.Store != nil && eh.sm.client.Store.Contacts != nil {
		contact, err := eh.sm.client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.Found && contact.PushName != "" {
			return contact.PushName
		}
	}
	return eh.sm.db.GetIdShort(jid.String())
}

func (sm *SessionManager) downloadMessage(msg Message, preview bool) (string, error) {
	if sm.client == nil || !sm.client.IsConnected() {
		return "", errors.New("not connected to WhatsApp")
	}

	downloadable, err := downloadableFromMessage(msg)
	if err != nil {
		return "", err
	}

	baseDir := config.Config.General.DownloadPath
	if preview {
		baseDir = config.Config.General.PreviewPath
	}
	if err = os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}

	fileName := downloadFileName(msg)
	fullPath := filepath.Join(baseDir, fileName)
	if _, err = os.Stat(fullPath); err == nil {
		return fullPath, nil
	}

	data, err := sm.client.Download(context.Background(), downloadable)
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", err
	}
	return fullPath, nil
}

func downloadableFromMessage(msg Message) (whatsmeow.DownloadableMessage, error) {
	if msg.RawMessage == nil {
		return nil, errors.New("This is not a downloadable message")
	}
	switch msg.Kind {
	case MessageKindImage:
		if media := msg.RawMessage.GetImageMessage(); media != nil {
			return media, nil
		}
	case MessageKindVideo:
		if media := msg.RawMessage.GetVideoMessage(); media != nil {
			return media, nil
		}
	case MessageKindAudio:
		if media := msg.RawMessage.GetAudioMessage(); media != nil {
			return media, nil
		}
	case MessageKindDocument:
		if media := msg.RawMessage.GetDocumentMessage(); media != nil {
			return media, nil
		}
	}
	return nil, errors.New("This is not a downloadable message")
}

func readUploadFile(path string) ([]byte, string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", "", err
	}
	fileName := filepath.Base(path)
	mimeType := detectMimeType(path, data)
	return data, mimeType, fileName, nil
}

func detectMimeType(path string, data []byte) string {
	if len(data) == 0 {
		if extType := mime.TypeByExtension(filepath.Ext(path)); extType != "" {
			return stripMimeParams(extType)
		}
		return "application/octet-stream"
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	detected := stripMimeParams(http.DetectContentType(sample))
	if extType := mime.TypeByExtension(filepath.Ext(path)); extType != "" {
		extType = stripMimeParams(extType)
		if detected == "application/octet-stream" || strings.HasPrefix(extType, "audio/") || strings.HasPrefix(extType, "video/") {
			return extType
		}
	}
	return detected
}

func stripMimeParams(value string) string {
	if idx := strings.Index(value, ";"); idx >= 0 {
		return value[:idx]
	}
	return value
}

func downloadFileName(msg Message) string {
	if msg.FileName != "" {
		safeName := path.Base(strings.ReplaceAll(msg.FileName, "\\", "/"))
		if safeName != "" && safeName != "." && safeName != ".." {
			return safeName
		}
	}
	ext := ""
	if msg.MimeType != "" {
		if exts, err := mime.ExtensionsByType(msg.MimeType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}
	return msg.Id + ext
}

func uploadMediaType(kind MessageKind) whatsmeow.MediaType {
	switch kind {
	case MessageKindImage:
		return whatsmeow.MediaImage
	case MessageKindVideo:
		return whatsmeow.MediaVideo
	case MessageKindAudio:
		return whatsmeow.MediaAudio
	default:
		return whatsmeow.MediaDocument
	}
}

func commandNameForKind(kind MessageKind) string {
	switch kind {
	case MessageKindImage:
		return "sendimage"
	case MessageKindVideo:
		return "sendvideo"
	case MessageKindAudio:
		return "sendaudio"
	default:
		return "upload"
	}
}

func mediaDisplayText(kind MessageKind, fileName, caption string) string {
	label := "[arquivo]"
	switch kind {
	case MessageKindImage:
		label = "[imagem]"
	case MessageKindVideo:
		label = "[vídeo]"
	case MessageKindAudio:
		label = "[áudio]"
	case MessageKindDocument:
		label = "[documento]"
	}
	parts := []string{label}
	if fileName != "" && kind == MessageKindDocument {
		parts = append(parts, fileName)
	}
	if caption != "" {
		parts = append(parts, caption)
	}
	return strings.Join(parts, " ")
}
