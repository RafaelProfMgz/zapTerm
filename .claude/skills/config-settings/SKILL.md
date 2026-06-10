---
name: config-settings
description: Como funciona o pacote config/ — arquivo INI do usuário, defaults, keymap, cores e .env. Use ao adicionar/alterar qualquer configuração, atalho de teclado ou cor.
---

# Configuração (pacote `config/`)

## Arquivos

| Arquivo | Responsabilidade |
| --- | --- |
| `settings.go` | Structs de config, defaults, load/save do INI |
| `dotenv.go` | Carrega `.env` da pasta do projeto (ex.: `OPENAI_API_KEY`) |

## Onde fica a config do usuário

- INI em `~/.config/whatscli/whatscli.config` (via xdg). Seções: `[general]`,
  `[keymap]`, `[ui]`, `[colors]`, `[bot]`.
- Sessão WhatsApp: `~/.config/whatscli/session.db` (SQLite do whatsmeow).
- O caminho real é exposto por `config.GetConfigFilePath()` (aparece no fim
  da ajuda F1).

## Como o load funciona (`InitConfig`)

1. `LoadDotEnv()` primeiro (para `os.ExpandEnv` funcionar nos valores).
2. Se o arquivo existe: `MapTo` seção a seção **por cima dos defaults** —
   campos ausentes no INI do usuário mantêm o default (por isso adicionar
   campo novo é seguro para configs antigas).
3. Se não existe: grava um INI completo gerado dos defaults.
4. Mapeamento de nomes: `ini.TitleUnderscore` — `AudioCommand` ↔
   `audio_command`, `InlineImageLines` ↔ `inline_image_lines`.

## Como adicionar uma configuração

1. Campo na struct certa (`General`, `Ui`, `Colors`, `Keymap`, `Bot`) com
   comentário explicando o que faz.
2. Default em `var Config = IniFile{...}`.
3. Usar via `config.Config.<Seção>.<Campo>` — nunca reler o arquivo.

## Atalhos (Keymap) e cores

- Strings de tecla no formato do cbind: `"Ctrl+f"`, `"p"`, `"Tab"`.
  O registro acontece em `LoadShortcuts()` (main.go).
- Cores são nomes do tcell (`"yellow"`, `"black"`…), resolvidas com
  `tcell.ColorNames[config.Config.Colors.X]`. O comando `/colorlist`
  imprime todas as opções no chat.

## Configs importantes do general

| Chave | Efeito |
| --- | --- |
| `download_path` / `preview_path` | Onde anexos são salvos (default `~/Downloads`) |
| `cmd_prefix` | Prefixo de comandos (default `/`) |
| `audio_command` | Player de áudio; vazio = auto-detecta mpv/ffplay/play/cvlc |
| `show_command` | Visualizador de imagem externo (default `jp2a --color`) |
| `inline_images` / `inline_image_lines` | Render half-block no terminal |
| `backlog_msg_quantity` | Quantas mensagens `/backlog` pede |
