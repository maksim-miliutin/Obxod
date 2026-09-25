package main

//go:generate rsrc -manifest obxod-ui.manifest -arch amd64 -o rsrc_windows_amd64.syso

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"obxod/internal/preset"
	"obxod/internal/runner"
)

func main() {
	window := app.New().NewWindow("Obxod")

	var session *runner.Session
	button := widget.NewButton("Включить", nil)
	button.OnTapped = func() {
		if session != nil {
			session.Stop()
			session = nil
			button.SetText("Включить")

			return
		}

		started, err := start()
		if err != nil {
			dialog.ShowError(err, window)

			return
		}

		session = started
		go session.Run()
		button.SetText("Выключить")
	}

	window.SetContent(button)
	window.Resize(fyne.NewSize(360, 420))

	window.ShowAndRun()
}

func start() (*runner.Session, error) {
	set, err := preset.All()[0].Rules()
	if err != nil {
		return nil, err
	}

	return runner.Open(runner.Config{
		Rules:    set,
		Wet:      true,
		DropQUIC: true,
		Report:   func(string) {},
	})
}
