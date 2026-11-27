/*
验证列表封面保留的页面、数量和字节边界，以及压力与失效时的资源释放。
*/

package app

import (
	"fmt"
	"image"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func keptListImage(size int) *artwork {
	return &artwork{bmp: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 4, 4))), size: size}
}

// The return allowance favors newer pages and never adds bytes outside the cache budget.
func TestListCoverRetentionByteAndPageLimits(t *testing.T) {
	for _, test := range []struct{ cost, kept int }{{1 << 20, 3}, {3 << 20, 2}} {
		t.Run(fmt.Sprint(test.cost), func(t *testing.T) {
			c := newArtCache(func(func()) {})
			t.Cleanup(c.close)
			for page := range 5 {
				key := fmt.Sprintf("list-%d", page)
				c.add(key, keptListImage(test.cost))
				c.noteListCover(route{page: pagePlaylist, id: int64(page)}, key)
				c.beginFrame()
			}
			c.add("large", keptListImage(c.limit-test.cost*test.kept))
			runtime.GC()
			for page := range 5 {
				key := fmt.Sprintf("list-%d", page)
				if (c.art[key] != nil) != (page >= 5-test.kept) {
					t.Fatalf("wrong retention for %s under pressure", key)
				}
			}
			s := c.stats()
			if s.Bytes > c.limit || s.RetainedListBytes != test.cost*test.kept || s.RetainedListEntries != test.kept || s.Evictions != uint64(5-test.kept) {
				t.Fatalf("retention exceeded its shared/page allowance: %+v", s)
			}
		})
	}
}

// Scrolling replaces a page's viewport; neither overscan nor past screens accumulate protection.
func TestListCoverRetentionTracksLastViewport(t *testing.T) {
	c := newArtCache(func(func()) {})
	t.Cleanup(c.close)
	c.limit = 10000
	page := route{page: pagePlaylist, id: 1}
	for i := range 40 {
		key := fmt.Sprint(i)
		c.add(key, keptListImage(100))
		c.noteListCover(page, key)
	}
	c.beginFrame()
	if s := c.stats(); s.RetainedListEntries != 30 {
		t.Fatalf("one viewport retained too many covers: %+v", s)
	}
	for i := 30; i < 40; i++ {
		c.noteListCover(page, fmt.Sprint(i))
	}
	c.beginFrame()
	c.add("large", keptListImage(9000))
	for i := range 40 {
		if (c.art[fmt.Sprint(i)] != nil) != (i >= 30) {
			t.Fatalf("retained a scrolled-past cover or evicted the latest viewport at %d", i)
		}
	}
	if s := c.stats(); s.Bytes > c.limit || s.RetainedListEntries != 10 {
		t.Fatalf("viewport accumulated old cover costs: %+v", s)
	}
}

// Shared album images count once, and visible work can release the oldest protection at the hard limit.
func TestListCoverRetentionSharesImagesAndYieldsToHardLimit(t *testing.T) {
	c := newArtCache(func(func()) {})
	t.Cleanup(c.close)
	c.limit = 100
	c.add("shared", keptListImage(20))
	for page := range 3 {
		key := fmt.Sprint(page)
		c.add(key, keptListImage(20))
		p := route{page: pagePlaylist, id: int64(page)}
		c.noteListCover(p, "shared")
		c.noteListCover(p, key)
		c.beginFrame()
	}
	if s := c.stats(); s.RetainedListBytes != 80 || s.RetainedListEntries != 4 {
		t.Fatalf("shared album was charged more than once: %+v", s)
	}
	c.add("visible-large", keptListImage(40))
	if c.art["0"] != nil || c.art["1"] == nil || c.art["2"] == nil || c.art["shared"] == nil || c.size > c.limit {
		t.Fatal("hard pressure did not release the oldest protection first")
	}
}

// Ordinary playback artwork is not promoted to list retention, and revocation cannot resurrect a retained image.
func TestListCoverRetentionDoesNotPinPlaybackOrRevokedImages(t *testing.T) {
	c := newArtCache(func(func()) {})
	t.Cleanup(c.close)
	c.now = func() time.Time { return time.Unix(100, 0) }
	c.add("playback", keptListImage(100))
	c.get("playback", false)
	if c.stats().RetainedListEntries != 0 {
		t.Fatal("playback artwork acquired list protection")
	}
	c.add("list", keptListImage(100))
	c.noteListCover(route{page: pagePlaylist}, "list")
	c.beginFrame()
	c.imageChanged("list", netease.ImageRevoked)
	runtime.GC()
	if c.stats().RetainedListBytes != 0 || bitmap(c.get("list", false)) != nil {
		t.Fatal("revoked image remained retained or came back from a weak reference")
	}
}
