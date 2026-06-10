# Estilo de código e UI — regras fixas

## Idioma

- **Textos visíveis ao usuário em pt-BR**: títulos de painéis, ajuda, hints,
  placeholders, mensagens de erro impressas no chat, labels de mídia
  (`[áudio]`, `[imagem]`…), README.
- **Código em inglês**: nomes de funções/variáveis e comentários seguem o
  padrão já existente (comentários em inglês).

## Visual (terminal hacker / matrix, estilo lazyvim)

- Tema padrão: **verde-sobre-preto** (lime/palegreen/springgreen), definido
  em `config/settings.go`. Estética de "protocolo": tags em colchetes
  (`[ONLINE]`, `[F1] AJUDA`, `[3]` não lidas), prompt `você@zapterm:~$`,
  mensagens estilo IRC `[15:04:05] <nome> texto`, separadores `<<< ... >>>`,
  títulos de painel `[ MAIÚSCULAS ]`.
- **Sem emojis na interface.** Marcadores minimalistas: `#` para grupos,
  `*` para a conversa aberta, `[n]` para não lidas, `▶` para reprodução,
  `…` para truncar. Antes de adicionar qualquer ícone novo, preferir texto.
- Colchetes literais em TextView com cores dinâmicas DEVEM passar por
  `tview.Escape()` (senão viram tag de cor) — vale para `[ONLINE]`,
  `[15:04:05]`, `[3]` e qualquer texto vindo do usuário/WhatsApp.
- Cores sempre via `config.Config.Colors.*` + `tcell.ColorNames[...]` —
  nunca cor hardcoded (exceto tags dim/bold `[::d]`/`[::b]`).

## Go

- `gofmt` obrigatório em tudo (`gofmt -l .` deve sair vazio).
- `go test ./...` deve passar antes de concluir qualquer mudança.
- `go vet` reporta avisos pré-existentes de struct literal sem chaves em
  `messages.Command{...}` — esse é o idioma do projeto, manter consistente
  (não "consertar" nem introduzir avisos de outro tipo).
- Compatibilidade Windows: o projeto cross-compila para Windows
  (`make dist`). Não usar `syscall`/APIs específicas de Linux no pacote
  `main` ou `messages` sem build tags.
- Comentários explicam restrições não óbvias (deadlocks, threading, quirks
  do WhatsApp) — seguir a densidade já existente nos arquivos.

## Configuração

- Toda opção nova entra como campo em `config/settings.go` (struct + default
  em `var Config`). Nomes no INI são derivados por `TitleUnderscore`
  (ex.: `AudioCommand` → `audio_command`).
- Tecla nova de atalho = campo em `Keymap` + registro em `LoadShortcuts()`
  (`main.go`) + linha na ajuda (`buildHelpText`) e, se for essencial, no
  `hintText()` do rodapé.
