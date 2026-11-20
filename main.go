// snag: paste a link, get the file in Downloads.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

var errNoTTY = errors.New("no terminal")

const usage = `snag - download video, audio and images from almost anywhere

  snag            open the interactive screen (settings are clickable)
  snag <url>...   download straight away with your saved settings
  snag serve [port]  run the browser version at http://localhost:8787

Files go to your Downloads folder. Needs yt-dlp and ffmpeg;
gallery-dl is optional and adds image posts.`

func normalize(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !strings.Contains(s, "://") {
			s = "https://" + s
		}
		out = append(out, s)
	}
	return out
}

func plain(cfg Config, dir string, urls []string) int {
	if dir == "" {
		fmt.Fprintln(os.Stderr, "snag: no downloads folder (Termux: run termux-setup-storage)")
		return 1
	}
	fmt.Println("Saving to", dir)
	saved, err := download(context.Background(), cfg, dir, normalize(urls), func(s string) {
		fmt.Fprintf(os.Stderr, "\r\x1b[2K%s", s)
	})
	fmt.Fprint(os.Stderr, "\r\x1b[2K")
	if err != nil {
		fmt.Fprintln(os.Stderr, "snag:", err)
		return 1
	}
	for _, f := range saved {
		fmt.Println(f)
	}
	return 0
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Println(usage)
		return
	}
	if len(args) > 0 && args[0] == "serve" {
		addr := "8787"
		if len(args) > 1 {
			addr = args[1]
		}
		if err := serve(addr); err != nil {
			fmt.Fprintln(os.Stderr, "snag:", err)
			os.Exit(1)
		}
		return
	}
	cfg := loadConfig()
	dir, derr := downloadsDir()
	if len(args) > 0 {
		os.Exit(plain(cfg, dir, args))
	}
	err := runTUI(cfg, dir, derr)
	if errors.Is(err, errNoTTY) {
		fmt.Print("Link: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		urls := strings.Fields(line)
		if len(urls) == 0 {
			os.Exit(2)
		}
		os.Exit(plain(cfg, dir, urls))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "snag:", err)
		os.Exit(1)
	}
}
