package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Config holds the clickable settings. Zero values are never used; see defaults().
type Config struct {
	Mode    string `json:"mode"`    // video | audio
	Res     string `json:"res"`     // max | 2160 | 1440 | 1080 | 720 | 480
	AQ      string `json:"aq"`      // best | 320 | 192 | 128
	Cookies string `json:"cookies"` // "" or a yt-dlp browser argument
	Dir     string `json:"dir"`     // "" = the Downloads folder
}

func defaults() Config {
	return Config{Mode: "video", Res: "max", AQ: "best"}
}

func in(v string, list ...string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func (c *Config) sanitize() {
	d := defaults()
	if !in(c.Mode, "video", "audio") {
		c.Mode = d.Mode
	}
	if !in(c.Res, "max", "2160", "1440", "1080", "720", "480") {
		c.Res = d.Res
	}
	if !in(c.AQ, "best", "320", "192", "128") {
		c.AQ = d.AQ
	}
	if !validBrowser(c.Cookies) {
		c.Cookies = d.Cookies
	}
}

func configPath() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "snag", "config.json")
}

func loadConfig() Config {
	c := defaults()
	if p := configPath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &c)
		}
	}
	c.sanitize()
	return c
}

func (c Config) save() {
	p := configPath()
	if p == "" {
		return
	}
	if os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return
	}
	if b, err := json.MarshalIndent(c, "", "  "); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}

func isTermux() bool {
	return strings.Contains(os.Getenv("PREFIX"), "com.termux")
}

// downloadsDir returns the user's Downloads folder, creating it if needed.
// Output always goes here.
func downloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var dir string
	switch {
	case isTermux():
		for _, c := range []string{
			filepath.Join(home, "storage", "downloads"),
			"/storage/emulated/0/Download",
		} {
			if st, err := os.Stat(c); err == nil && st.IsDir() {
				return c, nil
			}
		}
		return "", errors.New("no storage access. Run termux-setup-storage, then restart snag")
	case runtime.GOOS == "linux":
		if out, err := exec.Command("xdg-user-dir", "DOWNLOAD").Output(); err == nil {
			if p := strings.TrimSpace(string(out)); p != "" && p != home {
				dir = p
			}
		}
	}
	if dir == "" {
		dir = filepath.Join(home, "Downloads")
	}
	return dir, os.MkdirAll(dir, 0o755)
}

// expandDir turns "~/x" into an absolute path.
func expandDir(s string) string {
	s = strings.TrimSpace(s)
	home, err := os.UserHomeDir()
	switch {
	case s == "":
		return ""
	case err == nil && s == "~":
		return home
	case err == nil && strings.HasPrefix(s, "~/"):
		return filepath.Join(home, s[2:])
	}
	return s
}

// shortDir is the inverse, for display.
func shortDir(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// startDir is where files go: the saved path, else the Downloads folder.
func startDir(c Config) (string, error) {
	if c.Dir != "" {
		return expandDir(c.Dir), nil
	}
	return downloadsDir()
}
