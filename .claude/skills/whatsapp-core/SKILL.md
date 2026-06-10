---
name: whatsapp-core
description: Como funciona o pacote messages/ — conexão WhatsApp via whatsmeow, eventos, comandos, envio/download de mídia, storage em memória e bot de IA. Use ao mexer em qualquer lógica de WhatsApp, comandos ou mensagens.
---

# Núcleo WhatsApp (pacote `messages/`)

## Arquivos

| Arquivo | Responsabilidade |
| --- | --- |
| `messages.go` | Tipos (`Message`, `Chat`, `Contact`, `Command`), interface `UiMessageHandler`, sufixos JID |
| `session_manager.go` | Conexão whatsmeow, login QR, eventos, `execCommand`, envio e download de mídia |
| `storage.go` | `MessageDatabase` — storage em memória thread-safe |
| `bot.go` | Bot de auto-resposta via OpenAI (streaming), comandos `/ai`, `/end`, `/key` no chat |

## SessionManager

- `Init()` + `StartManager()` sobem a goroutine `runManager()`, que consome:
  - `CommandChannel` — comandos vindos da UI (`Command{Name, Params}`)
  - `ChatChannel` / `StatusChannel` — atualizações internas
- Conexão: `getConnection()` cria o `whatsmeow.Client` com device store
  SQLite (`config.GetSessionFilePath()+".db"`). Login por QR:
  `loginWithQRCode()`.
- Eventos: `eventHandler.Handle()` — `events.Message` →
  `handleLiveMessage()`; `events.HistorySync` → `handleHistorySync()`.
  Ambos normalizam via `normalizeEventMessage` → `messageFromInfo`
  (é AQUI que se extrai novo metadado de mensagem: mime, caption,
  duração `DurationSecs`, forwarded…).
- **Comandos**: switch em `execCommand()` (~linha 384). Casos atuais:
  send, select, read, backlog, download/open/show/play, url, upload,
  sendimage/sendvideo/sendaudio, revoke, grupos (create/add/remove/admin/
  subject/leave), login/logout/reset/disconnect.

## Mídia

- Download: `downloadCommand()` → `downloadMessage()` — baixa para
  `download_path` (ou `preview_path`), cacheado por nome de arquivo
  (`downloadFileName`: FileName original ou msgId+ext do MIME).
- `open` = baixa e abre no sistema (`uiHandler.OpenFile`);
  `show` = imagem inline (`uiHandler.PrintFile`);
  `play` = áudio/vídeo no player do terminal (`uiHandler.PlayFile`).
- Envio: `sendMediaCommand()` → `sendMedia()` — upload via
  `uploadMediaType()`, monta o proto por tipo.
- Labels de mídia em pt-BR: `mediaDisplayText()` (`[áudio]`, `[imagem]`…).

## Storage (`storage.go`)

- Maps protegidos por RWMutex: `messages[chatId][]Message` (ordenadas por
  timestamp), `messagesById`, `chats`, `contacts`.
- `AddMessage` deduplica por id e faz "upgrade" de metadados; atualiza
  `Chat.LastMessage`/`Unread`.
- `GetChatIds()` retorna chats **ordenados por mensagem mais recente** —
  é a ordem que a sidebar exibe; não reordenar na UI.
- Volátil: nada de mensagem é persistido localmente (histórico vem do sync).

## Identificadores (JIDs)

- Grupo: termina em `@g.us` (`GROUPSUFFIX`); contato: `@s.whatsapp.net`
  (`CONTACTSUFFIX`); status: `status@broadcast` (ignorado).
- `Chat.IsGroup` deriva do sufixo. `rawJid()` (em `finder.go`) remove o
  sufixo para exibir o número.

## Bot de IA (`bot.go`)

- Opcional, config na seção `[bot]` (enabled, chat_id, trigger_prefix,
  model…). Chave padrão em `OPENAI_API_KEY` (carregada de `.env` via
  `config/dotenv.go`).
- Responde apenas no `chat_id` configurado; streaming reescreve a própria
  mensagem via `MessageDatabase.UpdateMessageText`.

## Como adicionar um comando novo

1. `case "nome":` em `execCommand` chamando um método `sm.algo(params)`.
2. Erros/feedback via `sm.uiHandler.PrintError/PrintText` (nunca tocar UI).
3. Se a UI precisa reagir com algo novo, adicionar método à interface
   `UiMessageHandler` (messages.go) e implementar em `UiHandler` (main.go)
   com `go app.QueueUpdateDraw`.
4. Documentar em `buildHelpText()` (main.go).
