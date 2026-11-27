package app

import (
	"image"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/localmedia"
	"netease-music-mygo/internal/netease"
)

type playbackCover struct {
	pixels           int
	backdrop         bool
	prefetchPrevious bool
}

const barCoverPixels = 120
const nowPlayingCoverPixels = 600

var barCoverRequest = playbackCover{pixels: barCoverPixels, prefetchPrevious: true}
var nowPlayingCoverRequest = playbackCover{pixels: nowPlayingCoverPixels, backdrop: true}

func (a *app) trackArtwork(tr *netease.Track, px int, backdrop bool) *artwork {
	if !tr.IsLocal() {
		return a.art.get(netease.Sized(a.trackArtworkURL(tr), px), backdrop)
	}
	song := a.playbackLocalSong(tr)
	if !song.Cover {
		return a.art.get(netease.Sized(a.localArtworkURL(tr, song), px), backdrop)
	}
	source := song.embeddedArtworkKey()
	embedded := &a.local.embeddedArtwork
	key := source.artworkKey(px)
	return a.art.requestCover(key, backdrop, true, func(string) (*ui.Bitmap, image.Image, int, error) {
		return decodeArtSized(embedded.get(source, localmedia.Cover), px)
	})
}

func (a *app) trackCover(c *ui.Context, tr *netease.Track, px int, size, radius float32) ui.Element {
	if !tr.IsLocal() {
		return a.listCover(c, a.trackArtworkURL(tr), px, size, radius)
	}
	song := a.playbackLocalSong(tr)
	if !song.Cover {
		return a.listCover(c, a.localArtworkURL(tr, song), px, size, radius)
	}
	source := song.embeddedArtworkKey()
	embedded := &a.local.embeddedArtwork
	return a.coverImage(c, size, radius, func(box ui.Element) *artwork {
		return a.art.requestListCover(a.route, source.artworkKey(px), box, func(string) (*ui.Bitmap, image.Image, int, error) {
			return decodeArtSized(embedded.get(source, localmedia.Cover), px)
		})
	}, artImmediate)
}

func (a *app) hasTrackCover(tr *netease.Track) bool {
	if tr.IsLocal() {
		song := a.playbackLocalSong(tr)
		return song.Cover || a.localArtworkURL(tr, song) != ""
	}
	return a.trackArtworkURL(tr) != ""
}

// playbackTrackArtwork is the common request path for the bar and Now Playing.
func (a *app) playbackTrackArtwork(tr *netease.Track, cover playbackCover) *artwork {
	if tr.IsLocal() {
		return a.trackArtwork(tr, cover.pixels, cover.backdrop)
	}
	return a.playbackArtwork(a.trackArtworkURL(tr), cover)
}

// playbackArtwork requests the visible cover first, then warms the exact
// cache entries this view will use after a skip.
func (a *app) playbackArtwork(url string, cover playbackCover) *artwork {
	current := netease.Sized(url, cover.pixels)
	art := a.art.get(current, cover.backdrop)
	if a.network.offline {
		return art
	}
	next, prev := a.pl.AdjacentTracks()
	if !cover.prefetchPrevious {
		prev = nil
	}
	seen := current
	for _, tr := range [...]*netease.Track{next, prev} {
		if tr == nil {
			continue
		}
		url := a.trackArtworkURL(tr)
		if url == "" {
			continue
		}
		key := netease.Sized(url, cover.pixels)
		if key != current && key != seen {
			a.art.prefetch(key, cover.backdrop)
			seen = key
		}
	}
	return art
}
