#!/usr/bin/env bash
# ============================================================
#  ZapTerm - Lancador inteligente (Linux)
# ============================================================
#  Abre o ZapTerm SEM depender da configuracao de terminal do
#  sistema (x-terminal-emulator / Terminal=true do .desktop).
#
#  Comportamento:
#   - Se ja estiver rodando DENTRO de um terminal, executa direto.
#   - Se for aberto pelo menu/duplo clique (sem terminal), ele
#     DETECTA um terminal instalado e abre o ZapTerm nele.
#
#  Ordem de busca de terminal (usa o primeiro encontrado):
#   $TERMINAL, x-terminal-emulator, kitty, alacritty, wezterm,
#   gnome-terminal, konsole, xfce4-terminal, tilix, terminator,
#   mate-terminal, xterm.
# ============================================================
set -euo pipefail

# --- 1. localizar o binario zapterm ---------------------------------------
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -n "${ZAPTERM_BIN:-}" ] && [ -x "${ZAPTERM_BIN}" ]; then
  BIN="$ZAPTERM_BIN"
elif [ -x "$SELF_DIR/zapterm" ]; then
  BIN="$SELF_DIR/zapterm"
elif command -v zapterm >/dev/null 2>&1; then
  BIN="$(command -v zapterm)"
else
  echo "ERRO: nao encontrei o binario 'zapterm'." >&2
  exit 1
fi

# --- 2. ja estamos num terminal? entao roda direto ------------------------
if [ -t 1 ]; then
  exec "$BIN" "$@"
fi

TITLE="ZapTerm"

# --- 3. tenta avisar o usuario caso nenhum terminal seja encontrado -------
fail() {
  local msg="$1"
  if command -v zenity >/dev/null 2>&1; then
    zenity --error --title="ZapTerm" --text="$msg" 2>/dev/null || true
  elif command -v kdialog >/dev/null 2>&1; then
    kdialog --error "$msg" 2>/dev/null || true
  elif command -v notify-send >/dev/null 2>&1; then
    notify-send "ZapTerm" "$msg" 2>/dev/null || true
  fi
  echo "$msg" >&2
  exit 1
}

# --- 4. detectar e abrir o terminal ---------------------------------------
# Cada terminal tem uma sintaxe propria para "rode este comando".
have() { command -v "$1" >/dev/null 2>&1; }

# Respeita a variavel $TERMINAL, se o usuario tiver definido uma
if [ -n "${TERMINAL:-}" ] && have "$TERMINAL"; then
  exec "$TERMINAL" -e "$BIN" "$@"
fi

for term in x-terminal-emulator kitty alacritty wezterm gnome-terminal \
            konsole xfce4-terminal tilix terminator mate-terminal xterm; do
  have "$term" || continue
  case "$term" in
    gnome-terminal)
      exec gnome-terminal --title="$TITLE" -- "$BIN" "$@" ;;
    kitty)
      exec kitty --title "$TITLE" -- "$BIN" "$@" ;;
    alacritty)
      exec alacritty --title "$TITLE" -e "$BIN" "$@" ;;
    wezterm)
      exec wezterm start -- "$BIN" "$@" ;;
    konsole)
      exec konsole -p tabtitle="$TITLE" -e "$BIN" "$@" ;;
    tilix)
      exec tilix -t "$TITLE" -e "$BIN" "$@" ;;
    terminator)
      exec terminator -T "$TITLE" -e "$BIN" "$@" ;;
    xfce4-terminal)
      exec xfce4-terminal --title="$TITLE" --command="$BIN $*" ;;
    mate-terminal)
      exec mate-terminal --title="$TITLE" --command="$BIN $*" ;;
    xterm)
      exec xterm -T "$TITLE" -e "$BIN" "$@" ;;
    x-terminal-emulator)
      exec x-terminal-emulator -e "$BIN" "$@" ;;
  esac
done

fail "Nenhum terminal foi encontrado. Instale um (ex.: gnome-terminal, konsole, kitty ou xterm) e tente de novo."
