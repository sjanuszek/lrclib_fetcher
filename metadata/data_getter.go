package metadata

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	"github.com/hajimehoshi/go-mp3"
	"github.com/mewkiz/flac"
)

type NecessaryData struct {
	FilePath, TrackName, ArtistName, AlbumName, Duration string
}

func getTrackDuration(fileName string) (string, error) {
	ext := filepath.Ext(fileName)

	switch ext {
		case ".flac":
			return getFlacDuration(fileName)
		case ".mp3":
			return getMp3Duration(fileName)
		default:
			return getDurationFFprobe(fileName)
	}
}

func getFlacDuration(fileName string) (string, error) {
	stream, err := flac.ParseFile(fileName)
	if err != nil {
		return "", err
	}

	if stream.Info.SampleRate == 0 {
		return "", fmt.Errorf("invalid sample rate")
	}

	duration := stream.Info.NSamples / uint64(stream.Info.SampleRate)
	return fmt.Sprintf("%d", duration), nil
}

func getMp3Duration(fileName string) (string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return "", err
	}

	sampleRate := decoder.SampleRate()
	if sampleRate == 0 {
		return "", fmt.Errorf("invalid sample rate")
	}

	totalBytes := decoder.Length()
	duration := totalBytes / int64(4*sampleRate)

	return fmt.Sprintf("%d", duration), nil
}

func getDurationFFprobe(fileName string) (string, error) {
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

