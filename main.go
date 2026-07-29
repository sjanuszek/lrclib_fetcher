package main

import (
	"fmt"
	"lrclib_fetcher/tui"
	//"lrclib_fetcher/arguments"
	//"lrclib_fetcher/fetcher"
	"time"
)

func timeTrack(start time.Time, name string) {
    elapsed := time.Since(start)
    fmt.Printf("\n%s took %s to complete", name, elapsed)
}

func main() {
	//config := arguments.GetConfig()

	//defer timeTrack(time.Now(), "Fetching")
	//stats := fetcher.GetLyrics(config)

	//fmt.Print(stats.String())
	tui.RunTUI()
}
