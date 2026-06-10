# CLAUDE.md

ZapTerm — cliente de WhatsApp para terminal em Go (tview + tcell + whatsmeow).
Visual minimalista estilo lazyvim, textos de UI em pt-BR.

## Comandos

```sh
make build       # compilar (CGO obrigatório — go-sqlite3)
make run         # rodar (precisa de terminal interativo; login por QR code)
go test ./...    # testes
gofmt -l .       # formato (deve sair vazio)
make dist        # pacotes Linux/Windows em dist/
make release     # tag + workflow de release (GitHub Actions)
```

`go vet` reporta avisos pré-existentes de `messages.Command{...}` sem chaves —
é o idioma do projeto; ignorar esses, não introduzir avisos novos.

## Mapa do projeto

| Onde | O quê |
| --- | --- |
| `main.go` | UI: layout, painéis, keybindings, render de mensagens, `UiHandler` |
| `finder.go` | Busca de conversas (Ctrl+f) |
| `images.go` | Imagens inline em half-block |
| `audio.go` | Player de áudio de mensagens de voz |
| `messages/` | Tudo de WhatsApp: conexão, eventos, comandos, storage, bot IA |
| `config/` | INI do usuário (`~/.config/whatscli/whatscli.config`) e defaults |
| `packaging/`, `assets/` | Scripts de instalação e ícones usados pelo `make dist` |

## Arquitetura em 5 linhas

- `main` (UI) e `messages` (WhatsApp) são isolados: UI envia
  `messages.Command` pelo `CommandChannel`; o manager responde pela interface
  `UiMessageHandler` — cujas implementações em `main.go` envolvem tudo em
  `go app.QueueUpdateDraw(...)`.
- O `SessionManager` roda em goroutine própria; storage de mensagens é em
  memória (`messages/storage.go`); a sessão persiste no SQLite do whatsmeow.
- Comando novo = `case` em `execCommand` (`messages/session_manager.go`).
- Nunca chamar `app.GetFocus()` em código que roda durante o draw (deadlock).
- `var VERSION string = "vX.Y.Z"` em `main.go` é parseada por sed nos scripts
  de release — não mudar o formato.

## Regras e skills

- Regras fixas: `.claude/rules/architecture.md` e `.claude/rules/code-style.md`.
- Detalhe de cada módulo: skills em `.claude/skills/`
  (`ui-module`, `whatsapp-core`, `config-settings`, `build-release`).

## Convenções rápidas

- UI em pt-BR; tema matrix verde-sobre-preto, estética de terminal: tags em
  colchetes (`[ONLINE]`, `[3]`), mensagens IRC `[15:04:05] <nome>`, prompt
  `você@zapterm:~$`. Sem emojis (marcadores: `#` grupo, `*` conversa aberta,
  `▶` tocando).
- Cores sempre via `config.Config.Colors` + `tcell.ColorNames`.
- Texto externo E colchetes literais passam por `tview.Escape()` antes de ir
  para TextViews (colchete sem escape vira tag de cor).
- Config nova = campo + default em `config/settings.go` (INI usa
  title_underscore); atalho novo = `Keymap` + `LoadShortcuts()` + ajuda F1.
- Código deve cross-compilar para Windows (`make dist`).
