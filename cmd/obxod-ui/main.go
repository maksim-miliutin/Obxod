package main

//go:generate rsrc -manifest obxod-ui.manifest -arch amd64 -o rsrc_windows_amd64.syso

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

func main() {
	window := app.New().NewWindow("Obxod")

	on := false
	button := widget.NewButton("Включить", nil)
	button.OnTapped = func() {
		on = !on

		if on {
			button.SetText("Выключить")

			return
		}

		button.SetText("Включить")
	}

	window.SetContent(button)
	window.Resize(fyne.NewSize(360, 420))

	window.ShowAndRun()
}
