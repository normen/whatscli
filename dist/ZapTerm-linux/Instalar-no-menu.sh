#!/usr/bin/env bash
# ============================================================
#  ZapTerm - Instalador para Linux
# ============================================================
#  Coloca o ZapTerm no menu de aplicativos (tela inicial),
#  com icone proprio. Ao clicar, abre um terminal e roda o
#  WhatsApp na linha de comando.
#
#  NAO precisa ter o Go instalado: o binario "zapterm" ja
#  vem pronto nesta mesma pasta.
#
#  Como usar:
#    1. Abra um terminal nesta pasta.
#    2. Rode:  bash Instalar-no-menu.sh
#    3. Procure por "ZapTerm" no menu/tela inicial.
# ============================================================
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_ORIGEM="$DIR/zapterm"
LAUNCHER_ORIGEM="$DIR/Abrir-ZapTerm.sh"
ICO_ORIGEM="$DIR/zapterm.png"

if [ ! -f "$BIN_ORIGEM" ]; then
  echo "ERRO: nao encontrei o binario 'zapterm' nesta pasta ($DIR)."
  exit 1
fi

BIN_DEST="$HOME/.local/bin/zapterm"
LAUNCHER_DEST="$HOME/.local/bin/zapterm-launch"
ICON_DEST="$HOME/.local/share/icons/zapterm.png"
APP_DIR="$HOME/.local/share/applications"
DESKTOP="$APP_DIR/zapterm.desktop"

echo "==> Instalando binario em: $BIN_DEST"
mkdir -p "$HOME/.local/bin"
install -m 0755 "$BIN_ORIGEM" "$BIN_DEST"

echo "==> Instalando lancador (detecta o terminal) em: $LAUNCHER_DEST"
# O lancador busca o binario 'zapterm' ao lado dele; como ambos vao para
# ~/.local/bin, a deteccao por "binario vizinho" funciona automaticamente.
install -m 0755 "$LAUNCHER_ORIGEM" "$LAUNCHER_DEST"

echo "==> Instalando icone..."
mkdir -p "$HOME/.local/share/icons"
[ -f "$ICO_ORIGEM" ] && install -m 0644 "$ICO_ORIGEM" "$ICON_DEST" || ICON_DEST="utilities-terminal"

echo "==> Criando atalho no menu de aplicativos..."
mkdir -p "$APP_DIR"
# Terminal=false de proposito: quem abre a janela de terminal e o nosso
# lancador, que detecta um terminal instalado. Assim NAO dependemos do
# x-terminal-emulator nem da configuracao de terminal do sistema.
cat > "$DESKTOP" <<EOF
[Desktop Entry]
Type=Application
Version=1.0
Name=ZapTerm
GenericName=Cliente de WhatsApp
Comment=Use o WhatsApp pelo terminal: envie e receba mensagens, imagens, audios e documentos sem abrir o navegador
Exec=$LAUNCHER_DEST
Icon=$ICON_DEST
Terminal=false
Categories=Network;InstantMessaging;ConsoleOnly;
Keywords=whatsapp;zap;chat;mensagens;terminal;cli;
EOF
chmod +x "$DESKTOP"

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$APP_DIR" >/dev/null 2>&1 || true
fi

echo ""
echo "Pronto! Procure por \"ZapTerm\" no menu/tela inicial."
echo "Tambem da para rodar pelo terminal digitando: zapterm"
echo "(se 'zapterm' nao for encontrado, adicione \$HOME/.local/bin ao seu PATH)"
