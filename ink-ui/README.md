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

Cinco telas, seguindo os mockups de `design/`:

| Tecla | Tela | O quê |
| --- | --- | --- |
| `F1` | SESSÃO | conversas + stream de mensagens + prompt |
| `F2` | STORIES | status (`status@broadcast`) agrupados por contato, separados das conversas |
| `F3` | TÚNEL | mapa da conexão (host ⇄ WhatsApp) e estado da sessão |
| `F4` | LOGS | console de telemetria do núcleo Go + métricas |
| `F5` | CONFIG | arquivos, núcleo, tema e atalhos (somente leitura) |

## Teclas

| Tecla | Ação |
| --- | --- |
| `F1-F5` | troca de tela (fora da SESSÃO, `1-5` também funciona) |
| `Tab` | alterna painéis (conversas → mensagens → digitação) |
| `↑/↓` | navegar / rolar logs |
| `Enter` | abrir conversa / enviar mensagem / tocar áudio selecionado |
| `Ctrl+F` | buscador de contatos (nome ou número; `Tab` alterna Todos · Contatos · Grupos) |
| `1-4` / `f` | filtros: Todas · Não lidas · Grupos · Contatos |
| `p` / `o` / `d` | tocar áudio · abrir anexo · baixar (mensagem selecionada) |
| `b` | carregar histórico (backlog) |
| `/comando` | qualquer comando do núcleo Go (`/connect`, `/read`, `/sendaudio`…) |
| `Ctrl+Q` | sair |

## Mouse

A interface também responde ao mouse (protocolo SGR do terminal, habilitado
em `src/mouse.mjs` — as sequências são filtradas do stdin antes de chegarem
ao Ink):

| Ação | O quê |
| --- | --- |
| clique nas abas (cabeçalho ou taskbar `[F1]…[F5]`) | troca de tela |
| clique numa conversa | abre a conversa e foca a digitação |
| clique nas abas de filtro | aplica o filtro (Todas · Não lidas · Grupos · Contatos) |
| clique no painel de mensagens / no prompt | move o foco |
| rolagem na lista de conversas | navega a seleção |
| rolagem nas mensagens | navega/seleciona mensagens (como `↑/↓`) |
| rolagem na tela LOGS | rola o console |

Para selecionar texto com o mouse no terminal, segure `Shift` ao arrastar
(padrão dos terminais quando o rastreio de mouse está ativo).

## Lista de conversas

As conversas com histórico vêm primeiro, ordenadas da última conversada para
a mais antiga; abaixo da linha `── SEM_HISTÓRICO ──` ficam os contatos sem
conversa, em ordem alfabética.

## Stories (status@broadcast)

Os status do WhatsApp não se misturam mais com as conversas: o núcleo Go agrupa
`status@broadcast` por autor e manda no evento `stories` (já com as mensagens).
A tela `F2 STORIES` mostra os autores à esquerda (● = status não vistos) e os
posts do autor selecionado à direita. `↑/↓` ou rolagem navegam os contatos.

## Cache local

As conversas e os últimos ~100 recados de cada chat ficam num JSON leve em
`~/.config/whatscli/cache.json` (gravação com *debounce* de 2s + flush ao sair).
Assim a lista de conversas e os stories já aparecem na abertura, antes mesmo de
o WhatsApp sincronizar. O cache não guarda o proto bruto das mensagens (download
de mídia só funciona online). Lado Go: `messages/cache.go`.

## Testes

```sh
cd ink-ui
npm test            # node --test: unidade (funções puras) + e2e do bridge
```

Os testes e2e sobem um núcleo falso (`test-fixtures/fake-core.mjs`) que fala o
mesmo protocolo NDJSON, então não tocam no WhatsApp. No lado Go, `go test ./...`
cobre o cache (round-trip) e a separação dos stories.

## Tema

`src/theme.mjs` — "Terminal Protocol" (`design/terminal_protocol/DESIGN.md`):
âmbar queimado sobre terra escura, retro-técnico calmo. Cor com parcimônia:
âmbar só em foco/prompt/destaques, ciano só em status de rede/sistema, texto
corrido em off-white quente, contornos terrosos discretos e cada contato
ganha um tom quente estável próprio, estilo IRC. Sem glow, sem emoji.
