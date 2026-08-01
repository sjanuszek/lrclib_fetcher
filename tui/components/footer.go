package components

import "github.com/rivo/tview"

func MakeFooterView(text string) *tview.TextView {
	var footer *tview.TextView

	footer = tview.NewTextView()
	footer.SetText(text)
	
	return footer
}
