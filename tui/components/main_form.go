package components

import (
	"lrclib_fetcher/arguments"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func verifyMainForm() error {
	return nil
}

func MakeMainForm(onBrowse func(), onRun func(arguments.Config), onError func(string)) *tview.Form {
	var form *tview.Form

	pathInput := tview.NewInputField().
		SetLabel("Input path").
		SetFieldWidth(30).
		SetPlaceholder("Press <Enter> to browse...")
	
	debugCheck := tview.NewCheckbox().
		SetLabel("Enable debug output")

	mp3Check := tview.NewCheckbox().
		SetLabel("Include mp3 in file search")

	noSkipCheck := tview.NewCheckbox().
		SetLabel("Don't skip files with existing .lrc files")

	verboseCheck := tview.NewCheckbox().
		SetLabel("Enable verbose output").
		SetChecked(true)
	
	fetchJobsInput := tview.NewInputField().
		SetLabel("Fetch jobs").
		SetFieldWidth(5).
		SetAcceptanceFunc(tview.InputFieldInteger).
		SetText("4")

	jobsInput := tview.NewInputField().
		SetLabel("Jobs").
		SetFieldWidth(5).
		SetAcceptanceFunc(tview.InputFieldInteger).
		SetText("1")

	maxRetriesInput := tview.NewInputField().
		SetLabel("Max retries").
		SetFieldWidth(5).
		SetAcceptanceFunc(tview.InputFieldInteger).
		SetText("3")
	
	pathInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			onBrowse()
			return nil
		}
		return event
	})

	form = tview.NewForm().
		AddFormItem(pathInput).
		AddFormItem(fetchJobsInput).
		AddFormItem(jobsInput).
		AddFormItem(maxRetriesInput).
		AddFormItem(mp3Check).
		AddFormItem(noSkipCheck).
		AddFormItem(verboseCheck).
		AddFormItem(debugCheck).
		AddButton("Run", func() {
			parseInt := func (s string) int {
				val, _ := strconv.Atoi(s)
				return val
			}

			cfg := arguments.Config{
				InputPath: pathInput.GetText(),
				Jobs: parseInt(jobsInput.GetText()),
				FetchJobs: parseInt(fetchJobsInput.GetText()),
				MaxRetries: parseInt(maxRetriesInput.GetText()),
				ParseMP3: mp3Check.IsChecked(),
				Verbose: verboseCheck.IsChecked(),
				Debug: debugCheck.IsChecked(),
				NoSkip: noSkipCheck.IsChecked(),
			}

			if err := cfg.VerifyConfig(); err != nil {
				onError(err.Error())
				return
			}

			onRun(cfg)
		})

	form.SetBorder(true).
		SetTitle("Config").
		SetTitleAlign(tview.AlignLeft)
	
	form.SetFieldBackgroundColor(tcell.ColorWhite).
		SetFieldTextColor(tcell.ColorBlack).
		SetBackgroundColor(tcell.ColorBlack)

	return form
}
