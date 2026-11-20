package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type opt struct{ label, val string }

type row struct {
	label string
	opts  []opt
	get   func() string
	set   func(string)
}

type hit struct {
	y, x0, x1 int // 1-based, inclusive
	row       int // -1 = input box
	val       string
}

const (
	stInfo = iota
	stBusy
	stOK
	stErr
)

type ui struct {
	t       *tty
	cfg     Config
	dir     string
	input   []rune
	focus   int // 0 = input, 1.. = settings rows
	hits    []hit
	status  string
	kind    int
	busy    bool
	cancel  context.CancelFunc
	quit    bool
	paste   bool
	updates chan func()
}

func (u *ui) rows() []row {
	c := &u.cfg
	r := []row{{
		label: "Type",
		opts:  []opt{{"Video", "video"}, {"Audio", "audio"}},
		get:   func() string { return c.Mode },
		set:   func(v string) { c.Mode = v },
	}}
	if c.Mode == "video" {
		r = append(r, row{
			label: "Resolution",
			opts: []opt{{"Max", "max"}, {"2160p", "2160"}, {"1440p", "1440"},
				{"1080p", "1080"}, {"720p", "720"}, {"480p", "480"}},
			get: func() string { return c.Res },
			set: func(v string) { c.Res = v },
		})
	} else {
		r = append(r,
			row{
				label: "Quality",
				opts:  []opt{{"Best", "best"}, {"320k", "320"}, {"192k", "192"}, {"128k", "128"}},
				get:   func() string { return c.AQ },
				set:   func(v string) { c.AQ = v },
			},
			row{
				label: "Format",
				opts:  []opt{{"mp3", "mp3"}, {"m4a", "m4a"}, {"opus", "opus"}, {"flac", "flac"}},
				get:   func() string { return c.AFmt },
				set:   func(v string) { c.AFmt = v },
			})
	}
	r = append(r,
		row{
			label: "Playlist",
			opts:  []opt{{"Off", "off"}, {"On", "on"}},
			get: func() string {
				if c.Playlist {
					return "on"
				}
				return "off"
			},
			set: func(v string) { c.Playlist = v == "on" },
		},
		row{
			label: "Cookies",
			opts: []opt{{"None", ""}, {"Firefox", "firefox"}, {"Chrome", "chrome"},
				{"Chromium", "chromium"}, {"Brave", "brave"}, {"Edge", "edge"}},
			get: func() string { return c.Cookies },
			set: func(v string) { c.Cookies = v },
		})
	return r
}

func (u *ui) setStatus(kind int, s string) { u.kind, u.status = kind, s }

