#!/bin/sh
# Installs snag's dependencies (ffmpeg, yt-dlp) and builds snag.
# Linux and macOS. Android: use install-termux.sh.
set -e

have() { command -v "$1" >/dev/null 2>&1; }
as_root() { if [ "$(id -u)" = 0 ]; then "$@"; else sudo "$@"; fi; }

missing=""
have ffmpeg || missing="$missing ffmpeg"
have yt-dlp || missing="$missing yt-dlp"
have go     || missing="$missing go"

if [ -n "$missing" ]; then
  echo "Installing:$missing"
  if   have pacman; then as_root pacman -S --needed --noconfirm $(echo "$missing" | sed 's/\bgo\b/go/')
  elif have apt;    then as_root apt update && as_root apt install -y $(echo "$missing" | sed 's/\bgo\b/golang/; s/yt-dlp//') && { have yt-dlp || { have pipx || as_root apt install -y pipx; pipx install yt-dlp; }; }
  elif have dnf;    then as_root dnf install -y $(echo "$missing" | sed 's/\bgo\b/golang/')
  elif have zypper; then as_root zypper install -y $(echo "$missing" | sed 's/\bgo\b/go/')
  elif have brew;   then brew install $missing
  else echo "No supported package manager found. Install:$missing"; exit 1
  fi
fi

make
if [ "$(id -u)" = 0 ] || [ ! -w /usr/local/bin ]; then as_root make install; else make install; fi
echo
echo "Done. Run: snag"
