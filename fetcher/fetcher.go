package fetcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lrclib_fetcher/arguments"
	"lrclib_fetcher/util"

	"github.com/dhowden/tag"
)

type NecessaryData struct {
	filePath, trackName, artistName, albumName, duration string
}

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

func hasLrcFile(path string) bool {
    ext := filepath.Ext(path)
    lrcPath := strings.TrimSuffix(path, ext) + ".lrc"
    _, err := os.Stat(lrcPath)
    return err == nil
}

func getTags(file *os.File) (NecessaryData, bool) {
	m, err := tag.ReadFrom(file)
	if err != nil {
		return NecessaryData{}, false
	}

	duration, err := getTrackDuration(file.Name())
	if err != nil {
		return NecessaryData{}, false
	}

	return NecessaryData{
		file.Name(),
		m.Title(),
		m.AlbumArtist(),
		m.Album(),
		duration,
	}, true
}

func getTrackDuration(fileName string) (string, error) {
	cmd := exec.Command("ffprobe", "-i", fileName, "-show_entries", "format=duration", "-v", "quiet", "-of", "csv=p=0",)
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()

	val, _, _ := strings.Cut(strings.TrimSpace(out.String()), ".")

	return val, nil
}

func fetchLyrics(tasks []NecessaryData, stats *Statistics, lyricJobs int, maxRetries int, logger *util.Logger) LyricsCache {
	var wg sync.WaitGroup
	cache := LyricsCache {
		cache: make([]LyricTuple, 0, len(tasks)),
	}
	taskCh := make(chan NecessaryData, len(tasks))

	total := len(tasks)
	width := len(fmt.Sprintf("%d", total))

	var counter atomic.Int64

	client := &http.Client{Timeout: 12 * time.Second}

	for range lyricJobs {
		wg.Go(func() {
			for task := range taskCh {
				curr := counter.Add(1)

				params := url.Values{}
				params.Add("artist_name", task.artistName)
				params.Add("track_name", task.trackName)
				params.Add("album_name", task.albumName)
				params.Add("duration", task.duration)

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
						logger.Always("[%0*d/%0*d] Network error after retries: %s", width, curr, width, total, task.trackName)
						break
					}

					func ()  {
						defer respGet.Body.Close()

						if respGet.StatusCode == http.StatusTooManyRequests || respGet.StatusCode >= 500 {
							if attempt < maxRetries - 1 {
								time.Sleep(time.Duration(attempt) * 1 * time.Second)
								return
							}
							logger.Always("[%0*d/%0*d] HTTP %d (Rate limited / Server error): %s", width, curr, width, total, respGet.StatusCode, task.trackName)
							return
						}

						if respGet.StatusCode == http.StatusNotFound {
							logger.Always("[%0*d/%0*d] Not found (404): %s \n Trying search", width, curr, width, total, task.trackName)
							stats.notFoundCounter.Add(1)
							notFound = true 
							return
						}

						if respGet.StatusCode != http.StatusOK {
							logger.Always("[%0*d/%0*d] HTTP %d: %s", width, curr, width, total, respGet.StatusCode, task.trackName)
							return
						}

						var data GetResponse
						if err := json.NewDecoder(respGet.Body).Decode(&data); err != nil {
							logger.Always("[%0*d/%0*d] JSON parse error for %s: %v", width, curr, width, total, task.trackName, err)
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
					logger.Verbose("[%0*d/%0*d] Fetched synced lyrics: %s", width, curr, width, total, task.trackName)
					stats.syncedCounter.Add(1)
					cache.addToCache(task.filePath, syncedLyrics)
				} else if plainLyrics != "" {
					logger.Verbose("[%0*d/%0*d] Fetched plain lyrics: %s", width, curr, width, total, task.trackName)
					stats.plainCounter.Add(1)
					cache.addToCache(task.filePath, plainLyrics)
				} else if fetched {
					logger.Always("[%0*d/%0*d] Track found but contains no lyrics: %s", width, curr, width, total, task.trackName)
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

func getTasks(files []string, stats *Statistics, jobs int, noSkip bool, logger *util.Logger) []NecessaryData { 
	var wg sync.WaitGroup
	var mu sync.Mutex

	tasks := make([]NecessaryData, 0, len(files))
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

				if m, ok := getTags(file); ok {
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

func glob(dir string, ext string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ext && !strings.Contains(path, "Body 13") && !strings.Contains(path, "Bull of Heaven") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func GetLyrics(config arguments.Config) Statistics {
	files, err := glob(config.InputPath, ".flac")
	if err != nil {
		panic("FAILED TO GLOB FLAC")
	}

	if config.ParseMP3 {
		mp3, err := glob(config.InputPath, ".mp3")	
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

	tasks := getTasks(files, &stats, jobs, config.NoSkip, &logger)

	lyrics := fetchLyrics(tasks, &stats, fetchJobs, maxRetries, &logger)

	createLyricFiles(&lyrics, jobs, &logger)

	return stats
}
