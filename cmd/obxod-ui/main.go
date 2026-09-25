package main

//go:generate rsrc -manifest obxod-ui.manifest -arch amd64 -o rsrc_windows_amd64.syso

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"obxod/internal/meter"
	"obxod/internal/preset"
	"obxod/internal/runner"
)

var (
	working = color.NRGBA{R: 0x2f, G: 0x9e, B: 0x44, A: 0xff}
	idle    = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff}
)

func main() {
	window := app.New().NewWindow("Obxod")

	hosts := preset.Hosts()

	dot := canvas.NewText("●", idle)
	word := widget.NewLabel("Выключено")
	count := widget.NewLabel(fmt.Sprintf("Обходится сайтов: %d", len(hosts)))
	list := widget.NewLabel(strings.Join(hosts, "\n"))
	speed := widget.NewLabel("↓ —")

	chosen := preset.All()[0]

	var session *runner.Session
	var rate meter.Rate

	button := widget.NewButton("Включить", nil)

	turnOff := func() {
		session.Stop()
		session = nil

		dot.Color = idle
		dot.Refresh()
		word.SetText("Выключено")
		button.SetText("Включить")
	}

	turnOn := func() {
		started, err := start(chosen)
		if err != nil {
			dialog.ShowError(err, window)

			return
		}

		session = started
		rate = meter.Rate{}
		go session.Run()

		dot.Color = working
		dot.Refresh()
		word.SetText("Работает")
		button.SetText("Выключить")
	}

	button.OnTapped = func() {
		if session != nil {
			turnOff()

			return
		}

		turnOn()
	}

	choose := widget.NewSelect(preset.Names(), func(name string) {
		found, ok := preset.Named(name)
		if !ok {
			return
		}

		chosen = found

		if session != nil {
			turnOff()
			turnOn()
		}
	})
	choose.SetSelected(chosen.Name)

	go func() {
		for range time.Tick(time.Second) {
			fyne.Do(func() {
				if session == nil {
					speed.SetText("↓ —")

					return
				}

				speed.SetText("↓ " + meter.Human(rate.Sample(session.Downloaded(), time.Now())))
			})
		}
	}()

	top := container.NewVBox(
		container.NewHBox(dot, word),
		button,
		widget.NewLabel("Способ:"),
		choose,
		count,
	)
	window.SetContent(container.NewBorder(top, speed, nil, nil, container.NewVScroll(list)))
	window.Resize(fyne.NewSize(360, 420))

	window.ShowAndRun()
}

func start(p preset.Preset) (*runner.Session, error) {
	set, err := p.Rules()
	if err != nil {
		return nil, err
	}

	return runner.Open(runner.Config{
		Rules:    set,
		Ports:    runner.DefaultPorts(),
		Wet:      true,
		DropQUIC: true,
		Report:   func(string) {},
	})
}
