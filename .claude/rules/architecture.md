# Arquitetura — regras fixas

O ZapTerm é um TUI em Go (tview + whatsmeow) com três pacotes. As fronteiras
abaixo são regras, não sugestões.

## Separação de pacotes

- **`main` (raiz)** — só UI: widgets tview, keybindings, renderização de
  mensagens, player de áudio, busca. Nunca importa `whatsmeow` diretamente.
- **`messages/`** — toda a lógica de WhatsApp: conexão, eventos, envio,
  download de mídia, armazenamento em memória, bot de IA. Nunca importa tview
  nem toca em widget; fala com a UI apenas pela interface `UiMessageHandler`
  (`messages/messages.go`).
- **`config/`** — leitura/gravação do INI e defaults. Não tem lógica de
  negócio nem de UI.

## Modelo de threads (a regra mais importante)

- A UI roda no event loop do tview. O `SessionManager` roda em goroutine
  própria (`runManager` em `messages/session_manager.go`), consumindo canais.
- **UI → manager**: sempre via `sessionManager.CommandChannel <-
  messages.Command{nome, params}`. Nunca chamar métodos do manager
  diretamente da UI.
- **Manager → UI**: sempre via métodos de `UiMessageHandler`. Toda
  implementação em `main.go` (struct `UiHandler`) que mexe em widget DEVE
  envolver o trabalho em `go app.QueueUpdateDraw(func() { ... })`.
- **Nunca** chamar `app.GetFocus()` dentro de `SetBeforeDrawFunc` (ou de
  qualquer código que rode durante o draw): o draw segura o write-lock e
  `GetFocus()` pega o read-lock → deadlock. Usar `primitive.HasFocus()`.
- Estado compartilhado fora do event loop (ex.: playback em `audio.go`)
  exige mutex próprio.

## Fluxo de comandos

Texto digitado com prefixo `/` (config `cmd_prefix`) vira `Command` em
`EnterCommand` (`main.go`) → `CommandChannel` → switch em `execCommand`
(`messages/session_manager.go`). Para criar um comando novo: adicionar `case`
em `execCommand` + entrada na ajuda (`buildHelpText` em `main.go`).

## Persistência

- Mensagens/chats ficam **em memória** (`MessageDatabase` em
  `messages/storage.go`, maps com RWMutex). Não há persistência própria —
  o histórico vem do history sync do whatsmeow.
- A sessão do WhatsApp persiste no SQLite do whatsmeow em
  `config.GetSessionFilePath() + ".db"` (`~/.config/whatscli/session.db`).
- Config do usuário: `~/.config/whatscli/whatscli.config` (INI).

## Versão

`var VERSION string = "vX.Y.Z"` em `main.go` é lida por `make-dist.sh` e
`release.sh` via `sed` — **não mudar o formato dessa linha**, só o número.
