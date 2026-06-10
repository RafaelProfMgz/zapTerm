#!/usr/bin/env bash
# ============================================================
#  ZapTerm - Atualizador (Linux)
# ============================================================
#  Baixa a versao mais recente publicada no GitHub Releases e
#  reinstala tudo: binario, icone e atalho do menu.
#
#  Como usar:
#      - clique em "Atualizar ZapTerm" no menu de aplicativos, OU
#      - rode no terminal:  zapterm-update
#      - ou direto desta pasta:  bash Atualizar-ZapTerm.sh
#
#  Para apontar para outro fork/repositorio:
#      ZAPTERM_REPO=usuario/repo bash Atualizar-ZapTerm.sh
# ============================================================
set -euo pipefail

REPO="${ZAPTERM_REPO:-RafaelProfMgz/zapTerm}"
API="https://api.github.com/repos/$REPO/releases/latest"

have() { command -v "$1" >/dev/null 2>&1; }

# --- Aberto pelo menu (sem terminal)? Reabre dentro de um terminal --------
if [ ! -t 1 ] && [ -z "${ZAPTERM_UPDATE_IN_TERM:-}" ]; then
  export ZAPTERM_UPDATE_IN_TERM=1
  SELF="$(readlink -f "${BASH_SOURCE[0]}")"
  if [ -n "${TERMINAL:-}" ] && have "$TERMINAL"; then
    exec "$TERMINAL" -e bash "$SELF"
  fi
  for term in x-terminal-emulator kitty alacritty wezterm gnome-terminal \
              konsole xfce4-terminal tilix terminator mate-terminal xterm; do
    have "$term" || continue
    case "$term" in
      gnome-terminal) exec gnome-terminal --title="Atualizar ZapTerm" -- bash "$SELF" ;;
      kitty)          exec kitty --title "Atualizar ZapTerm" -- bash "$SELF" ;;
      alacritty)      exec alacritty --title "Atualizar ZapTerm" -e bash "$SELF" ;;
      wezterm)        exec wezterm start -- bash "$SELF" ;;
      konsole)        exec konsole -p tabtitle="Atualizar ZapTerm" -e bash "$SELF" ;;
      tilix)          exec tilix -t "Atualizar ZapTerm" -e bash "$SELF" ;;
      terminator)     exec terminator -T "Atualizar ZapTerm" -e bash "$SELF" ;;
      xfce4-terminal) exec xfce4-terminal --title="Atualizar ZapTerm" --command="bash $SELF" ;;
      mate-terminal)  exec mate-terminal --title="Atualizar ZapTerm" --command="bash $SELF" ;;
      xterm)          exec xterm -T "Atualizar ZapTerm" -e bash "$SELF" ;;
      x-terminal-emulator) exec x-terminal-emulator -e bash "$SELF" ;;
    esac
  done
  # sem terminal disponivel: segue rodando "as cegas" mesmo (melhor que nada)
fi

# Quando aberto pelo menu, segura a janela aberta no final (sucesso ou erro)
TMP=""
cleanup() {
  [ -n "$TMP" ] && rm -rf "$TMP"
  if [ -n "${ZAPTERM_UPDATE_IN_TERM:-}" ]; then
    echo ""
    read -rp "Pressione Enter para fechar esta janela..." _ || true
  fi
}
trap cleanup EXIT

fetch() { # fetch <url> [arquivo-destino]
  if have curl; then
    if [ $# -gt 1 ]; then curl -fsSL "$1" -o "$2"; else curl -fsSL "$1"; fi
  elif have wget; then
    if [ $# -gt 1 ]; then wget -qO "$2" "$1"; else wget -qO- "$1"; fi
  else
    echo "ERRO: preciso de 'curl' ou 'wget' para baixar a atualizacao." >&2
    exit 1
  fi
}

have unzip || { echo "ERRO: instale o 'unzip' para atualizar (ex.: sudo apt install unzip)." >&2; exit 1; }

case "$(uname -m)" in
  x86_64) FLAVOR="linux" ;;
  armv6l | armv7l | aarch64) FLAVOR="raspberrypi" ;;
  *)
    echo "ERRO: nao ha build publicado para a arquitetura $(uname -m)." >&2
    exit 1
    ;;
esac

echo "==> Procurando a versao mais recente em github.com/$REPO ..."
URL="$(fetch "$API" | grep -o "\"browser_download_url\": *\"[^\"]*ZapTerm-[^\"]*-$FLAVOR\.zip\"" | cut -d'"' -f4 | head -n1)"
if [ -z "$URL" ]; then
  echo "ERRO: nao encontrei um arquivo ZapTerm-*-$FLAVOR.zip na ultima release." >&2
  exit 1
fi

TMP="$(mktemp -d)"

echo "==> Baixando $URL"
fetch "$URL" "$TMP/zapterm.zip"
unzip -q "$TMP/zapterm.zip" -d "$TMP"

PASTA="$(find "$TMP" -maxdepth 1 -type d -name 'ZapTerm-*' | head -n1)"
if [ -z "$PASTA" ]; then
  echo "ERRO: o zip baixado nao contem uma pasta ZapTerm-*." >&2
  exit 1
fi

echo "==> Instalando a nova versao (binario, icone e atalho do menu)..."
bash "$PASTA/Instalar-no-menu.sh"

echo ""
echo "Atualizacao concluida!"
