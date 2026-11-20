#!/data/data/com.termux/files/usr/bin/sh
# Installs snag on Android (Termux). Safe to re-run to update.
set -e

pkg update -y
pkg install -y git golang python ffmpeg yt-dlp
pip install --upgrade gallery-dl || true

# Gives Termux access to shared storage so files land in Downloads.
[ -d "$HOME/storage/downloads" ] || termux-setup-storage

dir="$HOME/.snag-src"
if [ -d "$dir/.git" ]; then
  git -C "$dir" pull --ff-only
else
  git clone https://github.com/fordpepper/snag "$dir"
fi

cd "$dir"
make install
echo
echo "Done. Run: snag"
