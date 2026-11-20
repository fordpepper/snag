//go:build !windows

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

type tty struct {
	f     *os.File
	saved string
}

func openTTY() (*tty, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, errNoTTY
	}
	t := &tty{f: f}
	out, err := t.stty("-g")
	if err != nil {
		f.Close()
		return nil, errNoTTY
	}
	t.saved = strings.TrimSpace(out)
	if _, err := t.stty("raw", "-echo"); err != nil {
		f.Close()
		return nil, errNoTTY
	}
	return t, nil
}

func (t *tty) stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = t.f
	out, err := cmd.Output()
	return string(out), err
}

func (t *tty) restore() {
	t.stty(t.saved)
	t.f.Close()
}

func (t *tty) size() (cols, rows int) {
	cols, rows = 80, 24
	out, err := t.stty("size")
	if err != nil {
		return
	}
	p := strings.Fields(out)
	if len(p) == 2 {
		if r, e := strconv.Atoi(p[0]); e == nil && r > 0 {
			rows = r
		}
		if c, e := strconv.Atoi(p[1]); e == nil && c > 0 {
			cols = c
		}
	}
	return
}

func notifySignals(resize, quit chan<- os.Signal) {
	signal.Notify(resize, syscall.SIGWINCH)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGHUP)
}
