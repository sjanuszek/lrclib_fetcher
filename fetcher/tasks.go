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

func hasLrcFile(path string) bool {
    ext := filepath.Ext(path)
    lrcPath := strings.TrimSuffix(path, ext) + ".lrc"
    _, err := os.Stat(lrcPath)
    return err == nil
}

func GetTasks(files []string, stats *Statistics, jobs int, noSkip bool, logger *util.Logger) []metadata.NecessaryData {
	var wg sync.WaitGroup
	var mu sync.Mutex

	tasks := make([]metadata.NecessaryData, 0, len(files))
	filesCh := make(chan string, len(files))

	total := len(files)
	width := len(fmt.Sprintf("%d", total))

	var counter atomic.Int64

	for range jobs {
		wg.Go(func() {
			for file_name := range filesCh {
				curr := counter.Add(1)
				stats.filesCounter.Add(1)

				if !noSkip && hasLrcFile(file_name) {
					logger.Verbose("[%0*d/%0*d] Skipping: %s", width, curr, width, total, file_name)
					stats.skippedCounter.Add(1)
					continue
				}

				file, err := os.Open(file_name)
				if err != nil {
					logger.Always("[%0*d/%0*d] Failed to open: %s", width, curr, width, total, file_name)
					continue
				}

				if m, ok := metadata.GetTags(file); ok {
					logger.Verbose("[%0*d/%0*d] Getting metadata: %s", width, curr, width, total, file_name)
					stats.processedCounter.Add(1)
					mu.Lock()
					tasks = append(tasks, m)
					mu.Unlock()
				}

				file.Close()
			}
		})
	}

	for _, file := range files {
		filesCh <- file
	}
	close(filesCh)

	wg.Wait()
	return tasks
}

