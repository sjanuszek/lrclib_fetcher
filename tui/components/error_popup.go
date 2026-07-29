package components

import "github.com/rivo/tview"

func MakeErrorPopup(message string, onPress func()) *tview.Modal {
	var popup *tview.Modal

	popup = tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			onPress()
		})
	
	return popup
}
