package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func dataBin() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "snag", "bin")
}

func ytdlpAsset() string {
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "yt-dlp_linux_aarch64"
		}
		if runtime.GOARCH == "amd64" {
			return "yt-dlp_linux"
		}
	case "darwin":
		return "yt-dlp_macos"
	case "windows":
		return "yt-dlp.exe"
	}
	return ""
}

// ensureYtdlp returns a usable yt-dlp path, downloading the official
// standalone build into snag's own folder when none is installed.
func ensureYtdlp(prog func(string)) (string, error) {
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dir := dataBin()
	local := filepath.Join(dir, name)
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return local, nil
	}
	asset := ytdlpAsset()
	if asset == "" || dir == "" || isTermux() {
		return "", errors.New("yt-dlp not found. Install it: " + ytdlpHint())
	}
	prog("Installing yt-dlp")
	if err := fetch("https://github.com/yt-dlp/yt-dlp/releases/latest/download/"+asset, dir, local); err != nil {
		return "", fmt.Errorf("could not install yt-dlp (%v). Install it: %s", err, ytdlpHint())
	}
	return local, nil
}

func fetch(url, dir, dest string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 3 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	tmp, err := os.CreateTemp(dir, "dl-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

func pm() string {
	switch {
	case isTermux():
		return "pkg install"
	case runtime.GOOS == "darwin":
		return "brew install"
	case runtime.GOOS == "windows":
		return "winget install"
	}
	for _, c := range [][2]string{{"pacman", "sudo pacman -S"}, {"apt", "sudo apt install"},
		{"dnf", "sudo dnf install"}, {"zypper", "sudo zypper install"}, {"apk", "sudo apk add"}} {
		if _, err := exec.LookPath(c[0]); err == nil {
			return c[1]
		}
	}
	return "install"
}

func ffmpegHint() string {
	if runtime.GOOS == "windows" {
		return "winget install Gyan.FFmpeg"
	}
	return pm() + " ffmpeg"
}

func ytdlpHint() string {
	if runtime.GOOS == "windows" {
		return "winget install yt-dlp.yt-dlp"
	}
	return pm() + " yt-dlp"
}
