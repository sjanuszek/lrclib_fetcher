package components

import "github.com/rivo/tview"

func MakeProcessingTextViews() (*tview.TextView, *tview.TextView) {
	var statsTextView, loggerTextView *tview.TextView

	statsTextView = tview.NewTextView()
	statsTextView.SetBorder(true).SetTitle("Statistics")

	loggerTextView = tview.NewTextView().
		SetScrollable(true).
		SetWrap(true)

	loggerTextView.SetBorder(true).SetTitle("Logger output")

	return statsTextView, loggerTextView 
}
