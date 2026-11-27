package app

import (
	"container/list"
	"fmt"
	"sync"
)

const embeddedArtworkLimit = 8 << 20

type embeddedArtworkKey struct {
	path        string
	stamp, size int64
}

// artworkKey shares decoded sizes between lists and playback, while a rescan
// gives changed embedded pictures a new identity.
func (k embeddedArtworkKey) artworkKey(pixels int) string {
	return fmt.Sprintf("local-cover/%d/%d/%d/%s", k.stamp, k.size, pixels, k.path)
}

func (s localSong) embeddedArtworkKey() embeddedArtworkKey {
	return embeddedArtworkKey{s.Path, s.Stamp, s.Size}
}

type embeddedArtwork struct {
	key   embeddedArtworkKey
	ready chan struct{}
	data  []byte
	entry *list.Element
}

// The player and the different UI sizes share encoded artwork. Keep one
// in-flight read per file and evict encoded bytes independently of bitmaps.
type embeddedArtworkCache struct {
	mu    sync.Mutex
	files map[embeddedArtworkKey]*embeddedArtwork
	order list.List
	bytes int
}

func (c *embeddedArtworkCache) get(key embeddedArtworkKey, read func(string) []byte) []byte {
	c.mu.Lock()
	if found := c.files[key]; found != nil {
		if found.entry != nil {
			c.order.MoveToFront(found.entry)
		}
		c.mu.Unlock()
		<-found.ready
		return found.data
	}
	f := &embeddedArtwork{key: key, ready: make(chan struct{})}
	if c.files == nil {
		c.files = make(map[embeddedArtworkKey]*embeddedArtwork)
	}
	c.files[key] = f
	c.mu.Unlock()

	data := read(key.path)
	c.mu.Lock()
	f.data = data
	if len(data) == 0 || len(data) > embeddedArtworkLimit {
		delete(c.files, key)
	} else {
		f.entry = c.order.PushFront(f)
		c.bytes += len(data)
		for c.bytes > embeddedArtworkLimit {
			old := c.order.Remove(c.order.Back()).(*embeddedArtwork)
			delete(c.files, old.key)
			c.bytes -= len(old.data)
		}
	}
	close(f.ready)
	c.mu.Unlock()
	return data
}
