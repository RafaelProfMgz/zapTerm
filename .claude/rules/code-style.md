# Estilo de código e UI — regras fixas

## Idioma

- **Textos visíveis ao usuário em pt-BR**: títulos de painéis, ajuda, hints,
  placeholders, mensagens de erro impressas no chat, labels de mídia
  (`[áudio]`, `[imagem]`…), README.
- **Código em inglês**: nomes de funções/variáveis e comentários seguem o
  padrão já existente (comentários em inglês).

## Visual (estilo lazyvim)

- **Sem emojis na interface.** Marcadores minimalistas: `#` para grupos,
  `●` para não lidas, `▶` para reprodução, `…` para truncar. Antes de
  adicionar qualquer ícone novo, preferir texto.
- Cores sempre via `config.Config.Colors.*` + `tcell.ColorNames[...]` —
  nunca cor hardcoded (exceto tags dim/bold `[::d]`/`[::b]`).
- Texto vindo do usuário/WhatsApp passa por `tview.Escape()` antes de entrar
  em um TextView com cores dinâmicas (nomes de contato inclusive).

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
