package metadata

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"

	"go.senan.xyz/taglib"
)

type NecessaryData struct {
	FilePath, TrackName, ArtistName, AlbumName, Duration string
}

func getTrackDuration(fileName string) (string, error) {
	properties, err := taglib.ReadProperties(fileName);

	switch err {
		case nil:
			return strconv.Itoa(int(properties.Length.Seconds())), nil
		default:
			return getDurationFFprobe(fileName)
	}
}

func getDurationFFprobe(fileName string) (string, error) {
	cmd := exec.Command("ffprobe", "-i", fileName, "-show_entries", "format=duration", "-v", "quiet", "-of", "csv=p=0",)
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()

	val, _, _ := strings.Cut(strings.TrimSpace(out.String()), ".")

	return val, nil
}

func GetTags(fileName string) (NecessaryData, bool) {
	tags, err := taglib.ReadTags(fileName)
	if err != nil {
		return NecessaryData{}, false
	}

	duration, err := getTrackDuration(fileName)
	if err != nil {
		return NecessaryData{}, false
	}

	return NecessaryData{
		fileName,
		tags[taglib.Title][0],
		tags[taglib.AlbumArtist][0],
		tags[taglib.Album][0],
		duration,
	}, true
}

