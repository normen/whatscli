#!/usr/bin/env bash
# ============================================================
#  ZapTerm - Atualizador (Linux)
# ============================================================
#  Baixa a versao mais recente publicada no GitHub Releases e
#  reinstala tudo: binario, icone e atalho do menu.
#
#  Como usar:
#      bash Atualizar-ZapTerm.sh
#  (ou, depois de instalado no menu, rode:  zapterm-update)
#
#  Para apontar para outro fork/repositorio:
#      ZAPTERM_REPO=usuario/repo bash Atualizar-ZapTerm.sh
# ============================================================
set -euo pipefail

REPO="${ZAPTERM_REPO:-RafaelProfMgz/whatscli}"
API="https://api.github.com/repos/$REPO/releases/latest"

have() { command -v "$1" >/dev/null 2>&1; }

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
trap 'rm -rf "$TMP"' EXIT

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
