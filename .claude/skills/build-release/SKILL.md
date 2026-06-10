---
name: build-release
description: Como compilar, testar, empacotar (dist Linux/Windows) e lançar release do ZapTerm. Use para builds, cross-compile, ícones, pacotes e processo de versão/tag.
---

# Build, dist e release

## Comandos do dia a dia

```sh
make build     # go build (requer CGO por causa do go-sqlite3)
make run       # go run .
go test ./...  # testes (main + messages)
gofmt -l .     # deve sair vazio
make dist      # remonta dist/ (pacotes Linux + Windows + zips)
make release   # tag + push + acompanha o workflow "Release" no GitHub
```

## Versionamento

- A versão mora em `main.go`: `var VERSION string = "v1.1.6"`.
- `make-dist.sh` e `release.sh` extraem essa linha com `sed` — manter o
  formato exato. Bump de versão = editar só o número + commit
  `release: vX.Y.Z`.

## make-dist.sh (`make dist`)

- Build Linux: `CGO_ENABLED=1 go build -trimpath -ldflags="-s -w"` →
  `dist/ZapTerm-linux/zapterm` + scripts de `packaging/linux/`
  (Abrir/Instalar-no-menu/Atualizar/LEIA-ME) + `assets/zapterm.png`.
- Build Windows: cross-compile com mingw
  (`CC=x86_64-w64-mingw32-gcc`, instalar com
  `sudo apt install gcc-mingw-w64-x86-64`; sem ele o passo é pulado com
  aviso) → `dist/ZapTerm-windows/ZapTerm.exe` + `.bat`s de
  `packaging/windows/`.
- O ícone do `.exe` vem de `zapterm_windows_amd64.syso` (raiz), incluído
  automaticamente pelo go build quando `GOOS=windows`.
- Gera também `dist/ZapTerm-<versão>-*.zip` para anexar na release.

## release.sh (`make release`)

- Lê a versão (ou recebe como argumento), cria a tag git, faz push e espera
  o workflow GitHub Actions **"Release"** (usa `gh`). O workflow publica os
  assets, atualiza o tap Homebrew e os pacotes AUR.

## Onde fica cada coisa

| Caminho | Conteúdo |
| --- | --- |
| `packaging/linux/` | Scripts .sh de instalação/atalho/atualização + LEIA-ME |
| `packaging/windows/` | .bat equivalentes para Windows |
| `assets/` | `zapterm.png`/`.ico` + `make_icon.py`/`make_syso.py` (regeram ícone e .syso) |
| `dist/` | Saída do `make dist` (não editar à mão — é remontada do zero) |
| `doc/` | Screenshot usado no README |

## Cuidados

- `dist/` é destruída e recriada pelo script: mudanças permanentes em
  scripts de pacote vão em `packaging/`, nunca em `dist/`.
- CGO é obrigatório (sqlite3) — não usar `CGO_ENABLED=0`.
- Código novo precisa compilar para Windows (evitar `syscall`/APIs
  Linux-only fora de build tags).
