//go:build linux

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The installer's absolute icon path becomes a hicolor icon, which launchers
// that only look up freedesktop icon names can display.
func TestLauncherIconUsesFreedesktopName(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	resources := filepath.Join(root, "resources")
	exe := filepath.Join(root, "mygo-cloudmusic.app", "mygo-cloudmusic")
	entry := filepath.Join(data, "applications", "mygo-cloudmusic.desktop")
	for _, dir := range []string{filepath.Dir(exe), resources, filepath.Dir(entry)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(exe, []byte("app"), 0o755); err != nil {
		t.Fatal(err)
	}
	icon := []byte("current icon")
	if err := os.WriteFile(filepath.Join(resources, "icon.png"), icon, 0o644); err != nil {
		t.Fatal(err)
	}
	original := "[Desktop Entry]\nExec=\"" + exe + "\" %U\nIcon=" + exe + ".png\n"
	if err := os.WriteFile(entry, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncLauncherIcon(data, exe, resources); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Icon=mygo-cloudmusic\n") {
		t.Fatalf("desktop entry does not use the registered icon name:\n%s", got)
	}
	if strings.Contains(string(got), "Icon="+exe+".png") {
		t.Fatalf("desktop entry still requires absolute-path support:\n%s", got)
	}
	registered, err := os.ReadFile(filepath.Join(data, "icons", "hicolor", "1024x1024", "apps", "mygo-cloudmusic.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(registered) != string(icon) {
		t.Fatal("the registered icon is not the app's icon")
	}

	// Updates restore the absolute path; the next launch registers the new
	// resource again, instead of treating the already named icon as finished.
	if err := os.WriteFile(entry, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	icon = []byte("updated icon")
	if err := os.WriteFile(filepath.Join(resources, "icon.png"), icon, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncLauncherIcon(data, exe, resources); err != nil {
		t.Fatal(err)
	}
	registered, err = os.ReadFile(filepath.Join(data, "icons", "hicolor", "1024x1024", "apps", "mygo-cloudmusic.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(registered) != string(icon) {
		t.Fatal("an updated app did not refresh the registered icon")
	}
}

// Running a portable build never rewrites an unrelated install's entry.
func TestLauncherIconLeavesAnotherInstallAlone(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	resources := filepath.Join(root, "resources")
	exe := filepath.Join(root, "portable", "mygo-cloudmusic")
	installed := filepath.Join(root, "installed", "mygo-cloudmusic")
	entry := filepath.Join(data, "applications", "mygo-cloudmusic.desktop")
	for _, dir := range []string{filepath.Dir(exe), filepath.Dir(installed), resources, filepath.Dir(entry)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{exe, installed} {
		if err := os.WriteFile(file, []byte(file), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	original := "[Desktop Entry]\nExec=\"" + installed + "\" %U\nIcon=" + installed + ".png\n"
	if err := os.WriteFile(entry, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncLauncherIcon(data, exe, resources); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatal("changed another install's desktop entry")
	}
}
