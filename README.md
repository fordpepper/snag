# snag

Paste a link, get the file. YouTube, TikTok, Instagram, X, Reddit and well over a thousand other sites, without watermarks. A small Go front end for [yt-dlp](https://github.com/yt-dlp/yt-dlp), with [gallery-dl](https://github.com/mikf/gallery-dl) as a fallback for image posts.

Site: https://fordpepper.github.io/snag

```
  ╭────────────────────────────────────────╮
  │ Paste a link                           │
  ╰────────────────────────────────────────╯

  Type         Video   Audio
  Resolution   Max   2160p   1440p   1080p   720p   480p
  Playlist     Off   On
  Cookies      None   Firefox   Chrome   Brave

  Saves to  ~/Downloads
```

Run `snag`, paste a link, press Enter. Settings are clickable (or use Tab and the arrow keys) and are remembered. Defaults are maximum resolution and best audio quality. Everything is saved to your Downloads folder. `snag <url>` skips the screen and downloads straight away.

## Install

Needs Go 1.21+, yt-dlp and ffmpeg. gallery-dl is optional.

### Linux / macOS

```sh
git clone https://github.com/fordpepper/snag
cd snag
make
sudo make install   # optional
```

Dependencies: `brew install go yt-dlp ffmpeg gallery-dl` (macOS), `sudo pacman -S go yt-dlp ffmpeg` (Arch), `sudo apt install golang ffmpeg pipx && pipx install yt-dlp` (Debian/Ubuntu).

### Android (Termux, tmux)

Use Termux from F-Droid.

```sh
pkg install git
git clone https://github.com/fordpepper/snag
cd snag
sh install-termux.sh
```

Then `snag`. Files go to the phone's Downloads folder. In tmux, turn on mouse support so taps work: `tmux set -g mouse on`.

### Windows

`go build -o snag.exe .` then `snag <url>`. The clickable screen needs a unix terminal, so use WSL for it.

## Notes

- Watermarks are avoided by taking the original stream a site serves, not by editing video.
- Private or age-gated posts (Instagram stories, for example) need Cookies set to a browser you're logged in with.
- Sites change constantly. If one breaks, update yt-dlp first.
- Only download what you have the right to.

MIT
