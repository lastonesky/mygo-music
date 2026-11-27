/*
验证歌曲列表小封面在多页浏览、垃圾回收及异步恢复后无需等待淡入。
*/

package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func listCoverBitmap() *ui.Bitmap {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 255, A: 255}), image.Point{}, draw.Src)
	return ui.NewBitmap(img)
}

func assertListCoverRed(t *testing.T, view *ui.Tester) {
	t.Helper()
	r, ok := view.Find("列表封面")
	if !ok {
		t.Fatal("list cover was not built")
	}
	pixel := view.Image().RGBAAt(int(r.X+r.W/2), int(r.Y+r.H/2))
	if pixel.R < 240 || pixel.G > 20 || pixel.B > 20 {
		t.Fatalf("first available frame showed a placeholder/fade: %v", pixel)
	}
}

// A third recent list must keep its first frame through unrelated large images and GC.
func TestListCoverReturnFirstFrame(t *testing.T) {
	isolateUserData(t)
	a := newTestApp()
	now := time.Unix(1e9, 0)
	a.art = newArtCache(func(func()) {})
	a.art.now = func() time.Time { return now }
	a.art.limit = 64 << 10
	a.art.load = func(string) (*ui.Bitmap, image.Image, int, error) {
		return nil, nil, 0, fmt.Errorf("unexpected reload")
	}
	t.Cleanup(a.art.close)
	tracks := make([]netease.Track, 3)
	for i := range tracks {
		tracks[i] = netease.Track{ID: int64(i + 1), Album: netease.Album{PicURL: fmt.Sprintf("https://p1.music.126.net/list-%d.jpg", i)}}
		putListArtwork(a.art, netease.Sized(tracks[i].Album.PicURL, 96), &artwork{bmp: listCoverBitmap()})
	}
	a.route = route{page: pagePlaylist, id: 1}
	view := ui.NewTester(func(c *ui.Context) {
		a.art.pageFrame(a.route, true)
		if id := a.route.id; a.route.page == pagePlaylist {
			a.trackCover(c, &tracks[id-1], 96, 38, 6).Label("列表封面")
		}
	}, 200, 100)
	for _, id := range []int64{1, 2, 3} {
		a.route = route{page: pagePlaylist, id: id}
		view.Frame()
		view.Frame() // collect the committed visible viewport
		assertListCoverRed(t, view)
	}
	a.route = route{page: pageAlbum, id: 4}
	view.Frame()
	now = now.Add(time.Minute)
	putListArtwork(a.art, "unrelated-thumbnail", &artwork{bmp: listCoverBitmap()})
	putListArtwork(a.art, "large-offscreen", &artwork{bmp: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 96, 96)))})
	runtime.GC()
	t.Logf("after pressure: %+v", a.art.stats())
	a.route = route{page: pagePlaylist, id: 1}
	view.Frame()
	assertListCoverRed(t, view)
	if s := a.art.stats(); s.Loads != 0 || s.ListPlaceholders != 0 || s.RetainedListEntries != 3 || s.Bytes > 64<<10 {
		t.Fatalf("return decoded again, showed a placeholder or lost bounded retention: %+v", s)
	}
}

// A list bitmap arriving after a placeholder is fully visible in its first frame.
func TestListCoverRecoveryDoesNotFade(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprintf("local=%v", local), func(t *testing.T) {
			isolateUserData(t)
			a := newTestApp()
			tr := netease.Track{ID: 1, Album: netease.Album{PicURL: "https://p1.music.126.net/restored.jpg"}}
			key := netease.Sized(tr.Album.PicURL, 96)
			if local {
				tr = netease.Track{LocalID: 1}
				song := localSong{Track: tr, Cover: true, Path: filepath.Join(t.TempDir(), "fixture.flac"), Stamp: 1, Size: 4}
				a.local.replace(localCatalog{Songs: []localSong{song}})
				key = song.embeddedArtworkKey().artworkKey(96)
			}
			a.art = newArtCache(func(func()) {})
			a.art.load = func(string) (*ui.Bitmap, image.Image, int, error) {
				return nil, nil, 0, fmt.Errorf("held cache miss")
			}
			t.Cleanup(a.art.close)
			view := ui.NewTester(func(c *ui.Context) { a.trackCover(c, &tr, 96, 38, 6).Label("列表封面") }, 200, 100)
			view.Frame()
			putListArtwork(a.art, key, &artwork{bmp: listCoverBitmap()})
			view.Frame()
			assertListCoverRed(t, view)
		})
	}
}

