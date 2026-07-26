package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lrclib_fetcher/arguments"
	"lrclib_fetcher/metadata"
	"lrclib_fetcher/util"
)

type LyricTuple struct {
	filePath, data string
}

type LyricsCache struct {
	mu sync.Mutex
	cache []LyricTuple
}

type GetResponse struct {
	PlainLyrics string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

}

func (lc *LyricsCache) addToCache(filePath string, data string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.cache = append(lc.cache, LyricTuple{filePath, data})
}

func fetchLyrics(tasks []metadata.NecessaryData, stats *Statistics, lyricJobs int, maxRetries int, logger *util.Logger) LyricsCache {
	var wg sync.WaitGroup
	cache := LyricsCache {
		cache: make([]LyricTuple, 0, len(tasks)),
	}
	taskCh := make(chan metadata.NecessaryData, len(tasks))

	total := len(tasks)
	width := len(fmt.Sprintf("%d", total))

	var counter atomic.Int64

	client := &http.Client{Timeout: 12 * time.Second}

	for range lyricJobs {
		wg.Go(func() {
			for task := range taskCh {
				curr := counter.Add(1)

				params := url.Values{}
				params.Add("artist_name", task.ArtistName)
				params.Add("track_name", task.TrackName)
				params.Add("album_name", task.AlbumName)
				params.Add("duration", task.Duration)

				requestGet := APIBase + "get?" + params.Encode()

				logger.Debug("%s", requestGet)
				var syncedLyrics, plainLyrics string
				var fetched, notFound bool

				for attempt := range maxRetries - 1 {
					req, err := http.NewRequest("GET", requestGet, nil)
					if err != nil {
						fmt.Println(err)
						continue
					}

					req.Header.Set("User-Agent", "MyLrcFetcher/1.0")

					respGet, err := client.Do(req)
					if err != nil {
						if attempt < maxRetries {
							time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
							continue
						}
						logger.Always("[%0*d/%0*d] Network error after retries: %s", width, curr, width, total, task.TrackName)
						break
					}

					func ()  {
						defer respGet.Body.Close()

						if respGet.StatusCode == http.StatusTooManyRequests || respGet.StatusCode >= 500 {
							if attempt < maxRetries - 1 {
								time.Sleep(time.Duration(attempt) * 1 * time.Second)
								return
							}
							logger.Always("[%0*d/%0*d] HTTP %d (Rate limited / Server error): %s", width, curr, width, total, respGet.StatusCode, task.TrackName)
							return
						}

						if respGet.StatusCode == http.StatusNotFound {
							logger.Always("[%0*d/%0*d] Not found (404): %s \n Trying search", width, curr, width, total, task.TrackName)
							stats.notFoundCounter.Add(1)
							notFound = true 
							return
						}

						if respGet.StatusCode != http.StatusOK {
							logger.Always("[%0*d/%0*d] HTTP %d: %s", width, curr, width, total, respGet.StatusCode, task.TrackName)
							return
						}

						var data GetResponse
						if err := json.NewDecoder(respGet.Body).Decode(&data); err != nil {
							logger.Always("[%0*d/%0*d] JSON parse error for %s: %v", width, curr, width, total, task.TrackName, err)
							return
						}

						syncedLyrics = data.SyncedLyrics
						plainLyrics = data.PlainLyrics
						fetched = true

					}()

					if fetched || notFound {
						break
					}
				}
				
				if syncedLyrics != "" {
					logger.Verbose("[%0*d/%0*d] Fetched synced lyrics: %s", width, curr, width, total, task.TrackName)
					stats.syncedCounter.Add(1)
					cache.addToCache(task.FilePath, syncedLyrics)
				} else if plainLyrics != "" {
					logger.Verbose("[%0*d/%0*d] Fetched plain lyrics: %s", width, curr, width, total, task.TrackName)
					stats.plainCounter.Add(1)
					cache.addToCache(task.FilePath, plainLyrics)
				} else if fetched {
					logger.Always("[%0*d/%0*d] Track found but contains no lyrics: %s", width, curr, width, total, task.TrackName)
					stats.failedCounter.Add(1)
				}
			}
		})
	}

	for _, task := range tasks {
		taskCh <- task
	}
	close(taskCh)
	
	wg.Wait()

	return cache
}

func createLyricFiles(lyrics *LyricsCache, jobs int, logger *util.Logger) {
	var wg sync.WaitGroup
	jobCh := make(chan LyricTuple, len(lyrics.cache))

	total := len(lyrics.cache)
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

	for _, tuple := range lyrics.cache {
		jobCh <- tuple
	}
	close(jobCh)

	wg.Wait()
}

func GetLyrics(config arguments.Config) Statistics {
	files, err := util.Glob(config.InputPath, ".flac")
	if err != nil {
		panic("FAILED TO GLOB FLAC")
	}

	if config.ParseMP3 {
		mp3, err := util.Glob(config.InputPath, ".mp3")
		if err != nil {
			panic("FAILED TO GLOB MP3")
		}
		files = append(files, mp3...)
	}

	var stats Statistics
	jobs := min(max(config.Jobs, 0), runtime.NumCPU())
	fetchJobs := max(config.FetchJobs, 0)
	maxRetries := max(config.MaxRetries, 0)
	logger := util.Logger {
		IsVerbose: config.Verbose,
		IsDebug: config.Debug,
	}

	tasks := GetTasks(files, &stats, jobs, config.NoSkip, &logger)

	lyrics := fetchLyrics(tasks, &stats, fetchJobs, maxRetries, &logger)

	createLyricFiles(&lyrics, jobs, &logger)

	return stats
}
