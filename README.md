# ZapTerm

**Um cliente de WhatsApp para o terminal** — envie e receba mensagens sem abrir o navegador, direto na linha de comando.

> 🎓 **Projeto universitário.** O ZapTerm é uma adaptação/estudo construído sobre o
> projeto open-source [whatscli](https://github.com/normen/whatscli), usando as
> bibliotecas [whatsmeow](https://github.com/tulir/whatsmeow) (conexão com o WhatsApp)
> e [tview](https://github.com/rivo/tview) (interface no terminal).

![screenshot](/doc/screenshot.png?raw=true "ZapTerm")

## O que ele faz

- Envia e recebe mensagens de WhatsApp dentro de um app de terminal
- Conecta pela API do WhatsApp Web, sem precisar de navegador
- Login simples por QR Code
- Lista de conversas estilo WhatsApp Web: ordenada por atividade, com horário da
  última mensagem e contador de não lidas; filtros **1-4** (Todas · Não lidas ·
  Grupos · Contatos)
- **Toca áudios no terminal** (clique ou tecla `p`) — usa mpv/ffplay/sox/vlc,
  configurável via `audio_command`
- Separadores de data no histórico, visual minimalista sem ícones (estilo lazyvim)
- Baixa e abre anexos (imagem, vídeo, áudio, documento)
- Envia imagens, vídeos, áudios e documentos
- Gerenciamento básico de grupos
- Notificações no desktop
- Cores personalizáveis

### Limitações (importante saber)

- O histórico depende do que o WhatsApp sincroniza com aparelhos conectados; às vezes é preciso usar `/backlog`.
- A Meta não endossa apps assim — eles podem parar de funcionar quando o WhatsApp muda o app web.

---

## Como executar (passo a passo)

Há duas formas. Escolha a que combina com você.

### ✅ Opção A — Usar o programa pronto (NÃO precisa do Go)

Esta é a forma mais simples: baixe a pasta pronta para o seu sistema e use. Não precisa
instalar Go, compilador, nada de programação.

#### Windows

A pasta `ZapTerm-windows/` contém:

| Arquivo | Para que serve |
| --- | --- |
| `ZapTerm.exe` | O programa (já vem com ícone) |
| `Instalar-no-menu-iniciar.bat` | Cria o atalho **ZapTerm** no Menu Iniciar |
| `Abrir-ZapTerm.bat` | Abre o programa na hora, sem instalar |

1. Dê dois cliques em **`Instalar-no-menu-iniciar.bat`**.
2. Abra o **Menu Iniciar** e procure por **ZapTerm**.
3. Clique no atalho — abre uma janela de terminal com o programa.
4. **Escaneie o QR Code** com o WhatsApp do celular
   (*WhatsApp → Aparelhos conectados → Conectar um aparelho*).

> Só quer testar sem instalar? Dê dois cliques em `Abrir-ZapTerm.bat`.
> Na primeira execução o Windows pode mostrar o aviso do SmartScreen: clique em
> **Mais informações → Executar assim mesmo**.

#### Linux

A pasta `ZapTerm-linux/` contém:

| Arquivo | Para que serve |
| --- | --- |
| `zapterm` | O programa já compilado |
| `Instalar-no-menu.sh` | Coloca o **ZapTerm** no menu de aplicativos (tela inicial) |
| `Abrir-ZapTerm.sh` | Lançador inteligente: abre o programa em uma janela de terminal |

1. Abra um terminal na pasta e rode:
   ```sh
   bash Instalar-no-menu.sh
   ```
2. Abra o menu de aplicativos e procure por **ZapTerm**.
3. Clique no ícone — abre um terminal com o programa.
4. **Escaneie o QR Code** com o WhatsApp do celular
   (*WhatsApp → Aparelhos conectados → Conectar um aparelho*).

> Só quer testar sem instalar? Rode `bash Abrir-ZapTerm.sh`.

> **Como a janela de terminal é aberta (sem depender da sua configuração):**
> em vez do tradicional `Terminal=true` do `.desktop` — que depende do
> `x-terminal-emulator` estar configurado e às vezes falha — o atalho chama um
> **lançador inteligente** (`Abrir-ZapTerm.sh`) que detecta um terminal instalado
> (`kitty`, `gnome-terminal`, `konsole`, `alacritty`, `wezterm`, `xfce4-terminal`,
> `tilix`, `xterm`...) e abre o ZapTerm dentro dele. Se você rodar o lançador de
> dentro de um terminal, ele executa direto, sem abrir outra janela.

---

### 🛠️ Opção B — Compilar a partir do código (precisa do Go)

Use se quiser a versão mais recente ou contribuir com o projeto.

1. **Instale o Go** (versão 1.25 ou mais nova) em https://go.dev/dl e confira:
   ```sh
   go version
   ```
2. **Instale o Git** (https://git-scm.com/downloads).
3. **Clone e entre na pasta**:
   ```sh
   git clone <url-do-repositorio> zapterm
   cd zapterm
   ```
4. **Compile e rode** (para o seu próprio sistema):
   ```sh
   go run .          # roda direto
   # ou
   go build          # gera o executável e depois rode-o
   ```
   Também há um `Makefile`: `make run` ou `make build`.

> **Observação técnica:** o ZapTerm usa `go-sqlite3`, que depende de **CGO**, então é
> necessário ter um compilador C (no Linux, o `gcc`).

#### Gerar o executável do Windows (`ZapTerm.exe`) com ícone

O ícone é embutido via um arquivo de recurso `zapterm_windows_amd64.syso` (gerado a
partir de `assets/zapterm.ico`). Para regenerar os artefatos e compilar para Windows
a partir do Linux, é preciso o cross-compilador **mingw**:

```sh
# 1. compilador C para Windows (Debian/Ubuntu)
sudo apt-get install -y gcc-mingw-w64-x86-64

# 2. (opcional) regenerar ícone e recurso embutido
python3 assets/make_icon.py     # gera assets/zapterm.ico e .png
python3 assets/make_syso.py     # gera zapterm_windows_amd64.syso

# 3. compilar o .exe com o ícone embutido
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -o dist/ZapTerm-windows/ZapTerm.exe .
```

---

## Usando o ZapTerm

A ajuda completa, comandos e atalhos estão dentro do app: digite `/help` (ou tecle `F1`).

### Login

Ao iniciar, o ZapTerm tenta conectar e mostra um **QR Code** desenhado no terminal
(também salvo como imagem em `~/.config/whatscli/accounts/default/whatscli-qr.png`). Escaneie com o
WhatsApp do celular em *Aparelhos conectados > Conectar aparelho*. Se o QR não couber na
tela, diminua a fonte do terminal ou aumente a janela. Depois da primeira vez, ele
reconecta sozinho.

### Reconectar / trocar de conta

Quando a sessão cai ou é encerrada pelo celular, o rodapé mostra `[SEM SESSÃO]` e o QR
volta sozinho (desligue com `auto_reconnect = false` no `whatscli.config`). Para
reconectar na mão:

| Como | O quê |
| --- | --- |
| `Ctrl+R` (em qualquer tela) | reconectar com a sessão atual |
| Tela `[F5] CONFIG` | botões `[R] RECONECTAR`, `[N] NOVO_QR`, `[D] DESCONECTAR`, `[L] SAIR_DA_CONTA`, `[Z] RESETAR_SESSAO` (clique ou tecla) |
| `/reconectar` (ou `/re-conect`, `/connect`) | reconectar com a sessão atual |
| `/novoqr` | apagar a sessão e ler um QR novo |
| `/cancelqr` | cancelar a leitura do QR em andamento |
| `/openqr` | abrir a imagem do QR no visualizador do sistema |
| `/logout` | sair da conta neste computador |

Na tela do QR: `[O]` abre a imagem, `[N]` gera outro código, `[C]` cancela e `[ESC]`
esconde sem cancelar.

O código de pareamento do WhatsApp é grande (matriz de ~73x73 módulos), então desenhá-lo
no terminal exige uma janela de **~44 linhas**. Em janelas menores o ZapTerm abre sozinho
a imagem `~/.config/whatscli/accounts/default/whatscli-qr.png` — ou diminua a fonte do terminal (`Ctrl+-`)
e amplie a janela para lê-lo direto na tela.

### Várias contas

O ZapTerm conecta até **5 contas do WhatsApp ao mesmo tempo**. Cada uma tem a
sua sessão e o seu QR, e todas continuam recebendo mensagens enquanto você
conversa em outra.

| Como | O quê |
| --- | --- |
| `/conta nova <nome>` | adicionar uma conta — o QR dela aparece em seguida |
| `/contas` | listar as contas (`*` = ativa) com número e estado |
| `/conta <n>` ou `/conta <id>` | trocar a conta ativa |
| `/conta renomear <n> <nome>` | renomear uma conta |
| `/conta remover <n>` | desconectar no celular e apagar a conta deste computador |
| `Alt+1`…`Alt+9` · `Ctrl+↑/↓` | trocar de conta (interface padrão) |
| `+` ou clique em `+ nova conta` | adicionar uma conta (interface padrão) |

Na interface padrão, a barra `[ CONTAS ]` à esquerda mostra cada conta com as
não lidas somadas (`[3]`) e o estado (`[ONLINE]`, `[CONECTANDO]`, `[SEM SESSÃO]`);
clique numa conta para trocar. Os comandos sem conta (como `/novoqr` e
`/reconectar`) agem na conta ativa. A notificação de sistema mostra o nome da
conta quando há mais de uma conectada.

Cada conta fica em `~/.config/whatscli/accounts/<id>/` (sessão, cache e imagem
do QR); a lista está em `~/.config/whatscli/accounts.json`. Quem já usava o
ZapTerm antes tem a sessão movida para `accounts/default/` automaticamente —
não precisa ler o QR de novo.

Cada conta mantém as suas conversas em memória, então muitas contas com
bastante histórico usam mais RAM.

Bot de IA com várias contas: o `chat_id` da seção `[bot]` roda só na
**primeira conta** da lista. Para escolher outra, prefixe o id da conta:
`chat_id = trabalho:120363...@g.us`.

### Diagnóstico de conexão

Se o QR não aparece ou a conexão falha, rode com `ZAPTERM_DEBUG=1` (ou `=debug` para o
rastreio completo do protocolo): os logs da biblioteca do WhatsApp saem no **stderr**, sem
atrapalhar a interface. É assim que se vê, por exemplo, um `Client outdated (405)` — que
significa que a biblioteca `whatsmeow` precisa ser atualizada.

### Mensagens e comandos

Selecione uma conversa à esquerda e digite no campo de baixo para enviar. Use `Tab` para
alternar entre a lista de conversas e o campo de digitação. Comandos usam o prefixo `/`
(ex.: `/sendimage /caminho/para/foto.jpg`). Em caminhos, não precisa aspas nem barras
mesmo com espaços.

### Seleção de mensagens

`Ctrl-w` (padrão) entra no modo de seleção de mensagens. Com uma mensagem selecionada,
`o` abre anexos em um programa externo.

#### Exibir imagens

É possível mostrar imagens no terminal usando programas externos que convertem para
caracteres, como `jp2a` ou [PIXterm](https://github.com/eliukblau/pixterm). Configure o
comando em `show_command` no arquivo `whatscli.config` (veja a localização em `/help`).

#### Copiar IDs

Comandos como `/add` e `/remove` pedem um "user id". Copie o id de uma conversa/mensagem
selecionada com `Ctrl-c` e cole no campo com `Ctrl-v` (mapeamentos padrão).

### Notificações

Notificações de desktop usam a biblioteca `gen2brain/beeep`. Ative com
`enable_notifications = true` no `whatscli.config`. Para usar o "bell" do terminal,
`use_terminal_bell = true`.

### Configuração

Atalhos, cores e outras opções ficam no arquivo `whatscli.config`; o comando `/help`
mostra onde ele está.

---

## Estrutura do código (visão geral)

- `main.go` — elementos da interface (app `tview` na rotina principal), mapeamento de
  teclas (`tslocum/cbind`), seleção de mensagens e exibição da lista de conversas.
- `messages/session_manager.go` — roda em uma goroutine separada recebendo mensagens do
  `whatsmeow` (que mantém o websocket com o WhatsApp) e comandos da UI via canais,
  garantindo acesso "thread-safe" à conexão e aos dados.
- `messages/storage.go` — banco de mensagens (`MessageDatabase`).
- `messages/messages.go` — interfaces e estruturas de dados de comunicação.
- `config/settings.go` — singleton `Config` carregado via `gopkg.in/ini.v1` na inicialização.
- `assets/` — gerador do ícone (`make_icon.py`) e do recurso embutido no `.exe` (`make_syso.py`).
- `dist/` — pacotes prontos para distribuição (Windows e Linux).

## Créditos

ZapTerm é baseado no projeto [whatscli](https://github.com/normen/whatscli) (licença MIT).
Bibliotecas principais: [whatsmeow](https://github.com/tulir/whatsmeow) e
[tview](https://github.com/rivo/tview).

## Licença

Distribuído sob a licença MIT, herdada do projeto original.
