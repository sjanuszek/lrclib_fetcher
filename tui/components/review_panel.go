package components

import (
	"fmt"
	"lrclib_fetcher/fetcher"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func getShortcutRune(index int) rune {
	if index < 9 {
		return rune('1' + index)
	}
	if index < 35 {
		return rune('a' + (index - 9)) 
	}
	return 0
}

func MakeReviewPanelFlex(responses []fetcher.SearchResponse, curr, total int, onSelect func(lyrics string)) *tview.Flex {
	var list *tview.List
	var text *tview.TextView
	var flex *tview.Flex

	synced := true

	flex = tview.NewFlex()

	width := len(fmt.Sprintf("%d", total))
	flex.SetBorder(true).
		SetTitle(fmt.Sprintf("To review: %0*d/%0*d", width, curr, width, total))

	list = tview.NewList()
	for i, resp := range responses {
		hasSynced := responses[i].SyncedLyrics != ""
		hasPlain := responses[i].PlainLyrics != ""

		var versionInfo string
		switch {
		case hasSynced && hasPlain:
			versionInfo = fmt.Sprintf(" [#7ee787]%s[-]", tview.Escape("[S/P]"))
		case hasSynced:
			versionInfo = fmt.Sprintf(" [#7ee787]%s[-]", tview.Escape("[S]"))
		case hasPlain:
			versionInfo = fmt.Sprintf(" [#e3b341]%s[-]", tview.Escape("[P]"))
		default:
			versionInfo = fmt.Sprintf(" [#f85149]%s[-]", tview.Escape("[No Lyrics]"))
		}

		shortcut := getShortcutRune(i)
		
		header := fmt.Sprintf("%s" + versionInfo, resp.TrackName)
		description := fmt.Sprintf("%s - %s", resp.AlbumName, resp.ArtistName)

		list.AddItem(header, description, shortcut, nil)
	}

	updateText := func() {
		index := list.GetCurrentItem()
		if index < 0 || index >= len(responses) {
			return
		}

		if synced {
			lyrics := responses[index].SyncedLyrics
			text.SetTitle("SYNCED LYRICS")
			if lyrics == "" {
				text.SetText("No synced lyrics")
			} else {
				text.SetText(lyrics)
			}
		} else {
			lyrics := responses[index].PlainLyrics
			text.SetTitle("PLAIN LYRICS")
			if lyrics == "" {
				text.SetText("No plain lyrics")
			} else {
				text.SetText(lyrics)
			}
		}
	}

	list.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		updateText()
	})

	list.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		index := list.GetCurrentItem()
		if index < 0 || index >= len(responses) {
			return
		}

		if synced {
			onSelect(responses[index].SyncedLyrics)
		} else {
			onSelect(responses[index].PlainLyrics)
		}
	})

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			synced = !synced
			updateText()
			return nil
		}
		return event
	})

	text = tview.NewTextView()
	text.SetBorder(true)

	updateText()

	flex.AddItem(list, 0, 1, true).
		AddItem(text, 0, 2, false)

	return flex
}
