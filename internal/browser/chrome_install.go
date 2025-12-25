package browser

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/go-rod/rod/lib/launcher"
)

// InstallChrome downloads a Chromium build for the current OS/arch and installs deps if needed.
func InstallChrome(ctx context.Context, revision int) (string, error) {
	if err := InstallChromeDependencies(ctx); err != nil {
		return "", err
	}

	downloader := launcher.NewBrowser()
	downloader.Context = ctx
	if revision > 0 {
		downloader.Revision = revision
	}

	path, err := downloader.Get()
	if err != nil {
		return "", fmt.Errorf("failed to download chrome: %w", err)
	}

	return path, nil
}

// InstallChromeDependencies installs OS packages required by Chromium.
func InstallChromeDependencies(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	if path, _ := exec.LookPath("apt-get"); path != "" {
		if err := runCommand(ctx, path, "update"); err != nil {
			return err
		}
		args := append([]string{"install", "-y", "--no-install-recommends"}, chromeDepsApt...)
		return runCommand(ctx, path, args...)
	}

	if path, _ := exec.LookPath("dnf"); path != "" {
		args := append([]string{"install", "-y"}, chromeDepsDnf...)
		return runCommand(ctx, path, args...)
	}

	if path, _ := exec.LookPath("yum"); path != "" {
		args := append([]string{"install", "-y"}, chromeDepsYum...)
		return runCommand(ctx, path, args...)
	}

	if path, _ := exec.LookPath("apk"); path != "" {
		args := append([]string{"add", "--no-cache"}, chromeDepsApk...)
		return runCommand(ctx, path, args...)
	}

	return fmt.Errorf("no supported package manager found for Chrome dependencies")
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v failed: %w\n%s", name, args, err, out.String())
	}
	return nil
}

var chromeDepsApt = []string{
	"ca-certificates",
	"fonts-liberation",
	"libasound2",
	"libatk-bridge2.0-0",
	"libatk1.0-0",
	"libcups2",
	"libdbus-1-3",
	"libdrm2",
	"libgbm1",
	"libgtk-3-0",
	"libnspr4",
	"libnss3",
	"libx11-xcb1",
	"libxcomposite1",
	"libxdamage1",
	"libxfixes3",
	"libxrandr2",
	"libxshmfence1",
	"libxss1",
	"libxtst6",
	"libpango-1.0-0",
	"libpangocairo-1.0-0",
	"libxkbcommon0",
}

var chromeDepsDnf = []string{
	"alsa-lib",
	"atk",
	"cups-libs",
	"gtk3",
	"libX11",
	"libXcomposite",
	"libXdamage",
	"libXrandr",
	"libXfixes",
	"libX11-xcb",
	"libxcb",
	"libxkbcommon",
	"libxshmfence",
	"nss",
	"nspr",
	"pango",
	"mesa-libgbm",
	"libdrm",
}

var chromeDepsYum = chromeDepsDnf

var chromeDepsApk = []string{
	"ca-certificates",
	"freetype",
	"harfbuzz",
	"nss",
	"ttf-freefont",
	"alsa-lib",
	"atk",
	"at-spi2-atk",
	"cups-libs",
	"libxcomposite",
	"libxdamage",
	"libxrandr",
	"libxfixes",
	"libxkbcommon",
	"libx11",
	"libxrender",
	"libxext",
	"libxcb",
	"libdrm",
	"mesa-gbm",
	"gtk+3.0",
	"pango",
	"cairo",
	"gdk-pixbuf",
	"fontconfig",
	"libstdc++",
	"libgcc",
}
