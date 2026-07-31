package tui

import (
	"lrclib_fetcher/arguments"
	"lrclib_fetcher/fetcher"
	"lrclib_fetcher/tui/components"
	"lrclib_fetcher/util"
	"time"

	"github.com/rivo/tview"
)

var app *tview.Application

func RunTUI() {
	app = tview.NewApplication()

	pages := tview.NewPages()

	var form *tview.Form
	var tree *tview.TreeView
	var status *tview.TextView
	var statsTextView, loggerTextView  *tview.TextView
	var processingFlex, mainFlex *tview.Flex

	status = components.MakeStatusView()

	components.StartHealthCheck(func(online bool) {
		app.QueueUpdateDraw(func() {
			components.UpdateStatusView(status, online)
		})
	})

	form = components.MakeMainForm(func() {
		pages.SwitchToPage("browser")
		app.SetFocus(tree)
	}, func(cfg arguments.Config) {
		pages.SwitchToPage("processing")
		app.SetFocus(processingFlex)
		
		go func() {
			var stats fetcher.Statistics

			ticker := time.NewTicker(100 * time.Millisecond)
			done := make(chan struct{})

			go func()  {
				for {
					select {
					case <- ticker.C:
						app.QueueUpdateDraw(func() {
							statsTextView.SetText(stats.String())
						})
					case <- done:
						ticker.Stop()
						app.QueueUpdateDraw(func() {
							statsTextView.SetText(stats.String())
						})
						return
					}
				}
			}()

			logger := util.Logger{
				Output: loggerTextView,
				IsVerbose: cfg.Verbose,
				IsDebug: cfg.Debug,
			}
			fetcher := fetcher.NewFetcher(&stats, cfg.FetchJobs, cfg.MaxRetries, &logger)
			cache := fetcher.GetLyrics(cfg)
			fetcher.CreateLyricFiles(cache, cfg.Jobs)

			close(done)
		}()
	}, func(s string) {
		error_popup := components.MakeErrorPopup(s, func() {
			pages.RemovePage("error_popup")
			app.SetFocus(form)
		})

		pages.AddPage("error_popup", error_popup, false, true)
		app.SetFocus(error_popup)
	})

	tree = components.MakeTreeBrowser(func(path string) {
		if input, ok := form.GetFormItemByLabel("Input path").(*tview.InputField); ok {
			input.SetText(path)
		}
		pages.SwitchToPage("main")
		app.SetFocus(form)
	}, func() {
		pages.SwitchToPage("main")
		app.SetFocus(form)
	})

	statsTextView, loggerTextView = components.MakeProcessingTextViews()

	loggerTextView.SetChangedFunc(func() {
		loggerTextView.ScrollToEnd()
		app.Draw()
	})

	headerFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(status, 25, 0, false)

	processingFlex = tview.NewFlex().
		AddItem(statsTextView, 0, 1, true).
		AddItem(loggerTextView, 0, 3, true)

	mainFlex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(headerFlex, 1, 0, false).
		AddItem(form, 0, 1, true)

	pages.
		AddPage("main", mainFlex, true, true).
		AddPage("browser", tree, true, false).
		AddPage("processing", processingFlex, true, false)

	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
