#!/usr/bin/env bash
# ============================================================
#  make-dist.sh — remonta a pasta dist/ com binarios novos e os
#  atalhos/instaladores mais recentes (fonte: packaging/).
#
#  Uso:  ./make-dist.sh        (ou: make dist)
#
#  Gera:
#    dist/ZapTerm-linux/            pacote pronto p/ Linux
#    dist/ZapTerm-windows/          pacote pronto p/ Windows
#    dist/ZapTerm-<versao>-*.zip    zips para anexar em release
#
#  O build de Windows e pulado (com aviso) se o cross-compiler
#  mingw nao estiver instalado: sudo apt install gcc-mingw-w64-x86-64
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
rm -rf "$LINUX_DIR" "$WIN_DIR" dist/ZapTerm-*.zip
mkdir -p "$LINUX_DIR"

echo "==> Build Linux (amd64)..."
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$LINUX_DIR/zapterm" .
cp packaging/linux/Abrir-ZapTerm.sh \
   packaging/linux/Instalar-no-menu.sh \
   packaging/linux/Atualizar-ZapTerm.sh \
   packaging/linux/LEIA-ME.txt \
   "$LINUX_DIR/"
cp assets/zapterm.png "$LINUX_DIR/"
chmod +x "$LINUX_DIR"/*.sh "$LINUX_DIR/zapterm"

if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
  echo "==> Build Windows (amd64)..."
  mkdir -p "$WIN_DIR"
  # o icone do .exe vem do zapterm_windows_amd64.syso, incluido
  # automaticamente pelo go build quando GOOS=windows
  CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
    go build -trimpath -ldflags="-s -w" -o "$WIN_DIR/ZapTerm.exe" .
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
