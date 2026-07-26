package fetcher

import (
	"encoding/json"
	"errors"
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

type FormattingData struct {
	width, total int
	curr int64
	trackName string
}

type Fetcher struct {
	client http.Client
	stats Statistics
	lyricJobs int
	maxRetries int
	logger util.Logger
}

func (fetcher *Fetcher) tryGet(params url.Values, formattingData FormattingData) (GetResponse, error) {
	var data GetResponse
	var fetched, notFound bool

	for attempt := range fetcher.maxRetries - 1 {
		requestGet := APIBase + "get?" + params.Encode()
		fetcher.logger.Debug("%s", requestGet)

		req, err := http.NewRequest("GET", requestGet, nil)
		if err != nil {
			fmt.Println(err)
			return GetResponse{}, err
		}

		req.Header.Set("User-Agent", "MyLrcFetcher/1.0")

		respGet, err := fetcher.client.Do(req)
		if err != nil {
			if attempt < fetcher.maxRetries {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			fetcher.logger.Always("[%0*d/%0*d] Network error after retries: %s", formattingData.width, formattingData.curr, formattingData.width, formattingData.total, formattingData.trackName)
			return GetResponse{}, err
		}
		
		func ()  {
			defer respGet.Body.Close()

			if respGet.StatusCode == http.StatusTooManyRequests || respGet.StatusCode >= 500 {
				if attempt < fetcher.maxRetries - 1 {
					time.Sleep(time.Duration(attempt) * 1 * time.Second)
					return
				}
				fetcher.logger.Always("[%0*d/%0*d] HTTP %d (Rate limited / Server error): %s", formattingData.width, formattingData.curr, formattingData.width, formattingData.total, respGet.StatusCode, formattingData.trackName)
				return
			}

			if respGet.StatusCode == http.StatusNotFound {
				fetcher.logger.Always("[%0*d/%0*d] Not found (404): %s", formattingData.width, formattingData.curr, formattingData.width, formattingData.total, formattingData.trackName)
				fetcher.stats.notFoundCounter.Add(1)
				notFound = true
				return
			}

			if respGet.StatusCode != http.StatusOK {
				fetcher.logger.Always("[%0*d/%0*d] HTTP %d: %s", formattingData.width, formattingData.curr, formattingData.width, formattingData.total, respGet.StatusCode, formattingData.trackName)
				return
			}

			if err := json.NewDecoder(respGet.Body).Decode(&data); err != nil {
				fetcher.logger.Always("[%0*d/%0*d] JSON parse error for %s: %v", formattingData.width, formattingData.curr, formattingData.width, formattingData.total, formattingData.trackName, err)
				return
			}

			fetched = true

		}()

		if fetched {
			return data, nil
		} else if notFound {
			return GetResponse{}, errors.New("NOTHING FOUND 404")
		}
	}

	return GetResponse{}, errors.New("NOTHING FOUND WITH GET")
}

func (fetcher *Fetcher) fetchLyrics(tasks []metadata.NecessaryData) LyricsCache {
	var wg sync.WaitGroup
	cache := LyricsCache {
		cache: make([]LyricTuple, 0, len(tasks)),
	}
	taskCh := make(chan metadata.NecessaryData, len(tasks))

	total := len(tasks)
	width := len(fmt.Sprintf("%d", total))

	var counter atomic.Int64

	for range fetcher.lyricJobs {
		wg.Go(func() {
			for task := range taskCh {
				curr := counter.Add(1)

				params := url.Values{}
				params.Add("artist_name", task.ArtistName)
				params.Add("track_name", task.TrackName)
				params.Add("album_name", task.AlbumName)
				params.Add("duration", task.Duration)

				formattingData := FormattingData{
					width,
					total,
					curr,
					task.TrackName,
				}

				respGet, err := fetcher.tryGet(params, formattingData)

				if err != nil {
					fetcher.logger.Always("[%0*d/%0*d] Failed to fetch lyrics using get: %s", width, curr, width, total, task.TrackName)
				}
				
				if respGet.SyncedLyrics != "" {
					fetcher.logger.Verbose("[%0*d/%0*d] Fetched synced lyrics: %s", width, curr, width, total, task.TrackName)
					fetcher.stats.syncedCounter.Add(1)
					cache.addToCache(task.FilePath, respGet.SyncedLyrics)
				} else if respGet.PlainLyrics != "" {
					fetcher.logger.Verbose("[%0*d/%0*d] Fetched plain lyrics: %s", width, curr, width, total, task.TrackName)
					fetcher.stats.plainCounter.Add(1)
					cache.addToCache(task.FilePath, respGet.PlainLyrics)
				} else {
					fetcher.logger.Always("[%0*d/%0*d] Track found but contains no lyrics: %s", width, curr, width, total, task.TrackName)
					fetcher.stats.failedCounter.Add(1)
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

func (fetcher *Fetcher) createLyricFiles(lyrics *LyricsCache, jobs int) {
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

				fetcher.logger.Verbose("[%0*d/%0*d] Writing: %s", width, curr, width, total, new_path)

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
	client := &http.Client{Timeout: 12 * time.Second}
	jobs := min(max(config.Jobs, 0), runtime.NumCPU())
	fetchJobs := max(config.FetchJobs, 0)
	maxRetries := max(config.MaxRetries, 0)
	logger := util.Logger {
		IsVerbose: config.Verbose,
		IsDebug: config.Debug,
	}

	fetcher := Fetcher{
		*client,
		stats,
		fetchJobs,
		maxRetries,
		logger,
	}

	tasks := GetTasks(files, &stats, jobs, config.NoSkip, &logger)

	lyrics := fetcher.fetchLyrics(tasks)

	fetcher.createLyricFiles(&lyrics, jobs)

	return stats
}
