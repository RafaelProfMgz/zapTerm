---
name: ui-module
description: Como funciona o módulo de UI (pacote main) — layout tview, lista de conversas com filtros, renderização de mensagens, keybindings, finder, imagens inline e player de áudio. Use ao mexer em qualquer coisa visual ou de interação.
---

# Módulo de UI (pacote `main`, raiz do repo)

## Arquivos

| Arquivo | Responsabilidade |
| --- | --- |
| `main.go` | Layout, painéis, keybindings, renderização de mensagens, ajuda, `UiHandler` |
| `finder.go` | Busca de conversas estilo Telescope (Ctrl+f), fuzzy match |
| `images.go` | Render de imagem inline em half-block (▀) p/ terminais com cor |
| `audio.go` | Player de áudio (mpv/ffplay/play/cvlc), toggle tocar/parar |

## Layout (main.go, função `main`)

Grid 4 linhas × 2 colunas: `topBar`+`infoBar` (linha 0), `treeView` (conversas,
coluna esquerda, largura `ui.chat_sidebar_width`) + `textView` (mensagens) na
linha 1, `textInput` (linha 2), `hintBar` (linha 3). Tudo dentro de
`pages` (tview.Pages) com overlays "help" e "find".

## Lista de conversas (sidebar)

- `MakeTree()` cria o TreeView **plano** (root oculto via `SetTopLevel(1)`,
  `SetGraphics(false)`) — lê como lista, não árvore.
- `rebuildChatTree()` repopula a partir de `allChats` (mantida por
  `UiHandler.SetChats`, já ordenada por recência pelo storage).
- Filtros estilo abas: consts `chatFilterAll/Unread/Groups/Contacts` +
  `chatFilter` global. Teclas `1-4` e `f` (registradas em `keysChatPanel`
  dentro de `LoadShortcuts`). Filtro "Todas" só mostra chats com atividade
  (`LastMessage > 0`), exceto antes do primeiro sync.
- Formato da linha: `formatChatEntry()` — `* ` marca a conversa aberta
  (atualizado por `refreshChatMarkers()` sem rebuild), nome truncado à
  esquerda (runewidth), à direita `[N]` (não lidas) ou horário curto
  (`shortChatTime`: hoje→`15:04`, semana→`seg`, antigo→`02/01`).
  Grupos têm prefixo `# `. Sem emojis (regra do projeto).
- Navegar (changed) abre a conversa; Enter (selected) abre e foca o input.

## Renderização de mensagens

- `getMessagesString()` monta o chat inteiro; insere separador de data
  (`dateSeparator`) quando o dia muda. `UiHandler.NewMessage` (mensagem ao
  vivo) faz a mesma checagem comparando com a última de `curRegions`.
- `getTextMessageString()` formata uma mensagem estilo IRC
  `[15:04:05] <nome> texto`: região clicável `["msgId"]...[""]` (tview
  regions), `<eu>` para mensagens próprias, dica de mídia (`mediaHint`:
  teclas em colchetes + duração de áudio/vídeo) e marcador `▶ TOCANDO`
  quando `msg.Id == currentPlayingMsgId()`. Colchetes literais passam por
  `tview.Escape`.
- `curRegions []messages.Message` é a lista de mensagens na tela — paralela
  às regiões do textView; toda navegação/seleção usa ela.
- Clique em mensagem: capturado em `SetHighlightedFunc` — áudio envia
  comando `play`, demais anexos `open`.
- Re-render seguro de qualquer goroutine: `queueRefreshChat()` →
  `redrawChat()`.

## Áudio (audio.go)

- Estado global com mutex: `audioCmd`, `playingMsgId`.
- `playAudioFile(path, msgId)`: toggle — tocar a mesma mensagem para; outra
  mensagem substitui. Goroutine `cmd.Wait()` limpa o estado e re-renderiza.
- Player: config `audio_command` ou auto-detecção em
  `audioPlayerCandidates` (mpv → ffplay → play/sox → cvlc).
- Fluxo completo: tecla `p`/clique → comando `play` → session manager baixa
  o arquivo → `UiHandler.PlayFile` → `playAudioFile`.

## Keybindings

- Tudo em `LoadShortcuts()` via cbind, com 3 escopos: `keyBindings`
  (globais), `keysMessages` (painel de mensagens: j/k, d/o/s/p/u/i/r…),
  `keysChatPanel` (lista: 1-4, f).
- Teclas configuráveis vêm de `config.Config.Keymap`. Para adicionar uma:
  campo na struct `Keymap` + default + `keyBindings.Set(...)` + ajuda.
- O finder, quando aberto (`finderVisible`), engole todas as teclas globais.

## Imagens inline (images.go)

- `canRenderInlineImages()` checa COLORTERM/terminal; `renderInlineImage()`
  gera blocos `[#RRGGBB:#RRGGBB]▀`; cache em `inlineImages[msgId]`, embutido
  dentro da região da mensagem. Fallback: `PrintImage` (jp2a) ou visualizador
  do sistema.