// Full virtual lists return with ready row pixels in both themes and at minimum/normal window sizes.
func TestListCoverNavigationFrames(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, size := range [][2]int{{640, 440}, {1180, 780}} {
			t.Run(fmt.Sprintf("dark=%v/%dx%d", dark, size[0], size[1]), func(t *testing.T) {
				isolateUserData(t)
				a := newTestApp()
				a.settings = defaultSettings()
				now := time.Unix(1e9, 0)
				a.art = newArtCache(func(func()) {})
				a.art.now = func() time.Time { return now }
				a.art.limit = 256 << 10
				a.art.load = func(string) (*ui.Bitmap, image.Image, int, error) {
					return nil, nil, 0, fmt.Errorf("fixture cache miss")
				}
				t.Cleanup(a.art.close)
				for page := int64(1); page <= 3; page++ {
					pl := &netease.Playlist{ID: page, Name: fmt.Sprintf("歌单 %d", page)}
					for i := range 3 {
						tr := netease.Track{ID: page*10 + int64(i), Name: fmt.Sprintf("歌曲 %d-%d", page, i),
							Album: netease.Album{PicURL: fmt.Sprintf("https://p1.music.126.net/%d-%d.jpg", page, i)}}
						pl.Tracks = append(pl.Tracks, tr)
						putListArtwork(a.art, netease.Sized(tr.Album.PicURL, 96), &artwork{bmp: listCoverBitmap()})
					}
					a.playlists[page] = pl
				}
				a.route = route{page: pagePlaylist, id: 1}
				view := ui.NewTesterWith(a.view, size[0], size[1], ui.TesterOptions{Dark: dark})
				for page := int64(1); page <= 3; page++ {
					a.route.id = page
					view.Frame()
					view.Frame()
				}
				a.route = route{page: pageCharts}
				view.Frame()
				now = now.Add(time.Minute)
				for i := range 4 {
					putListArtwork(a.art, fmt.Sprintf("unrelated-%d", i), &artwork{bmp: listCoverBitmap()})
				}
				putListArtwork(a.art, "large", &artwork{bmp: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 192, 192)))})
				runtime.GC()
				a.route = route{page: pagePlaylist, id: 1}
				view.Frame()
				row, ok := view.AccessByRole(ui.RoleButton, "歌曲 1-0")
				if !ok {
					t.Fatal("returned list did not build its first song")
				}
				bounds := row.Bounds
				pixel := view.Image().RGBAAt(int(bounds.X)+12+26+14+19, int(bounds.Y+bounds.H/2))
				if pixel.R < 240 || pixel.G > 20 || pixel.B > 20 {
					t.Fatalf("returned row flashed its placeholder: %v, stats: %+v", pixel, a.art.stats())
				}
				if dir := os.Getenv("NM_FRAMES"); dir != "" {
					file, err := os.Create(filepath.Join(dir, fmt.Sprintf("list-return-dark-%v-%dx%d.png", dark, size[0], size[1])))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(file, view.Image())
					closeErr := file.Close()
					if err != nil || closeErr != nil {
						t.Fatalf("write frame: %v / %v", err, closeErr)
					}
				}
			})
		}
	}
}

// Refreshing a displayed playlist with another CDN host must never clear a ready row cover.
func TestListCoverPlaylistRefreshCDNHost(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, size := range [][2]int{{640, 440}, {1180, 780}} {
			t.Run(fmt.Sprintf("dark=%v/%dx%d", dark, size[0], size[1]), func(t *testing.T) {
				isolateUserData(t)
				a := newTestApp()
				a.settings = defaultSettings()
				posts := make(chan func(), 16)
				requests := make(chan string, 16)
				a.art = newArtCache(func(fn func()) { posts <- fn })
				a.art.load = func(url string) (*ui.Bitmap, image.Image, int, error) {
					requests <- url
					img := image.NewRGBA(image.Rect(0, 0, 32, 32))
					draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{B: 255, A: 255}), image.Point{}, draw.Src)
					return ui.NewBitmap(img), img, 0, nil
				}
				t.Cleanup(a.art.close)
				old := netease.Track{ID: 1, Name: "刷新的歌曲", Album: netease.Album{PicURL: "https://p2.music.126.net/fixture/cover.jpg"}}
				fresh := old
				fresh.Album.PicURL = "https://p1.music.126.net/fixture/cover.jpg"
				putListArtwork(a.art, netease.Sized(old.Album.PicURL, 96), &artwork{bmp: listCoverBitmap()})
				a.route = route{page: pagePlaylist, id: 1}
				a.playlists[1] = &netease.Playlist{ID: 1, Name: "缓存的歌单", Tracks: []netease.Track{old}}
				// Check every build, including intermediate visibility/layout builds
				// which Tester.Frame settles before returning its final pixels.

				view := ui.NewTesterWith(a.view, size[0], size[1], ui.TesterOptions{Dark: dark})
				assertRow := func(want color.RGBA) {
					t.Helper()
					row, ok := view.AccessByRole(ui.RoleButton, fresh.Name)
					if !ok {
						t.Fatal("refreshed row missing")
					}
					r := row.Bounds
					pixel := view.Image().RGBAAt(int(r.X)+12+26+14+19, int(r.Y+r.H/2))
					if pixel != want {
						t.Fatalf("refresh flashed placeholder: %v", pixel)
					}
				}
				assertRow(color.RGBA{R: 255, A: 255})
				a.playlists[1] = &netease.Playlist{ID: 1, Name: "刷新的歌单", Tracks: []netease.Track{fresh}}
				for range 5 {
					view.Frame()
					assertRow(color.RGBA{R: 255, A: 255})
				}
				if got := wait(t, requests, "fresh CDN request"); got != netease.Sized(fresh.Album.PicURL, 96) {
					t.Fatalf("request identity changed: %s", got)
				}
				wait(t, posts, "fresh CDN decode")()
				for range 5 {
					view.Frame()
					assertRow(color.RGBA{B: 255, A: 255})
				}
				if s := a.art.stats(); s.ListPlaceholders != 0 {
					t.Fatalf("cover disappeared during intermediate builds: %+v", s)
				}
			})
		}
	}
}

// Seed the same bitmap accounting and display family as an asynchronous decode.
func putListArtwork(c *artCache, key string, value *artwork) {
	value.family = netease.PublicCoverFamily(key)
	value.used, value.loaded = c.now(), c.now()
	value.size = pixelBytes(value.bmp)
	c.add(key, value)
}
