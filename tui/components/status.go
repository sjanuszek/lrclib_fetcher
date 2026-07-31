package components

import (
	"lrclib_fetcher/fetcher"
	"net/http"
	"time"

	"github.com/rivo/tview"
)

func MakeStatusView() *tview.TextView {
	status := tview.NewTextView()
	status.SetDynamicColors(true)

	status.SetText("[#e3b341]Checking API...[-]")

	return status
}

func UpdateStatusView(view *tview.TextView, online bool) {
	if online {
		view.SetText("[#7ee787]LRCLIB: Online[-]")
	} else {
		view.SetText("[#f85149]LRCLIB: Offline[-]")
	}
}

func StartHealthCheck(onStatusChange func(online bool)) {
	go func() {
		onStatusChange(pingAPI())

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			onStatusChange(pingAPI())
		}
	}()
}

func pingAPI() bool {
	client := http.Client{Timeout: 3 * time.Second}

	req, err := http.NewRequest("GET", fetcher.APIBase + "search?q=ping", nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", fetcher.Header)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}
