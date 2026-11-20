# snag

Paste a link, get the file. YouTube, TikTok, Instagram, X, Reddit and well over a thousand other sites, without watermarks. A small Go front end for [yt-dlp](https://github.com/yt-dlp/yt-dlp), with [gallery-dl](https://github.com/mikf/gallery-dl) as a fallback for image posts.

Site: https://fordpepper.github.io/snag

```
                snag

      ╭──────────────────────────────╮
      │ paste a link                 │
      ╰──────────────────────────────╯

       format       mp4   mp3
       resolution   max   2160p   1440p   1080p   720p   480p
       browser      none   firefox

       path         ~/Downloads
```

Run `snag`, paste a link, press Enter. Settings are clickable, or focus one with Tab and type (`720`, `mp3`, `zen`). They are remembered. Defaults are maximum resolution and best audio quality. Files go to your Downloads folder; focus the path row and type to change it. `snag <url>` skips the screen and downloads straight away.

## In the browser

```sh
snag serve
```

Open http://localhost:8787, or use the box on the [site](https://fordpepper.github.io/snag), which finds the server on your machine. Paste links, pick settings, and the file downloads through your browser (several links come back as a zip). The server listens on localhost only and rejects requests from other websites. `snag serve 9000` changes the port. It works in Termux too.

Browsers can't fetch from these sites directly, so a copy of snag has to run somewhere. The site doesn't run one for you.

## Install

Needs Go 1.21+ and ffmpeg. yt-dlp is downloaded automatically on first use if it's missing. gallery-dl is optional.

### Linux / macOS

```sh
git clone https://github.com/fordpepper/snag
cd snag
sh install.sh
```

`install.sh` installs ffmpeg, yt-dlp and Go with pacman, apt, dnf, zypper or brew, then builds and installs snag. Or by hand: `make && sudo make install`.

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
- Leave browser on none unless a site asks you to sign in (Instagram stories, age-gated posts, YouTube bot checks). Only browsers installed on your machine are listed, Firefox forks like Zen and LibreWolf included.
- If a download seems stuck, it is usually your network. snag forces IPv4, because broken IPv6 makes yt-dlp hang without any output.
- Sites change constantly. If one breaks, update yt-dlp first.
- Only download what you have the right to.

MIT
