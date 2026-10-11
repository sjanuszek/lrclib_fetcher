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

type LyricTruple struct {
	filePath, data, ext string
}

type UnresolvedLyrics struct {
	Candidates []SearchResponse
	Data metadata.NecessaryData
}

type LyricsCache struct {
	mu sync.Mutex
	Resolved []LyricTruple
	Unresolved []UnresolvedLyrics
}

func (lc *LyricsCache) AddToResolved(filePath, data, ext string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.Resolved = append(lc.Resolved, LyricTruple{filePath, data, ext})
}

func (lc *LyricsCache) addToUnresolved(candidates []SearchResponse, data metadata.NecessaryData) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.Unresolved = append(lc.Unresolved, UnresolvedLyrics{candidates, data})
}

func (cache *LyricsCache) CreateLyricFiles(jobs int, logger *util.Logger) {
	var wg sync.WaitGroup
	jobCh := make(chan LyricTruple, len(cache.Resolved))

	total := len(cache.Resolved)
	width := len(fmt.Sprintf("%d", total))

	var counter atomic.Int64

	for range jobs {
		wg.Go(func() {
			for truple := range jobCh {
				curr := counter.Add(1)
				ext := filepath.Ext(truple.filePath)
				new_path := strings.TrimSuffix(truple.filePath, ext) + truple.ext

				logger.Verbose("[%0*d/%0*d] Writing: %s", width, curr, width, total, new_path)

				err := os.WriteFile(
					new_path,
					[]byte(truple.data),
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

