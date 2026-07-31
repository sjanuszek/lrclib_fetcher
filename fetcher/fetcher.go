package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lrclib_fetcher/arguments"
	"lrclib_fetcher/metadata"
	"lrclib_fetcher/util"
)

type GetResponse struct {
	PlainLyrics string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

type SearchResponse struct {
	TrackName string `json:"trackName"`
	ArtistName string `json:"artistName"`
	AlbumName string `json:"albumName"`
	Duration float32 `json:"duration"`
	Instrumental bool `json:"instrumental"`
	PlainLyrics string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

type FormattingData struct {
	width, total int
	curr int64
	trackName string
}

type Fetcher struct {
	client http.Client
	stats *Statistics
	lyricJobs int
	maxRetries int
	logger *util.Logger
}

func NewFetcher(stats *Statistics, lyricJobs, maxRetries int, logger *util.Logger) Fetcher {
	return Fetcher{
		http.Client{Timeout: 12 * time.Second},
		stats,
		lyricJobs,
		maxRetries,
		logger,
	}
}

func parseRetryAfter(header string) time.Duration {
	if sec, err := strconv.Atoi(header); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}

	return 2 * time.Second 
}

func doAPIRequest[T any](fetcher *Fetcher, endpoint string, params url.Values, formattingData FormattingData) (T, error) {
	var data T

	requestURL:= APIBase + endpoint + "?" + params.Encode()
	endpointFormatting := strings.ToUpper(endpoint)
	for attempt := range fetcher.maxRetries  {
		fetcher.logger.Debug("%s", requestURL)

		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			return data, err
		}

		req.Header.Set("User-Agent", Header)

		resp, err := fetcher.client.Do(req)
		if err != nil {
			if attempt < fetcher.maxRetries {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			fetcher.logger.Always("[%s][%0*d/%0*d] Network error after retries: %s", endpointFormatting, formattingData.width, formattingData.curr, formattingData.width, formattingData.total, formattingData.trackName)
			return data, err
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return data, err 
		}
		
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			if attempt < fetcher.maxRetries - 1 {
				fetcher.logger.Always("[%s][%0*d/%0*d] HTTP %d (Rate Limited). Retrying in %v: %s", endpointFormatting, formattingData.width, formattingData.curr, formattingData.width, formattingData.total, resp.StatusCode, retryAfter, formattingData.trackName)
				time.Sleep(retryAfter)
				continue
			}
			fetcher.logger.Always("[%s][%0*d/%0*d] HTTP %d Rate limit exceeded: %s", endpointFormatting, formattingData.width, formattingData.curr, formattingData.width, formattingData.total, resp.StatusCode, formattingData.trackName)
			return data, fmt.Errorf("[%s] HTTP %d", endpointFormatting, resp.StatusCode)
		}

		if resp.StatusCode == http.StatusNotFound {
			fetcher.logger.Always("[%s][%0*d/%0*d] Not found (404): %s", endpointFormatting, formattingData.width, formattingData.curr, formattingData.width, formattingData.total, formattingData.trackName)
			fetcher.stats.notFoundCounter.Add(1)
			return data, fmt.Errorf("[%s] NOTHING FOUND 404", endpointFormatting)
		}

		if resp.StatusCode != http.StatusOK {
			fetcher.logger.Always("[%s][%0*d/%0*d] HTTP %d: %s", endpointFormatting, formattingData.width, formattingData.curr, formattingData.width, formattingData.total, resp.StatusCode, formattingData.trackName)
			return data, fmt.Errorf("[%s] HTTP %d", endpointFormatting, resp.StatusCode)
		}

		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			fetcher.logger.Always("[%s][%0*d/%0*d] JSON parse error for %s: %v", endpointFormatting, formattingData.width, formattingData.curr, formattingData.width, formattingData.total, formattingData.trackName, err)
			return data, err
		}

		return data, nil
	}

	return data, fmt.Errorf("NOTHING FOUND WITH %s", endpointFormatting)
}

func (fetcher *Fetcher) tryGet(params url.Values, formattingData FormattingData) (GetResponse, error) {
	return doAPIRequest[GetResponse](fetcher, "get", params, formattingData)
}

func (fetcher *Fetcher) trySearch(params url.Values, formattingData FormattingData) ([]SearchResponse, error) {
	return doAPIRequest[[]SearchResponse](fetcher, "search", params, formattingData)
}

func (fetcher *Fetcher) fetchLyrics(tasks []metadata.NecessaryData) *LyricsCache {
	var wg sync.WaitGroup
	cache := LyricsCache {
		resolved: make([]LyricTuple, 0, len(tasks)),
		unresolved: make([]UnresolvedLyrics, 0, len(tasks)),
	}

	taskCh := make(chan metadata.NecessaryData, len(tasks))

	total := len(tasks)
	width := len(fmt.Sprintf("%d", total))

	var counter atomic.Int64

	for range fetcher.lyricJobs {
		wg.Go(func() {
			for task := range taskCh {
				curr := counter.Add(1)

				time.Sleep(250 * time.Millisecond)

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

				var syncedLyrics, plainLyrics string

				respGet, err := fetcher.tryGet(params, formattingData)
				plainLyrics, syncedLyrics = respGet.PlainLyrics, respGet.SyncedLyrics

				if err != nil || syncedLyrics == "" {
					fetcher.logger.Always("[%0*d/%0*d] Failed to fetch lyrics using get (%s) trying search: %s", width, curr, width, total, err, task.TrackName)
					respSearch, err := fetcher.trySearch(params, formattingData)
					if err != nil {
						fetcher.logger.Always("[%0*d/%0*d] Failed to fetch lyrics using search (%s): %s", width, curr, width, total, err, task.TrackName)
					} else {
						fetcher.logger.Always("[%0*d/%0*d] Fetch lyrics using search (%s): %s", width, curr, width, total, err, task.TrackName)
						cache.addToUnresolved(respSearch, task)
					}

				}
				
				if syncedLyrics != "" {
					fetcher.logger.Verbose("[%0*d/%0*d] Fetched synced lyrics: %s", width, curr, width, total, task.TrackName)
					fetcher.stats.syncedCounter.Add(1)
					cache.addToResolved(task.FilePath, syncedLyrics)
				} else if plainLyrics != "" {
					fetcher.logger.Verbose("[%0*d/%0*d] Fetched plain lyrics: %s", width, curr, width, total, task.TrackName)
					fetcher.stats.plainCounter.Add(1)
					cache.addToResolved(task.FilePath, plainLyrics)
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

	return &cache
}

func (fetcher *Fetcher) CreateLyricFiles(lyrics *LyricsCache, jobs int) {
	var wg sync.WaitGroup
	jobCh := make(chan LyricTuple, len(lyrics.resolved))

	total := len(lyrics.resolved)
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

	for _, tuple := range lyrics.resolved {
		jobCh <- tuple
	}
	close(jobCh)

	wg.Wait()
}

func (fetcher *Fetcher) GetLyrics(config arguments.Config) *LyricsCache {
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

	tasks := GetTasks(files, fetcher.stats, config.Jobs, config.NoSkip, fetcher.logger)

	cache := fetcher.fetchLyrics(tasks)

	fetcher.logger.Always("FINISHED")

	return cache
}
