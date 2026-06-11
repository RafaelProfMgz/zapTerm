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

## Telas

Quatro telas, seguindo os mockups de `design/`:

| Tecla | Tela | O quê |
| --- | --- | --- |
| `F1` | SESSÃO | conversas + stream de mensagens + prompt |
| `F2` | TÚNEL | mapa da conexão (host ⇄ WhatsApp) e estado da sessão |
| `F3` | LOGS | console de telemetria do núcleo Go + métricas |
| `F4` | CONFIG | arquivos, núcleo, tema e atalhos (somente leitura) |

## Teclas

| Tecla | Ação |
| --- | --- |
| `F1-F4` | troca de tela (fora da SESSÃO, `1-4` também funciona) |
| `Tab` | alterna painéis (conversas → mensagens → digitação) |
| `↑/↓` | navegar / rolar logs |
| `Enter` | abrir conversa / enviar mensagem / tocar áudio selecionado |
| `1-4` / `f` | filtros: Todas · Não lidas · Grupos · Contatos |
| `p` / `o` / `d` | tocar áudio · abrir anexo · baixar (mensagem selecionada) |
| `b` | carregar histórico (backlog) |
| `/comando` | qualquer comando do núcleo Go (`/connect`, `/read`, `/sendaudio`…) |
| `Ctrl+Q` | sair |

## Tema

`src/theme.mjs` — "Terminal Protocol" (`design/terminal_protocol/DESIGN.md`):
âmbar queimado sobre terra escura, retro-técnico calmo. Cor com parcimônia:
âmbar só em foco/prompt/destaques, ciano só em status de rede/sistema, texto
corrido em off-white quente, contornos terrosos discretos e cada contato
ganha um tom quente estável próprio, estilo IRC. Sem glow, sem emoji.
