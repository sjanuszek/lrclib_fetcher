package fetcher

import (
	"fmt"
	"lrclib_fetcher/metadata"
	"lrclib_fetcher/util"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	Resolved []LyricTuple
	Unresolved []UnresolvedLyrics
}

func (lc *LyricsCache) AddToResolved(filePath string, data string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.Resolved = append(lc.Resolved, LyricTuple{filePath, data})
}

func (lc *LyricsCache) addToUnresolved(candidates []SearchResponse, data metadata.NecessaryData) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.Unresolved = append(lc.Unresolved, UnresolvedLyrics{candidates, data})
}

func (cache *LyricsCache) CreateLyricFiles(jobs int, logger *util.Logger) {
	var wg sync.WaitGroup
	jobCh := make(chan LyricTuple, len(cache.Resolved))

	total := len(cache.Resolved)
	width := len(fmt.Sprintf("%d", total))

	var counter atomic.Int64

	for range jobs {
		wg.Go(func() {
			for tuple := range jobCh {
				curr := counter.Add(1)
				ext := filepath.Ext(tuple.filePath)
				new_path := strings.TrimSuffix(tuple.filePath, ext) + ".lrc"

				logger.Verbose("[%0*d/%0*d] Writing: %s", width, curr, width, total, new_path)

				err := os.WriteFile(
					new_path,
					[]byte(tuple.data),
					0644,
				)

				if err != nil {
					fmt.Println(err)
				}
			}
		})
	}

	for _, tuple := range cache.Resolved {
		jobCh <- tuple
	}
	close(jobCh)

	wg.Wait()
}

