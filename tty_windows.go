//go:build windows

package main

import "os"

// The interactive UI needs a unix terminal. On Windows snag falls back to a
// plain prompt (or run it inside WSL).
type tty struct{ f *os.File }

func openTTY() (*tty, error)              { return nil, errNoTTY }
func (t *tty) restore()                   {}
func (t *tty) size() (int, int)           { return 80, 24 }
func notifySignals(_, _ chan<- os.Signal) {}
