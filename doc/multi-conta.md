# Plano: mais de uma conta do WhatsApp no ZapTerm

Objetivo: conectar várias contas ao mesmo tempo, escolher em qual conversar por
uma barra lateral à esquerda e parear cada conta nova por QR code — sem perder a
sessão de quem já usa o app hoje.

Este documento é o plano acordado **depois** da correção de reconexão (QR/
`/reconectar`/botões da tela CONFIG), que já está no código. Cada fase abaixo é
entregável sozinha e mantém o app funcionando.

## Onde o código está amarrado a uma conta só

| Ponto | Hoje |
| --- | --- |
| `config.GetSessionFilePath()` | um único `~/.config/whatscli/session.db` |
| `config.GetCacheFilePath()` | um único `~/.config/whatscli/cache.json` |
| QR em `waitForQRCode()` | um único `whatscli-qr.png` |
| `SessionManager` | instância única criada em `main.go`/`jsonui.go` |
| `UiMessageHandler` | métodos sem identificação de conta |
| Protocolo NDJSON | eventos sem campo de conta |
| Ink `app.mjs` | `chats`, `msgs`, `status`, `qr`, `currentChat` são estado plano |

O `SessionManager` já guarda tudo o que precisa em campos próprios (db, client,
canais, estado do bot), então **várias instâncias convivem** assim que os
caminhos de arquivo e o roteamento de eventos deixarem de ser globais.

## Fase 1 — caminhos por conta + registro (sem mudança visível) — FEITA

1. `config/accounts.go`: registro em `~/.config/whatscli/accounts.json`
   ```json
   {"active":"default","accounts":[{"id":"default","label":"Pessoal","jid":"5511...@s.whatsapp.net"}]}
   ```
   `id` é gerado (slug curto), `label` é editável pelo usuário, `jid` é
   preenchido depois do pareamento (`client.Store.ID`).
2. Novos helpers `config.AccountDir(id)`, `GetSessionFilePathFor(id)`,
   `GetCacheFilePathFor(id)`, `GetQRFilePathFor(id)` → tudo dentro de
   `~/.config/whatscli/accounts/<id>/`.
3. **Migração**: na primeira execução, se existir `session.db`/`cache.json`
   antigos na raiz, mover para `accounts/default/` e criar o registro. Ninguém
   precisa ler QR de novo por causa da atualização.
4. `SessionManager` ganha o campo `AccountID` e troca as chamadas de
   `config.GetSessionFilePath()`/`GetCacheFilePath()` pelas versões com id.

Teste: subir o binário com um `XDG_CONFIG_HOME` contendo a estrutura antiga e
conferir que a sessão foi migrada e continua conectando.

## Fase 2 — `AccountManager` + campo `account` no protocolo — FEITA

1. `messages/accounts.go`:
   ```go
   type AccountManager struct {
       managers map[string]*SessionManager
       order    []string
       active   string
       ui       UiAccountHandler
   }
   ```
   com `Add(label) (id, error)`, `Remove(id)`, `SetActive(id)`, `Send(id, Command)`
   e `StartAll()`.
2. Cada `SessionManager` recebe um decorador que implementa o
   `UiMessageHandler` atual e repassa para uma interface nova com o id junto
   (`NewMessage(accountID string, msg Message)` etc.). Assim **o
   `SessionManager` não muda** — só ganha um handler que carimba a origem.
3. JSON (`jsonui.go`): todo evento passa a levar `"account":"<id>"`; comandos
   aceitam `{"cmd":"...","account":"<id>","params":[...]}` e, sem `account`,
   vão para a conta ativa. Dois eventos novos: `accounts` (lista completa com
   label/jid/estado) e `account` (troca de conta ativa).
4. Comandos: `/contas`, `/conta <id|número>`, `/conta nova [label]`,
   `/conta remover <id>`, `/conta renomear <id> <label>`. `/novoqr` e
   `/reconectar` passam a agir na conta ativa (ou na que vier no comando).
5. `main.go` (tview) continua usando **só a conta ativa** — a barra lateral de
   contas é exclusiva do Ink nesta fase.

Teste: `fake-core.mjs` ganha duas contas e o teste do bridge confere que um
`select` com `account` só mexe naquela conta.

Como ficou (difere do rascunho acima em dois pontos):

- Em vez de uma interface com todos os métodos duplicados com `accountID`, o
  `UiAccountHandler` tem `ForAccount(id) UiMessageHandler`: o JSON devolve um
  handler filho que carimba `"account"`; o tview devolve um filtro que só
  repassa a conta ativa (`accounts_ui.go`). O decorador `accountHandler`
  guarda status/QR por conta para reenviar na troca.
- O tview também passou a usar o `AccountManager` (troca por `/conta`, sem
  barra lateral), e o Ink, até a Fase 3, descarta eventos das contas
  inativas — a troca zera o estado e o núcleo reenvia tudo (`__resync`).
- O limite de 5 contas (`config.MaxAccounts`) já entrou aqui.

## Fase 3 — barra lateral de contas no Ink

1. Estado por conta em `app.mjs`: trocar `chats`/`msgs`/`status`/`qr`/
   `currentChat` por `byAccount[id] = {...}` + `activeId`; a renderização lê
   `byAccount[activeId]`. É a maior refatoração do frontend — vale extrair um
   `useAccounts(bridge)` (hook) para concentrar o merge de eventos.
2. Componente `accounts.mjs` — trilho vertical de ~16 colunas à esquerda do
   painel de conversas, no estilo do resto (sem emoji):
   ```
   [ CONTAS ]
   * 1 Pessoal
       [ONLINE]
     2 Trabalho
       [3] [ONLINE]
     3 Loja
       [SEM SESSÃO]
   + nova conta
   ```
   `*` = conta ativa, `[n]` = não lidas somadas, tag de estado reaproveitando
   `[ONLINE]`/`[CONECTANDO]`/`[SEM SESSÃO]`.
3. Navegação: `Alt+1..9` troca direto, `Ctrl+↑/↓` anda na lista, clique do
   mouse troca (mapear em `mouseRef`, como `filterTabAt`), `+` abre a conta
   nova (pede label e já mostra o QR daquela conta).
4. A tela de QR (`qr.mjs`) passa a mostrar de qual conta é o código; as outras
   contas continuam recebendo mensagens enquanto o QR está aberto.
5. Notificação de sistema e bell prefixam o label da conta quando há mais de
   uma conectada.

## Fase 4 — acabamento

- Ajuda (`buildHelpText` no tview e o cartão ATALHOS no Ink) com os comandos
  `/conta*`.
- README: seção "Várias contas".
- Limite prático (ex.: 5 contas) — cada conta é um websocket, uma goroutine e
  um `MessageDatabase` em memória; história sincronizada multiplica a RAM.
- `config.Bot.ChatId` hoje é global: qualificar por conta (`conta:chatid`) ou
  documentar que o bot roda só na conta ativa.

## Decisões já tomadas

- **Um SQLite por conta** (`accounts/<id>/session.db`) em vez de vários devices
  no mesmo container: apagar/resetar uma conta vira remover uma pasta, e não há
  contenção de escrita entre contas.
- **Sem conta "global"**: a conta ativa decide o que o painel de mensagens e os
  comandos sem `account` fazem.
- **tview fica em uma conta** até a Fase 4; o Ink é a interface padrão desde a
  v2.0.0.
