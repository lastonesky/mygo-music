package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// A window snapped to half a display keeps its player actions and a usable
// progress bar instead of forcing controls past the right edge.
func TestCompactWindowKeepsPlayerControlsUsable(t *testing.T) {
	for _, width := range []int{640, 688, 689, 768, 769} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			testCompactPlayer(t, width)
		})
	}
}

func testCompactPlayer(t *testing.T, width int) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000}})
	tt := ui.NewTester(a.view, 1180, 780)
	tt.Frame()
	tt.SetSize(width, 480)
	tt.Frame()

	for _, label := range []string{"播放栏", "当前歌曲", "播放控制", "播放器操作", "打开正在播放", "喜欢 宝船", "播放/暂停", "播放进度", "桌面歌词", "评论", "播放队列", "音量"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("compact player lost %q", label)
		}
		if r.X < 168 || r.X+r.W > float32(width) {
			t.Errorf("%q is outside the compact window: %+v", label, r)
		}
		if label != "播放进度" && r.W < 32 {
			t.Errorf("%q was clipped to %v DIPs: %+v", label, r.W, r)
		}
	}
	_, title := tt.Find("歌曲名称")
	_, artists := tt.Find("歌曲作者")
	if title != artists || title != (width >= 689) {
		t.Fatalf("title and artists must hide together at %d: title=%v artists=%v", width, title, artists)
	}
	if progress, _ := tt.Find("播放进度"); progress.W < 50 {
		t.Fatalf("compact progress bar is only %v DIPs wide", progress.W)
	}
	info, _ := tt.Find("打开正在播放")
	like, _ := tt.Find("喜欢 宝船")
	if info.X+info.W > like.X {
		t.Fatalf("song cover and like overlap: info=%+v like=%+v", info, like)
	}
	cover, ok := tt.Find("歌曲封面")
	if !ok || cover.W != 48 || cover.H != 48 {
		t.Fatalf("compact cover does not match its drawing: %+v", cover)
	}
	if title {
		titleBounds, _ := tt.Find("歌曲名称")
		artistBounds, _ := tt.Find("歌曲作者")
		if cover.X+cover.W > titleBounds.X || titleBounds.X+titleBounds.W > like.X ||
			titleBounds.Y >= artistBounds.Y || artistBounds.Y+artistBounds.H > cover.Y+cover.H {
			t.Fatalf("cover and two-line song details overlap: cover=%+v title=%+v artists=%+v", cover, titleBounds, artistBounds)
		}
	}
	if err := tt.Click("播放队列"); err != nil {
		t.Fatalf("cannot open the queue in a compact window: %v", err)
	}
	if !a.nowPlaying || a.npPane != paneQueue {
		t.Fatal("the compact queue button did not open the queue")
	}
}

// A snapped window leaves Now Playing's queue and comments a readable column
// of their own, instead of the few dozen DIPs two desktop margins and a wide
// gap left beside the cover.
func TestCompactNowPlayingKeepsItsPaneReadable(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "宝船", Duration: 200000}, {ID: 2, Name: "春日影", Duration: 199000}})
	a.pl.SetLyrics(nil)
	a.nowPlaying, a.npPane = true, paneQueue
	tt := ui.NewTesterWith(a.view, 640, 480, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	tt.Frame()
	tt.Advance(npSettle)
	info, ok := tt.AccessByRole(ui.RoleButton, "宝船")
	if !ok {
		t.Fatal("no queue row beside the cover")
	}
	row := info.Bounds
	if row.W < 260 || row.X+row.W > 640 {
		t.Fatalf("the queue row is %v DIPs wide at x %v, not a readable column inside the window", row.W, row.X)
	}
	if err := tt.Click("歌词"); err != nil {
		t.Fatalf("cannot leave the queue: %v", err)
	}
	if a.npPane == paneQueue {
		t.Fatal("the tabs stopped taking clicks in the compact queue")
	}
}

// With NM_FRAMES set, saves the home page and player at the compact minimum
// width and at the width just before the normal layout takes over.
func TestCompactWindowFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, width := range []int{640, 999} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.pl.Cue([]netease.Track{{ID: 7, Name: "春日影", Artists: []netease.Artist{{Name: "MyGO!!!!!"}}, Duration: 199000}})
		tt := ui.NewTester(a.view, width, 600)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("compact-%d.png", width)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
