package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// A browser counts as installed when its profile folder exists, since
// that is what yt-dlp reads cookies from. Firefox forks are passed to
// yt-dlp as "firefox:<profile dir>".
type browserDef struct {
	label string
	arg   string   // yt-dlp name; empty = firefox fork, use dir
	dirs  []string // relative to $HOME
	win   []string // relative to %LOCALAPPDATA% ("L:") or %APPDATA% ("A:")
}

var knownBrowsers = []browserDef{
	{"firefox", "firefox", []string{".mozilla/firefox", "snap/firefox/common/.mozilla/firefox", ".var/app/org.mozilla.firefox/.mozilla/firefox", "Library/Application Support/Firefox"}, []string{"A:Mozilla/Firefox"}},
	{"chrome", "chrome", []string{".config/google-chrome", ".var/app/com.google.Chrome/config/google-chrome", "Library/Application Support/Google/Chrome"}, []string{"L:Google/Chrome/User Data"}},
	{"chromium", "chromium", []string{".config/chromium", "snap/chromium/common/chromium", ".var/app/org.chromium.Chromium/config/chromium"}, []string{"L:Chromium/User Data"}},
	{"brave", "brave", []string{".config/BraveSoftware/Brave-Browser", ".var/app/com.brave.Browser/config/BraveSoftware/Brave-Browser", "Library/Application Support/BraveSoftware/Brave-Browser"}, []string{"L:BraveSoftware/Brave-Browser/User Data"}},
	{"edge", "edge", []string{".config/microsoft-edge", "Library/Application Support/Microsoft Edge"}, []string{"L:Microsoft/Edge/User Data"}},
	{"opera", "opera", []string{".config/opera", "Library/Application Support/com.operasoftware.Opera"}, []string{"A:Opera Software/Opera Stable"}},
	{"vivaldi", "vivaldi", []string{".config/vivaldi", "Library/Application Support/Vivaldi"}, []string{"L:Vivaldi/User Data"}},
	{"safari", "safari", []string{"Library/Containers/com.apple.Safari"}, nil},
	{"zen", "", []string{".config/zen", ".zen", ".var/app/app.zen_browser.zen/.zen", ".var/app/app.zen_browser.zen/config/zen"}, []string{"A:zen"}},
	{"librewolf", "", []string{".librewolf", ".var/app/io.gitlab.librewolf-community/.librewolf"}, []string{"A:librewolf"}},
	{"floorp", "", []string{".floorp", ".var/app/one.ablaze.floorp/.floorp"}, []string{"A:Floorp"}},
	{"waterfox", "", []string{".waterfox"}, []string{"A:Waterfox"}},
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

var (
	browserOnce sync.Once
	browserList []opt
)

// browsers returns "none" plus every installed browser.
func browsers() []opt {
	browserOnce.Do(func() {
		browserList = []opt{{"none", ""}}
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		for _, b := range knownBrowsers {
			var dirs []string
			if runtime.GOOS == "windows" {
				for _, w := range b.win {
					base := os.Getenv("LOCALAPPDATA")
					if strings.HasPrefix(w, "A:") {
						base = os.Getenv("APPDATA")
					}
					if base != "" {
						dirs = append(dirs, filepath.Join(base, w[2:]))
					}
				}
			} else {
				for _, d := range b.dirs {
					dirs = append(dirs, filepath.Join(home, d))
				}
			}
			for _, d := range dirs {
				if !isDir(d) {
					continue
				}
				arg := b.arg
				if arg == "" {
					arg = "firefox:" + d
				}
				browserList = append(browserList, opt{b.label, arg})
				break
			}
		}
	})
	return browserList
}

func validBrowser(v string) bool {
	for _, o := range browsers() {
		if o.val == v {
			return true
		}
	}
	return false
}
