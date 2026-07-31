package fetcher

import (
	"lrclib_fetcher/metadata"
	"sync"
)

type LyricTuple struct {
	filePath, data string
}

type UnresolvedLyrics struct {
	Candidates []SearchResponse
	Data metadata.NecessaryData
}

type LyricsCache struct {
	mu sync.Mutex
	resolved []LyricTuple
	unresolved []UnresolvedLyrics
}

func (lc *LyricsCache) addToResolved(filePath string, data string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.resolved = append(lc.resolved, LyricTuple{filePath, data})
}

func (lc *LyricsCache) addToUnresolved(candidates []SearchResponse, data metadata.NecessaryData) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.unresolved = append(lc.unresolved, UnresolvedLyrics{candidates, data})
}
