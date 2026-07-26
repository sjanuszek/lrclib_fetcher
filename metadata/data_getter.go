package metadata

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"github.com/dhowden/tag"
)

type NecessaryData struct {
	FilePath, TrackName, ArtistName, AlbumName, Duration string
}

func getTrackDuration(fileName string) (string, error) {
	cmd := exec.Command("ffprobe", "-i", fileName, "-show_entries", "format=duration", "-v", "quiet", "-of", "csv=p=0",)
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()

	val, _, _ := strings.Cut(strings.TrimSpace(out.String()), ".")

	return val, nil
}

func GetTags(file *os.File) (NecessaryData, bool) {
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

