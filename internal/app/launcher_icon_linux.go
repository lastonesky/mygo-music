//go:build linux

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// registerLauncherIcon registers the app icon by its freedesktop name for
// launchers such as hyprlauncher, which do not load an absolute Icon= path.
// Only the desktop entry that install.sh created for this executable is
// changed; system packages and manually installed desktop entries are left
// alone. An update's desktop-entry rewrite is repaired on the next launch.
func registerLauncherIcon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(data) {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		data = filepath.Join(home, ".local", "share")
	}
	resources, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return err
	}
	return syncLauncherIcon(data, exe, resources)
}

func syncLauncherIcon(data, exe, resources string) error {
	name := filepath.Base(exe)
	entryPath := filepath.Join(data, "applications", name+".desktop")
	entry, err := os.ReadFile(entryPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	registered := registeredLauncherExec(string(entry))
	if registered == "" || !sameLauncherExecutable(registered, exe) {
		return nil
	}
	icon, err := os.ReadFile(filepath.Join(resources, "icon.png"))
	if err != nil {
		return err
	}
	updated := false
	for _, line := range strings.Split(string(entry), "\n") {
		if line == "Icon="+name || line == "Icon="+filepath.Join(filepath.Dir(registered), name+".png") {
			updated = true
			break
		}
	}
	if !updated {
		return nil
	}
	themePath := filepath.Join(data, "icons", "hicolor")
	iconPath := filepath.Join(themePath, "1024x1024", "apps", name+".png")
	if err := os.MkdirAll(filepath.Dir(iconPath), 0o755); err != nil {
		return fmt.Errorf("create the icon directory: %w", err)
	}
	// An icon directory without an index is invisible to GTK and rofi, even
	// though some launchers scan it as a fallback. Keep an existing user theme.
	indexPath := filepath.Join(themePath, "index.theme")
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		index := "[Icon Theme]\nName=Hicolor\nComment=Fallback icon theme\nHidden=true\nDirectories=1024x1024/apps\n\n[1024x1024/apps]\nSize=1024\nContext=Applications\nType=Fixed\n"
		if err := os.WriteFile(indexPath, []byte(index), 0o644); err != nil {
			return fmt.Errorf("describe the icon theme: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check the icon theme: %w", err)
	}
	if err := os.WriteFile(iconPath, icon, 0o644); err != nil {
		return fmt.Errorf("register the launcher icon: %w", err)
	}
	var b strings.Builder
	for line := range strings.Lines(string(entry)) {
		line = strings.TrimSuffix(line, "\n")
		if line == "Icon="+name || line == "Icon="+filepath.Join(filepath.Dir(registered), name+".png") {
			line = "Icon=" + name
		}
		b.WriteString(line + "\n")
	}
	return os.WriteFile(entryPath, []byte(b.String()), 0o644)
}

func registeredLauncherExec(entry string) string {
	for line := range strings.SplitSeq(entry, "\n") {
		if rest, ok := strings.CutPrefix(line, `Exec="`); ok {
			if exe, _, ok := strings.Cut(rest, `"`); ok && filepath.IsAbs(exe) {
				return exe
			}
		}
	}
	return ""
}

func sameLauncherExecutable(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}
