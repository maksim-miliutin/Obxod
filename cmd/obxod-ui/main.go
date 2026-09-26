package main

//go:generate rsrc -manifest obxod-ui.manifest -arch amd64 -o rsrc_windows_amd64.syso

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"obxod/internal/logbook"
	"obxod/internal/meter"
	"obxod/internal/preset"
	"obxod/internal/runner"
	"obxod/internal/sites"
)

var (
	working = color.NRGBA{R: 0x2f, G: 0x9e, B: 0x44, A: 0xff}
	idle    = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff}
)

func main() {
	window := app.New().NewWindow("Obxod")

	driverErr := unpackDriver()

	own, _ := sites.Load(sitesFile())
	book := logbook.New(200)

	dot := canvas.NewText("●", idle)
	word := widget.NewLabel("Выключено")
	count := widget.NewLabel("")
	speed := widget.NewLabel("↓ —")
	logView := widget.NewLabel("")
	always := widget.NewLabel(strings.Join(preset.Hosts(), "\n"))
	ownRows := container.NewVBox()

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
		started, err := start(chosen, own.List(), book.Add)
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

	restart := func() {
		if session != nil {
			turnOff()
			turnOn()
		}
	}

	save := func() {
		if err := own.Save(sitesFile()); err != nil {
			dialog.ShowError(err, window)
		}
	}

	var show func()
	show = func() {
		count.SetText(fmt.Sprintf("Обходится сайтов: %d", len(preset.HostsWith(own.List()))))

		ownRows.RemoveAll()

		for _, name := range own.List() {
			row := container.NewHBox(
				widget.NewButton("×", func() {
					own.Remove(name)
					save()
					show()
					restart()
				}),
				widget.NewLabel(name),
			)
			ownRows.Add(row)
		}

		ownRows.Refresh()
	}
	show()

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
		restart()
	})
	choose.SetSelected(chosen.Name)

	caveat := widget.NewLabel("Не подошёл — попробуйте другой.\nУ разных провайдеров работают разные.")

	entry := widget.NewEntry()
	entry.SetPlaceHolder("instagram.com")

	add := widget.NewButton("Добавить сайт", func() {
		if strings.TrimSpace(entry.Text) == "" {
			return
		}

		own.Add(entry.Text)
		save()
		entry.SetText("")
		show()
		restart()
	})

	go func() {
		for range time.Tick(time.Second) {
			fyne.Do(func() {
				logView.SetText(book.Text())

				if session == nil {
					speed.SetText("↓ —")

					return
				}

				speed.SetText("↓ " + meter.Human(rate.Sample(session.Downloaded(), time.Now())))
			})
		}
	}()

	obhod := container.NewVBox(
		container.NewHBox(dot, word),
		button,
		widget.NewLabel("Способ:"),
		choose,
		caveat,
		speed,
	)

	yourSites := container.NewBorder(
		container.NewVBox(
			container.NewBorder(nil, nil, nil, add, entry),
			count,
			widget.NewLabel("Ваши сайты:"),
		),
		nil, nil, nil,
		container.NewVScroll(container.NewVBox(
			ownRows,
			widget.NewSeparator(),
			widget.NewLabel("Всегда обходятся:"),
			always,
		)),
	)

	about := container.NewVBox(
		widget.NewLabel("Obxod — обход DPI-блокировок."),
		widget.NewLabel("Открывает то, что режут по имени хоста:\nDiscord, YouTube, X и добавленные вами."),
		widget.NewLabel("Не ускоряет то, что не блокируют."),
		widget.NewSeparator(),
		widget.NewLabel("При запуске Windows может сказать\n«неизвестный издатель» — это нормально,\nподписи пока нет: Подробнее → Всё равно запустить."),
	)

	logsTab := container.NewBorder(
		widget.NewButton("Скопировать", func() {
			window.Clipboard().SetContent(book.Text())
		}),
		nil, nil, nil,
		container.NewVScroll(logView),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Обход", obhod),
		container.NewTabItem("Сайты", yourSites),
		container.NewTabItem("Логи", logsTab),
		container.NewTabItem("О программе", about),
	)
	window.SetContent(tabs)
	window.Resize(fyne.NewSize(440, 600))
	window.SetFixedSize(true)

	if driverErr != nil {
		dialog.ShowError(driverErr, window)
	}

	window.ShowAndRun()
}

func sitesFile() string {
	exe, err := os.Executable()
	if err != nil {
		return "sites.txt"
	}

	return filepath.Join(filepath.Dir(exe), "sites.txt")
}

func start(p preset.Preset, extra []string, report func(string)) (*runner.Session, error) {
	set, err := p.RulesVoice(extra)
	if err != nil {
		return nil, err
	}

	return runner.Open(runner.Config{
		Rules:    set,
		Ports:    runner.DefaultPorts(),
		Voice:    runner.DefaultVoice(),
		Voiced:   voiceDatagram,
		Wet:      true,
		DropQUIC: true,
		Report:   report,
	})
}
