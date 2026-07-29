package fetcher

import "sync"

type LyricTuple struct {
	filePath, data string
}

type LyricsCache struct {
	mu sync.Mutex
	cache []LyricTuple
}

func (lc *LyricsCache) addToCache(filePath string, data string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.cache = append(lc.cache, LyricTuple{filePath, data})
}
