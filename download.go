package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// stream runs a command and calls onLine for each output line (stdout+stderr).
func stream(ctx context.Context, name string, args []string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, name, args...)
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return err
	}
	pw.Close()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	sc.Split(splitLines)
	for sc.Scan() {
		onLine(sc.Text())
	}
	pr.Close()
	return cmd.Wait()
}

func ytdlpArgs(c Config, dir string, urls []string) []string {
	tmpl := "%(title).120B [%(id)s].%(ext)s"
	a := []string{
		"-P", dir, "-o", tmpl,
		"--no-mtime", "--windows-filenames", "--embed-metadata",
		"--newline", "--progress", "--no-warnings",
		"-4", "--socket-timeout", "20", "--retries", "3", // broken IPv6 makes yt-dlp hang silently
		"--print", "after_move:filepath",
	}
	// A video link inside a playlist gets just that video; a pure
	// playlist or profile link still downloads everything.
	a = append(a, "--no-playlist", "--ignore-errors")
	if c.Cookies != "" {
		a = append(a, "--cookies-from-browser", c.Cookies)
	}
	if c.Mode == "audio" {
		q := "0"
		if c.AQ != "best" {
			q = c.AQ + "K"
		}
		a = append(a, "-f", "ba/b", "-x", "--audio-format", "mp3", "--audio-quality", q)
	} else {
		sel := "bv*+ba/b"
		if c.Res != "max" {
			sel = "bv*[height<=" + c.Res + "]+ba/b[height<=" + c.Res + "]/b"
		}
		// Highest resolution first, then prefer mp4/m4a. Watermarked
		// variants (TikTok) are ranked below the clean stream by yt-dlp.
		a = append(a, "-f", sel, "-S", "res,ext:mp4:m4a", "--merge-output-format", "mp4/mkv")
	}
	return append(a, urls...)
}

// download fetches urls into dir. Returns the saved file paths.
func download(ctx context.Context, c Config, dir string, urls []string, prog func(string)) ([]string, error) {
	if dir == "" {
		return nil, errors.New("set a download path first")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, errors.New("can't use that folder: " + err.Error())
	}
	ytdlp, err := ensureYtdlp(prog)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, errors.New("ffmpeg not found. install it: " + ffmpegHint())
	}

	var saved []string
	lastErr := ""
	err = stream(ctx, ytdlp, ytdlpArgs(c, dir, urls), func(l string) {
		l = strings.TrimSpace(l)
		switch {
		case l == "":
		case strings.HasPrefix(l, "ERROR:"):
			lastErr = strings.TrimSpace(strings.TrimPrefix(l, "ERROR:"))
		case strings.HasPrefix(l, "[download]"):
			if strings.Contains(l, "%") {
				prog(strings.Join(strings.Fields(strings.TrimPrefix(l, "[download]")), " "))
			} else {
				prog("Downloading")
			}
		case strings.HasPrefix(l, "["):
			prog("Processing")
		case filepath.IsAbs(l):
			saved = append(saved, l)
		}
	})
	if len(saved) > 0 {
		return saved, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Image posts and sites yt-dlp can't read: try gallery-dl if present.
	if _, e := exec.LookPath("gallery-dl"); e == nil {
		prog("Trying images")
		g := []string{"-D", dir}
		if c.Cookies != "" {
			g = append(g, "--cookies-from-browser", c.Cookies)
		}
		_ = stream(ctx, "gallery-dl", append(g, urls...), func(l string) {
			l = strings.TrimSpace(l)
			if l != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "[") && filepath.IsAbs(l) {
				saved = append(saved, l)
				prog("Saved " + filepath.Base(l))
			}
		})
		if len(saved) > 0 {
			return saved, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	if lastErr == "" {
		lastErr = "download failed"
		if err != nil {
			lastErr = err.Error()
		}
	}
	return nil, errors.New(friendly(lastErr))
}

// friendly turns common yt-dlp failures into something actionable.
func friendly(e string) string {
	l := strings.ToLower(e)
	switch {
	case strings.Contains(l, "confirm you") && strings.Contains(l, "bot"),
		strings.Contains(l, "sign in"), strings.Contains(l, "log in"), strings.Contains(l, "login"),
		strings.Contains(l, "logged-in"), strings.Contains(l, "cookies"):
		return "this needs a login. set browser to one you're signed in with, then try again."
	case strings.Contains(l, "unsupported url"):
		return "that link isn't supported."
	case strings.Contains(l, "ffmpeg"):
		return e + " (install ffmpeg: " + ffmpegHint() + ")"
	}
	if len(e) > 200 {
		e = e[:200] + "…"
	}
	return e
}