func clip(s string, n int) string {
	if n < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func tail(r []rune, n int) string {
	if len(r) > n {
		r = r[len(r)-n:]
	}
	return string(r)
}

func (u *ui) render() {
	cols, _ := u.t.size()
	var b strings.Builder
	u.hits = u.hits[:0]
	y := 0
	emit := func(s string) {
		y++
		fmt.Fprintf(&b, "\x1b[%d;1H\x1b[2K%s", y, s)
	}
	const (
		dim  = "\x1b[2m"
		bold = "\x1b[1m"
		rev  = "\x1b[7m"
		off  = "\x1b[0m"
	)

	emit("")
	emit("  " + bold + "snag" + off)
	emit("")

	w := cols - 4
	if w > 72 {
		w = 72
	}
	if w < 12 {
		w = 12
	}
	inner := w - 4 // text width inside "│ " and " │"
	emit("  ╭" + strings.Repeat("─", w-2) + "╮")
	text := tail(u.input, inner)
	disp := text
	if len(u.input) == 0 {
		disp = dim + "Paste a link" + off
	}
	pad := inner - utf8.RuneCountInString(text)
	if len(u.input) == 0 {
		pad = inner - len("Paste a link")
	}
	inputY := y + 1
	emit("  │ " + disp + strings.Repeat(" ", max(pad, 0)) + " │")
	u.hits = append(u.hits, hit{y: y - 1, x0: 3, x1: 3 + w, row: -1})
	u.hits = append(u.hits, hit{y: y, x0: 3, x1: 3 + w, row: -1})
	emit("  ╰" + strings.Repeat("─", w-2) + "╯")
	u.hits = append(u.hits, hit{y: y, x0: 3, x1: 3 + w, row: -1})
	emit("")

	for i, r := range u.rows() {
		label := fmt.Sprintf("%-12s", r.label)
		if u.focus == i+1 {
			label = bold + label + off
		} else {
			label = dim + label + off
		}
		line := "  " + label
		x := 3 + 12
		cur := r.get()
		for _, o := range r.opts {
			wd := utf8.RuneCountInString(o.label) + 2
			if x+wd > cols {
				break
			}
			if o.val == cur {
				line += rev + " " + o.label + " " + off
			} else {
				line += dim + " " + o.label + " " + off
			}
			u.hits = append(u.hits, hit{y: y + 1, x0: x, x1: x + wd - 1, row: i, val: o.val})
			x += wd + 1
			line += " "
		}
		emit(line)
	}
	emit("")

	home, _ := os.UserHomeDir()
	shown := u.dir
	if home != "" && strings.HasPrefix(shown, home) {
		shown = "~" + strings.TrimPrefix(shown, home)
	}
	if shown == "" {
		shown = "(none)"
	}
	emit("  " + dim + "Saves to  " + clip(shown, cols-14) + off)
	emit("")

	st := clip(u.status, cols-4)
	switch u.kind {
	case stOK:
		st = "\x1b[32m" + st + off
	case stErr:
		st = "\x1b[31m" + st + off
	case stInfo:
		st = dim + st + off
	}
	emit("  " + st)
	emit("")
	help := "enter download · click or ←→ to change · esc cancel · ctrl-c quit"
	emit("  " + dim + clip(help, cols-4) + off)
	b.WriteString("\x1b[J")

	if u.focus == 0 {
		fmt.Fprintf(&b, "\x1b[%d;%dH\x1b[?25h", inputY, 5+utf8.RuneCountInString(text))
	} else {
		b.WriteString("\x1b[?25l")
	}
	u.t.f.WriteString(b.String())
}

func (u *ui) click(x, y int) {
	for _, h := range u.hits {
		if y != h.y || x < h.x0 || x > h.x1 {
			continue
		}
		if h.row < 0 {
			u.focus = 0
			return
		}
		u.focus = h.row + 1
		u.rows()[h.row].set(h.val)
		u.cfg.sanitize()
		u.cfg.save()
		return
	}
}

// step moves the focused setting one option left (-1) or right (+1).
func (u *ui) step(d int) {
	rs := u.rows()
	if u.focus < 1 || u.focus > len(rs) {
		return
	}
	r := rs[u.focus-1]
	cur := 0
	for i, o := range r.opts {
		if o.val == r.get() {
			cur = i
		}
	}
	cur = (cur + d + len(r.opts)) % len(r.opts)
	r.set(r.opts[cur].val)
	u.cfg.sanitize()
	u.cfg.save()
}

func (u *ui) insert(r rune) {
	u.focus = 0
	u.input = append(u.input, r)
}

func (u *ui) enter() {
	if u.busy {
		return
	}
	fields := strings.Fields(string(u.input))
	if len(fields) == 0 {
		u.setStatus(stInfo, "Paste a link first")
		return
	}
	urls := normalize(fields)
	for _, s := range urls {
		if !strings.Contains(s, ".") {
			u.setStatus(stErr, "That doesn't look like a link")
			return
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel, u.busy = cancel, true
	u.setStatus(stBusy, "Starting")
	cfg, dir := u.cfg, u.dir
	go func() {
		saved, err := download(ctx, cfg, dir, urls, func(s string) {
			select {
			case u.updates <- func() { u.setStatus(stBusy, s) }:
			default:
			}
		})
		u.updates <- func() {
			u.busy = false
			switch {
			case ctx.Err() != nil:
				u.setStatus(stInfo, "Cancelled")
			case err != nil:
				u.setStatus(stErr, err.Error())
			default:
				msg := "Saved  " + filepath.Base(saved[len(saved)-1])
				if len(saved) > 1 {
					msg = fmt.Sprintf("Saved %d files", len(saved))
				}
				u.input = nil
				u.focus = 0
				u.setStatus(stOK, msg)
			}
		}
	}()
}

func (u *ui) handle(b []byte) {
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c == 0x1b:
			if i+1 >= len(b) {
				if u.busy && u.cancel != nil {
					u.cancel()
				} else if !u.busy {
					u.quit = true
				}
				return
			}
			if b[i+1] != '[' {
				i += 2
				continue
			}
			j := i + 2
			for j < len(b) && (b[j] < 0x40 || b[j] > 0x7e) {
				j++
			}
			if j >= len(b) {
				return
			}
			u.csi(string(b[i+2:j]), b[j])
			i = j + 1
		case c == 0x03:
			u.quit = true
			return
		case c == 0x15:
			u.input = nil
			i++
		case c == 0x17:
			for len(u.input) > 0 && u.input[len(u.input)-1] == ' ' {
				u.input = u.input[:len(u.input)-1]
			}
			for len(u.input) > 0 && u.input[len(u.input)-1] != ' ' {
				u.input = u.input[:len(u.input)-1]
			}
			i++
		case c == 0x7f || c == 0x08:
			if len(u.input) > 0 {
				u.input = u.input[:len(u.input)-1]
			}
			i++
		case c == '\r' || c == '\n':
			if !u.paste {
				u.enter()
			}
			i++
		case c == '\t':
			u.focus = (u.focus + 1) % (len(u.rows()) + 1)
			i++
		case c < 0x20:
			i++
		default:
			r, n := utf8.DecodeRune(b[i:])
			if r != utf8.RuneError || n > 1 {
				u.insert(r)
			}
			i += n
		}
	}
}

func (u *ui) csi(params string, final byte) {
	n := len(u.rows())
	switch {
	case strings.HasPrefix(params, "<") && (final == 'M'):
		var cb, x, y int
		if _, err := fmt.Sscanf(params[1:], "%d;%d;%d", &cb, &x, &y); err == nil && cb == 0 {
			u.click(x, y)
		}
	case params == "200" && final == '~':
		u.paste = true
	case params == "201" && final == '~':
		u.paste = false
	case final == 'A':
		u.focus = (u.focus + n) % (n + 1)
	case final == 'B':
		u.focus = (u.focus + 1) % (n + 1)
	case final == 'C':
		u.step(+1)
	case final == 'D':
		u.step(-1)
	case final == 'Z':
		u.focus = (u.focus + n) % (n + 1)
	}
}

func runTUI(cfg Config, dir string, dirErr error) error {
	t, err := openTTY()
	if err != nil {
		return err
	}
	u := &ui{t: t, cfg: cfg, dir: dir, updates: make(chan func(), 64)}
	t.f.WriteString("\x1b[?1049h\x1b[?1000h\x1b[?1006h\x1b[?2004h")
	defer func() {
		if u.cancel != nil {
			u.cancel()
		}
		t.f.WriteString("\x1b[?2004l\x1b[?1006l\x1b[?1000l\x1b[?25h\x1b[?1049l")
		t.restore()
	}()
	if dirErr != nil {
		u.setStatus(stErr, dirErr.Error())
	}

	in := make(chan []byte)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := t.f.Read(buf)
			if n > 0 {
				in <- append([]byte(nil), buf[:n]...)
			}
			if err != nil {
				close(in)
				return
			}
		}
	}()
	resize := make(chan os.Signal, 1)
	quit := make(chan os.Signal, 1)
	notifySignals(resize, quit)

	for !u.quit {
		u.render()
		select {
		case b, ok := <-in:
			if !ok {
				return nil
			}
			u.handle(b)
		case f := <-u.updates:
			f()
		case <-resize:
		case <-quit:
			return nil
		}
	}
	return nil
}
