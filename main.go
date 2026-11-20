// snag: a thin, tidy front end for yt-dlp.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const usage = `snag - download video and audio from almost anywhere

usage:
  snag [options] <url> [url...]

options:
  -a            audio only (mp3)
  -q <height>   max video height, e.g. 720, 1080 (default: best)
  -o <dir>      output directory (default: current)
  -b <browser>  use cookies from a browser (firefox, chrome, ...)
                needed for private or age-gated posts
  -p            download whole playlist / profile when the url is one
  -v            show yt-dlp output
  -h            show this help

needs yt-dlp and ffmpeg on PATH.`

func main() {
	var (
		audio    = flag.Bool("a", false, "")
		quality  = flag.String("q", "", "")
		outDir   = flag.String("o", ".", "")
		browser  = flag.String("b", "", "")
		playlist = flag.Bool("p", false, "")
		verbose  = flag.Bool("v", false, "")
	)
	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	flag.Parse()

	urls := flag.Args()
	if len(urls) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	for _, bin := range []string{"yt-dlp", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			fmt.Fprintf(os.Stderr, "snag: %s not found on PATH (see README for install)\n", bin)
			os.Exit(1)
		}
	}

	args := buildArgs(*audio, *quality, *outDir, *browser, *playlist, *verbose)
	args = append(args, urls...)

	cmd := exec.Command("yt-dlp", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "snag:", err)
		os.Exit(1)
	}
}

func buildArgs(audio bool, quality, outDir, browser string, playlist, verbose bool) []string {
	args := []string{
		"-o", strings.TrimRight(outDir, "/") + "/%(extractor)s - %(title).120B [%(id)s].%(ext)s",
		"--no-mtime",
		"--embed-metadata",
		"--restrict-filenames",
		"--progress",
	}
	if !verbose {
		args = append(args, "--quiet", "--no-warnings")
	}
	if playlist {
		args = append(args, "--yes-playlist")
	} else {
		args = append(args, "--no-playlist")
	}
	if browser != "" {
		args = append(args, "--cookies-from-browser", browser)
	}

	if audio {
		return append(args, "-x", "--audio-format", "mp3", "--audio-quality", "0")
	}

	// Prefer the clean source stream. For TikTok this skips the
	// watermarked "download" variant; merged output is always mp4.
	sel := "bv*+ba/b"
	if quality != "" {
		sel = fmt.Sprintf("bv*[height<=%s]+ba/b[height<=%s]/b", quality, quality)
	}
	return append(args,
		"-f", sel,
		"-S", "res,ext:mp4:m4a",
		"--merge-output-format", "mp4",
	)
}
