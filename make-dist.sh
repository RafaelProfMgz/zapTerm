#!/usr/bin/env bash
# ============================================================
#  make-dist.sh — remonta a pasta dist/ com binarios novos e os
#  atalhos/instaladores mais recentes (fonte: packaging/).
#
#  Uso:  ./make-dist.sh        (ou: make dist)
#
#  Modelo de empacotamento (v2+): a interface PADRAO e a Ink,
#  compilada com "bun --compile" num executavel unico que NAO
#  exige Node instalado. O nucleo Go (cerebro: sessao, storage,
#  midia) vai ao lado como "zapterm-core" e roda headless
#  (--ui=json). Quem preferir a UI classica (tview) roda o
#  proprio "zapterm-core" direto.
#
#  Gera:
#    dist/ZapTerm-linux/      zapterm (Ink) + zapterm-core (Go)
#    dist/ZapTerm-windows/    ZapTerm.exe (Ink) + zapterm-core.exe
#    dist/ZapTerm-<versao>-*.zip
#
#  Requisitos:
#    - bun  ......... para compilar a UI Ink (https://bun.sh).
#                     Sem bun: empacota so o nucleo Go como UI
#                     (tview) e avisa.
#    - mingw ........ cross-compile do nucleo Go p/ Windows:
#                     sudo apt install gcc-mingw-w64-x86-64
# ============================================================
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

VERSION="$(sed -n 's/^var VERSION string = "\(v[^"]*\)"$/\1/p' main.go | head -n1)"
if [ -z "$VERSION" ]; then
  echo "ERRO: nao consegui ler a VERSION em main.go" >&2
  exit 1
fi
echo "==> Empacotando ZapTerm $VERSION"

LINUX_DIR="dist/ZapTerm-linux"
WIN_DIR="dist/ZapTerm-windows"
INK_DIR="ink-ui"
rm -rf "$LINUX_DIR" "$WIN_DIR" dist/ZapTerm-*.zip
mkdir -p "$LINUX_DIR"

HAVE_BUN=0
command -v bun >/dev/null 2>&1 && HAVE_BUN=1

# garante as dependencias do Ink (inclui react-devtools-core, exigido pelo
# bundle do ink) antes de compilar
ensure_ink_deps() {
  if [ ! -d "$INK_DIR/node_modules/ink" ]; then
    echo "==> Instalando dependencias do Ink..."
    if [ "$HAVE_BUN" -eq 1 ]; then (cd "$INK_DIR" && bun install); else (cd "$INK_DIR" && npm install); fi
  fi
}

# compile_ink <saida-relativa-a-INK_DIR> [target-bun]
compile_ink() {
  local out="$1" target="${2:-}"
  local args=(build src/index.mjs --compile --outfile "$out")
  [ -n "$target" ] && args+=(--target="$target")
  (cd "$INK_DIR" && bun "${args[@]}")
}

echo "==> Build nucleo Go (Linux amd64)..."
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$LINUX_DIR/zapterm-core" .

if [ "$HAVE_BUN" -eq 1 ]; then
  ensure_ink_deps
  echo "==> Compilando UI Ink (Linux)..."
  compile_ink "../$LINUX_DIR/zapterm"
else
  echo "AVISO: 'bun' nao encontrado — usando o nucleo Go (tview) como UI padrao."
  echo "       Instale o bun (https://bun.sh) para empacotar a interface Ink."
  cp "$LINUX_DIR/zapterm-core" "$LINUX_DIR/zapterm"
fi

cp packaging/linux/Abrir-ZapTerm.sh \
   packaging/linux/Instalar-no-menu.sh \
   packaging/linux/Atualizar-ZapTerm.sh \
   packaging/linux/LEIA-ME.txt \
   "$LINUX_DIR/"
cp assets/zapterm.png "$LINUX_DIR/"
chmod +x "$LINUX_DIR"/*.sh "$LINUX_DIR"/zapterm "$LINUX_DIR"/zapterm-core

if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
  echo "==> Build nucleo Go (Windows amd64)..."
  mkdir -p "$WIN_DIR"
  # o icone do .exe do nucleo vem do zapterm_windows_amd64.syso, incluido
  # automaticamente pelo go build quando GOOS=windows
  CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
    go build -trimpath -ldflags="-s -w" -o "$WIN_DIR/zapterm-core.exe" .

  if [ "$HAVE_BUN" -eq 1 ]; then
    ensure_ink_deps
    echo "==> Compilando UI Ink (Windows)..."
    # bun acrescenta ".exe" no alvo windows; passamos o nome sem extensao
    compile_ink "../$WIN_DIR/ZapTerm" "bun-windows-x64"
  else
    cp "$WIN_DIR/zapterm-core.exe" "$WIN_DIR/ZapTerm.exe"
  fi

  cp packaging/windows/Abrir-ZapTerm.bat \
     packaging/windows/Instalar-no-menu-iniciar.bat \
     packaging/windows/Atualizar-ZapTerm.bat \
     packaging/windows/LEIA-ME.txt \
     "$WIN_DIR/"
  cp assets/zapterm.ico "$WIN_DIR/"
else
  echo "AVISO: x86_64-w64-mingw32-gcc nao encontrado — pulando o build Windows."
  echo "       Instale com: sudo apt install gcc-mingw-w64-x86-64"
fi

if command -v zip >/dev/null 2>&1; then
  echo "==> Gerando zips..."
  (
    cd dist
    zip -qr "ZapTerm-$VERSION-linux.zip" ZapTerm-linux
    if [ -f ZapTerm-windows/ZapTerm.exe ]; then
      zip -qr "ZapTerm-$VERSION-windows.zip" ZapTerm-windows
    fi
  )
else
  echo "AVISO: 'zip' nao encontrado — pastas geradas, mas sem os zips."
fi

echo "==> Pronto:"
ls -lh dist/ | sed 's/^/    /'
