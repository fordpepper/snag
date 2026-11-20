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
	y, x0, x1 int // 1-based screen cells, inclusive
	row       int // -1 = link box, -2 = path
	val       string
}

const (
	stInfo = iota
	stBusy
	stOK
	stErr
)

type ui struct {
	t        *tty
	cfg      Config
	dir      string
	input    []rune
	pathEdit []rune
	focus    int // 0 = link, 1..n = settings, n+1 = path
	buf      string
	hits     []hit
	status   string
	kind     int
	busy     bool
	cancel   context.CancelFunc
	quit     bool
	paste    bool
	updates  chan func()
}

func (u *ui) rows() []row {
	c := &u.cfg
	r := []row{{
		label: "format",
		opts:  []opt{{"mp4", "video"}, {"mp3", "audio"}},
		get:   func() string { return c.Mode },
		set:   func(v string) { c.Mode = v },
	}}
	if c.Mode == "video" {
		r = append(r, row{
			label: "resolution",
			opts: []opt{{"max", "max"}, {"2160p", "2160"}, {"1440p", "1440"},
				{"1080p", "1080"}, {"720p", "720"}, {"480p", "480"}},
			get: func() string { return c.Res },
			set: func(v string) { c.Res = v },
		})
	} else {
		r = append(r, row{
			label: "quality",
			opts:  []opt{{"best", "best"}, {"320k", "320"}, {"192k", "192"}, {"128k", "128"}},
			get:   func() string { return c.AQ },
			set:   func(v string) { c.AQ = v },
		})
	}
	if b := browsers(); len(b) > 1 {
		r = append(r, row{
			label: "browser",
			opts:  b,
			get:   func() string { return c.Cookies },
			set:   func(v string) { c.Cookies = v },
		})
	}
	return r
}

func (u *ui) pathFocus() int { return len(u.rows()) + 1 }

func (u *ui) setFocus(f int) {
	if f != u.focus {
		u.buf = ""
	}
	u.focus = f
}

func (u *ui) setStatus(kind int, s string) {
	if kind != stOK {
		s = strings.ToLower(s)
	}
	u.kind, u.status = kind, s
}

func tail(r []rune, n int) string {
	if n < 0 {
		n = 0
	}
	if len(r) > n {
		r = r[len(r)-n:]
	}
	return string(r)
}

