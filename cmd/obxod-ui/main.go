package main

//go:generate rsrc -manifest obxod-ui.manifest -arch amd64 -o rsrc_windows_amd64.syso

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

func main() {
	window := app.New().NewWindow("Obxod")

	window.SetContent(widget.NewLabel("Obxod"))
	window.Resize(fyne.NewSize(360, 420))

	window.ShowAndRun()
}
