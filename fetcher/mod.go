package fetcher

import (
	"fmt"
	"sync/atomic"
)

const APIBase = "https://lrclib.net/api/"
const Header = "lrclib_fetcher/0.1 (https://github.com/sjanuszek/lrclib_fetcher)"

type Statistics struct {
	filesCounter, skippedCounter, processedCounter, notFoundCounter, syncedCounter, plainCounter, failedCounter atomic.Int64
}

func (s *Statistics) String() string {
	return fmt.Sprintf(
		"Files Found: %d\n"+
		"Skipped (.lrc exists): %d\n"+
		"Metadata Processed: %d\n"+
		"Synced Lyrics Fetched: %d\n"+
		"Plain Lyrics Fetched: %d\n"+
		"Not Found (404): %d\n"+
		"Failed / No Lyrics: %d\n",
		s.filesCounter.Load(),
		s.skippedCounter.Load(),
		s.processedCounter.Load(),
		s.syncedCounter.Load(),
		s.plainCounter.Load(),
		s.notFoundCounter.Load(),
		s.failedCounter.Load(),
	)
}