func clip(s string, n int) string {
	r := []rune(s)
	if n < 1 {
		return ""
	}
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func (u *ui) render() {
	cols, rowsN := u.t.size()
	const (
		dim  = "\x1b[2m"
		bold = "\x1b[1m"
		rev  = "\x1b[7m"
		off  = "\x1b[0m"
	)
	var lines []string
	var hits []hit // y = line index for now
	add := func(s string) int { lines = append(lines, s); return len(lines) - 1 }
	center := func(n int, s string) string {
		p := (cols - n) / 2
		if p < 0 {
			p = 0
		}
		return strings.Repeat(" ", p) + s
	}

	w := cols - 4
	if w > 60 {
		w = 60
	}
	if w < 24 {
		w = 24
	}
	m := (cols - w) / 2
	if m < 0 {
		m = 0
	}
	ind := strings.Repeat(" ", m)
	inner := w - 4

	add(center(4, bold+"snag"+off))
	add("")
	add(ind + "╭" + strings.Repeat("─", w-2) + "╮")
	text := tail(u.input, inner)
	disp := text
	used := utf8.RuneCountInString(text)
	if len(u.input) == 0 {
		disp, used = dim+"paste a link"+off, len("paste a link")
	}
	inputLine := add(ind + "│ " + disp + strings.Repeat(" ", max(inner-used, 0)) + " │")
	add(ind + "╰" + strings.Repeat("─", w-2) + "╯")
	for l := inputLine - 1; l <= inputLine+1; l++ {
		hits = append(hits, hit{y: l, x0: m + 1, x1: m + w, row: -1})
	}
	add("")

	labelCol := m + 2
	optCol := labelCol + 12
	for i, r := range u.rows() {
		label := fmt.Sprintf("%-12s", r.label)
		if u.focus == i+1 {
			label = bold + label + off
		} else {
			label = dim + label + off
		}
		line := ind + " " + label
		x := optCol
		cur := r.get()
		li := len(lines)
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
			hits = append(hits, hit{y: li, x0: x, x1: x + wd - 1, row: i, val: o.val})
			x += wd + 1
			line += " "
		}
		add(line)
	}

	add("")
	pathLabel := fmt.Sprintf("%-12s", "path")
	ptxt := tail(u.pathEdit, cols-optCol-1)
	pline := ind + " "
	if u.focus == u.pathFocus() {
		pline += bold + pathLabel + off + ptxt
	} else {
		pline += dim + pathLabel + off + ptxt
	}
	if len(u.pathEdit) == 0 {
		pline += dim + "type a folder" + off
	}
	pathLine := add(pline)
	hits = append(hits, hit{y: pathLine, x0: m + 1, x1: cols, row: -2})
	add("")

	st := clip(u.status, cols-4)
	stN := utf8.RuneCountInString(st)
	switch u.kind {
	case stOK:
		st = "\x1b[32m" + st + off
	case stErr:
		st = "\x1b[31m" + st + off
	default:
		st = dim + st + off
	}
	add(center(stN, st))
	add("")
	help := clip("click or type to change · tab moves · enter downloads · ctrl-c quits", cols-2)
	add(center(utf8.RuneCountInString(help), dim+help+off))

	top := (rowsN - len(lines)) / 2
	if top < 0 {
		top = 0
	}
	var b strings.Builder
	for y := 1; y <= rowsN; y++ {
		i := y - 1 - top
		if i >= 0 && i < len(lines) {
			fmt.Fprintf(&b, "\x1b[%d;1H\x1b[2K%s", y, lines[i])
		} else {
			fmt.Fprintf(&b, "\x1b[%d;1H\x1b[2K", y)
		}
	}
	u.hits = u.hits[:0]
	for _, h := range hits {
		h.y += top + 1
		u.hits = append(u.hits, h)
	}
	switch {
	case u.focus == 0:
		fmt.Fprintf(&b, "\x1b[%d;%dH\x1b[?25h", inputLine+top+1, m+3+used*boolInt(len(u.input) > 0))
	case u.focus == u.pathFocus():
		fmt.Fprintf(&b, "\x1b[%d;%dH\x1b[?25h", pathLine+top+1, optCol+utf8.RuneCountInString(ptxt))
	default:
		b.WriteString("\x1b[?25l")
	}
	u.t.f.WriteString(b.String())
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (u *ui) commit() {
	u.cfg.sanitize()
	u.cfg.save()
}

func (u *ui) click(x, y int) {
	for _, h := range u.hits {
		if y != h.y || x < h.x0 || x > h.x1 {
			continue
		}
		switch h.row {
		case -1:
			u.setFocus(0)
		case -2:
			u.setFocus(u.pathFocus())
		default:
			u.setFocus(h.row + 1)
			u.rows()[h.row].set(h.val)
			u.commit()
		}
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
	r.set(r.opts[(cur+d+len(r.opts))%len(r.opts)].val)
	u.commit()
}

// field is the text the keyboard edits right now, if any.
func (u *ui) field() *[]rune {
	switch {
	case u.focus == 0:
		return &u.input
	case u.focus == u.pathFocus():
		return &u.pathEdit
	}
	return nil
}

func (u *ui) pathChanged() {
	u.dir = expandDir(string(u.pathEdit))
	u.cfg.Dir = string(u.pathEdit)
	u.cfg.save()
}

func (u *ui) typeRune(r rune) {
	if u.paste {
		u.setFocus(0)
	}
	if f := u.field(); f != nil {
		*f = append(*f, r)
		if u.focus != 0 {
			u.pathChanged()
		}
		return
	}
	// settings: typing "720" or "mp3" or "zen" picks that option
	rs := u.rows()
	if u.focus < 1 || u.focus > len(rs) {
		return
	}
	rw := rs[u.focus-1]
	pick := func(p string) bool {
		for _, o := range rw.opts {
			if strings.HasPrefix(strings.ToLower(o.label), p) {
				rw.set(o.val)
				return true
			}
		}
		return false
	}
	ch := strings.ToLower(string(r))
	switch {
	case pick(u.buf + ch):
		u.buf += ch
	case pick(ch):
		u.buf = ch
	default:
		return
	}
	u.commit()
}

func (u *ui) edit(fn func(r []rune) []rune) {
	if f := u.field(); f != nil {
		*f = fn(*f)
		if u.focus != 0 {
			u.pathChanged()
		}
	}
}

func (u *ui) enter() {
	if u.focus == u.pathFocus() {
		u.setFocus(0)
		return
	}
	if u.busy {
		return
	}
	fields := strings.Fields(string(u.input))
	if len(fields) == 0 {
		u.setStatus(stInfo, "paste a link first")
		return
	}
	urls := normalize(fields)
	for _, s := range urls {
		if !strings.Contains(s, ".") {
			u.setStatus(stErr, "that doesn't look like a link")
			return
		}
	}
	if u.dir == "" {
		u.setStatus(stErr, "type a download folder first")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel, u.busy = cancel, true
	u.setStatus(stBusy, "starting")
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
				u.setStatus(stInfo, "cancelled")
			case err != nil:
				u.setStatus(stErr, err.Error())
			default:
				msg := "saved " + filepath.Base(saved[len(saved)-1])
				if len(saved) > 1 {
					msg = fmt.Sprintf("saved %d files", len(saved))
				}
				u.input = nil
				u.setFocus(0)
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
			u.edit(func([]rune) []rune { return nil })
			i++
		case c == 0x17:
			u.edit(func(r []rune) []rune {
				for len(r) > 0 && r[len(r)-1] == ' ' {
					r = r[:len(r)-1]
				}
				for len(r) > 0 && r[len(r)-1] != ' ' && r[len(r)-1] != '/' {
					r = r[:len(r)-1]
				}
				return r
			})
			i++
		case c == 0x7f || c == 0x08:
			u.edit(func(r []rune) []rune {
				if len(r) > 0 {
					return r[:len(r)-1]
				}
				return r
			})
			i++
		case c == '\r' || c == '\n':
			if !u.paste {
				u.enter()
			}
			i++
		case c == '\t':
			u.setFocus((u.focus + 1) % (u.pathFocus() + 1))
			i++
		case c < 0x20:
			i++
		default:
			r, n := utf8.DecodeRune(b[i:])
			if r != utf8.RuneError || n > 1 {
				u.typeRune(r)
			}
			i += n
		}
	}
}

func (u *ui) csi(params string, final byte) {
	n := u.pathFocus() + 1
	switch {
	case strings.HasPrefix(params, "<") && final == 'M':
		var cb, x, y int
		if _, err := fmt.Sscanf(params[1:], "%d;%d;%d", &cb, &x, &y); err == nil && cb == 0 {
			u.click(x, y)
		}
	case params == "200" && final == '~':
		u.paste = true
	case params == "201" && final == '~':
		u.paste = false
	case final == 'A' || final == 'Z':
		u.setFocus((u.focus + n - 1) % n)
	case final == 'B':
		u.setFocus((u.focus + 1) % n)
	case final == 'C':
		u.step(+1)
	case final == 'D':
		u.step(-1)
	}
}

func runTUI(cfg Config, dir string, dirErr error) error {
	t, err := openTTY()
	if err != nil {
		return err
	}
	u := &ui{t: t, cfg: cfg, dir: dir, updates: make(chan func(), 64)}
	u.pathEdit = []rune(shortDir(dir))
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
