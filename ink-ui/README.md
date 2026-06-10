# ZapTerm Ink UI (experimental)

Frontend visual do ZapTerm escrito em [Ink](https://github.com/vadimdemedes/ink)
(React para terminal) + [Ink UI](https://github.com/vadimdemedes/ink-ui).

**O Go continua sendo a raiz e o cérebro de tudo**: sessão WhatsApp, storage,
comandos, download de mídia e reprodução de áudio rodam no binário Go. Este
app só desenha a interface e devolve comandos.

## Arquitetura

```
┌────────────────────┐   NDJSON (stdout)    ┌──────────────────┐
│  zapterm (Go)      │ ───────────────────▶ │  ink-ui (Node)   │
│  --ui=json         │   eventos: chats,    │  só renderiza    │
│  cérebro: sessão,  │   screen, message,   │                  │
│  storage, mídia,   │   status, playing…   │                  │
│  áudio             │ ◀─────────────────── │                  │
└────────────────────┘   NDJSON (stdin)     └──────────────────┘
                          comandos: select,
                          send, play, open…
```

- Lado Go: `jsonui.go` (handler `UiMessageHandler` headless) — flag `--ui=json`.
- Lado Node: `src/bridge.mjs` sobe o binário e troca NDJSON; `src/app.mjs`
  monta a interface.

## Como rodar

```sh
# na raiz do repo: compilar o cérebro
go build

# aqui dentro: instalar e rodar
cd ink-ui
npm install
npm start          # ou: ZAPTERM_BIN=/caminho/zapterm npm start
```

## Teclas

| Tecla | Ação |
| --- | --- |
| `Tab` | alterna painéis (conversas → mensagens → digitação) |
| `↑/↓` | navegar |
| `Enter` | abrir conversa / enviar mensagem / tocar áudio selecionado |
| `1-4` / `f` | filtros: Todas · Não lidas · Grupos · Contatos |
| `p` / `o` / `d` | tocar áudio · abrir anexo · baixar (mensagem selecionada) |
| `b` | carregar histórico (backlog) |
| `/comando` | qualquer comando do núcleo Go (`/connect`, `/read`, `/sendaudio`…) |
| `Ctrl+Q` | sair |

## Tema

`src/theme.mjs` — mistura de verdes com cinza clay (nada de "tudo verde"):
texto corrido em cinza claro, cromo (bordas/horários/dicas) em clay, verde
brilhante apenas em destaques (foco, não lidas, reprodução) e cada contato
ganha um tom de verde estável próprio, estilo IRC.
