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
	if title != (width >= 689) || artists != (width >= 769) {
		t.Fatalf("wrong progressive song details at %d: title=%v artists=%v", width, title, artists)
	}
	if progress, _ := tt.Find("播放进度"); progress.W < 50 {
		t.Fatalf("compact progress bar is only %v DIPs wide", progress.W)
	}
	info, _ := tt.Find("打开正在播放")
	like, _ := tt.Find("喜欢 宝船")
	if info.X+info.W > like.X {
		t.Fatalf("song cover and like overlap: info=%+v like=%+v", info, like)
	}
	if err := tt.Click("播放队列"); err != nil {
		t.Fatalf("cannot open the queue in a compact window: %v", err)
	}
	if !a.nowPlaying || a.npPane != paneQueue {
		t.Fatal("the compact queue button did not open the queue")
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
