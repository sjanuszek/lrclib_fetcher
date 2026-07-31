package fetcher

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
