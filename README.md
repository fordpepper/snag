# snag

Download video and audio from YouTube, TikTok, Instagram, X, Reddit and about a thousand other sites. A small Go wrapper around [yt-dlp](https://github.com/yt-dlp/yt-dlp) that picks the clean source stream, so no watermarks.

Site and command builder: https://fordpepper.github.io/snag

## Build

Needs Go 1.21+, yt-dlp and ffmpeg.

```sh
git clone https://github.com/fordpepper/snag
cd snag
make
sudo make install   # optional
```

Windows: `go build -o snag.exe .`

## Use

```sh
snag https://youtu.be/dQw4w9WgXcQ
snag -a https://soundcloud.com/artist/track
snag -q 720 -o ~/Videos https://x.com/user/status/123
snag -p https://www.tiktok.com/@user
snag -b firefox https://www.instagram.com/stories/user/
```

| flag | |
|---|---|
| `-a` | audio only (mp3) |
| `-q N` | max height |
| `-o DIR` | output directory |
| `-b NAME` | cookies from browser (private / age-gated) |
| `-p` | whole playlist or profile |
| `-v` | verbose |

If a site stops working, update yt-dlp first.

Only download what you have the right to.

MIT
