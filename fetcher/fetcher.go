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

type Statistics struct {
	filesCounter, skippedCounter, processedCounter, notFoundCounter, syncedCounter, plainCounter, failedCounter atomic.Int64
}

type Logger struct {
	mu sync.Mutex
	isVerbose bool
	isDebug bool
}

type GetResponse struct {
	PlainLyrics string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

func (l *Logger) verbose(format string, a ...any) {
	if !l.isVerbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf(format + "\n", a...)
}

func (l *Logger) debug(format string, a ...any) {
	if !l.isDebug {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf("[DEBUG] " + format + "\n", a...)
}

func (l *Logger) always(format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf(format + "\n", a...)
}

func (s *Statistics) String() string {
	return fmt.Sprintf(
		"\n================ Statistics ================\n"+
		"Files Found: %d\n"+
		"Skipped (.lrc exists): %d\n"+
		"Metadata Processed: %d\n"+
		"Synced Lyrics Fetched: %d\n"+
		"Plain Lyrics Fetched: %d\n"+
		"Not Found (404): %d\n"+
		"Failed / No Lyrics: %d\n"+
		"============================================",
		s.filesCounter.Load(),
		s.skippedCounter.Load(),
		s.processedCounter.Load(),
		s.syncedCounter.Load(),
		s.plainCounter.Load(),
		s.notFoundCounter.Load(),
		s.failedCounter.Load(),
	)
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

func fetchLyrics(tasks []NecessaryData, stats *Statistics, lyricJobs int, maxRetries int, logger *Logger) LyricsCache {
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

				request := "https://lrclib.net/api/get?" + params.Encode()
				logger.debug("%s", request)
				var syncedLyrics, plainLyrics string
				var fetched, notFound bool

				for attempt := range maxRetries - 1 {
					req, err := http.NewRequest("GET", request, nil)
					if err != nil {
						fmt.Println(err)
						continue
					}

					req.Header.Set("User-Agent", "MyLrcFetcher/1.0")

					resp, err := client.Do(req)
					if err != nil {
						if attempt < maxRetries {
							time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
							continue
						}
						logger.always("[%0*d/%0*d] Network error after retries: %s", width, curr, width, total, task.trackName)
						break
					}

					func ()  {
						defer resp.Body.Close()

						if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
							if attempt < maxRetries - 1 {
								time.Sleep(time.Duration(attempt) * 1 * time.Second)
								return
							}
							logger.always("[%0*d/%0*d] HTTP %d (Rate limited / Server error): %s", width, curr, width, total, resp.StatusCode, task.trackName)
							return
						}

						if resp.StatusCode == http.StatusNotFound {
							logger.always("[%0*d/%0*d] Not found (404): %s", width, curr, width, total, task.trackName)
							stats.notFoundCounter.Add(1)
							notFound = true 
							return
						}

						if resp.StatusCode != http.StatusOK {
							logger.always("[%0*d/%0*d] HTTP %d: %s", width, curr, width, total, resp.StatusCode, task.trackName)
							return
						}

						var data GetResponse
						if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
							logger.always("[%0*d/%0*d] JSON parse error for %s: %v", width, curr, width, total, task.trackName, err)
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
					logger.verbose("[%0*d/%0*d] Fetched synced lyrics: %s", width, curr, width, total, task.trackName)
					stats.syncedCounter.Add(1)
					cache.addToCache(task.filePath, syncedLyrics)
				} else if plainLyrics != "" {
					logger.verbose("[%0*d/%0*d] Fetched plain lyrics: %s", width, curr, width, total, task.trackName)
					stats.plainCounter.Add(1)
					cache.addToCache(task.filePath, plainLyrics)
				} else if fetched {
					logger.always("[%0*d/%0*d] Track found but contains no lyrics: %s", width, curr, width, total, task.trackName)
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

func getTasks(files []string, stats *Statistics, jobs int, logger *Logger) []NecessaryData { 
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

				if hasLrcFile(file_name) {
					logger.verbose("[%0*d/%0*d] Skipping: %s", width, curr, width, total, file_name)
					stats.skippedCounter.Add(1)
					continue
				}

				file, err := os.Open(file_name)
				if err != nil {
					logger.always("[%0*d/%0*d] Failed to open: %s", width, curr, width, total, file_name)
					continue
				}

				if m, ok := getTags(file); ok {
					logger.verbose("[%0*d/%0*d] Getting metadata: %s", width, curr, width, total, file_name)
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

func createLyricFiles(lyrics *LyricsCache, jobs int, logger *Logger) {
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

				logger.verbose("[%0*d/%0*d] Writing: %s", width, curr, width, total, new_path)

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
	logger := Logger {
		isVerbose: config.Verbose,
		isDebug: config.Debug,
	}

	tasks := getTasks(files, &stats, jobs, &logger)

	lyrics := fetchLyrics(tasks, &stats, fetchJobs, maxRetries, &logger)

	createLyricFiles(&lyrics, jobs, &logger)

	return stats
}
