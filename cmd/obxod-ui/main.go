package main

//go:generate rsrc -manifest obxod-ui.manifest -ico icon.ico -arch amd64 -o rsrc_windows_amd64.syso

import (
	"errors"
	"fmt"
	"image/color"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"obxod/internal/autopick"
	"obxod/internal/lang"
	"obxod/internal/logbook"
	"obxod/internal/meter"
	"obxod/internal/preset"
	"obxod/internal/probe"
	"obxod/internal/runner"
	"obxod/internal/sites"
)

var (
	working = color.NRGBA{R: 0x2f, G: 0x9e, B: 0x44, A: 0xff}
	idle    = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff}
)

var errNoMethod = errors.New("obxod-ui: method not found")

func starredKey(list []string, key string) bool {
	for _, k := range list {
		if k == key {
			return true
		}
	}

	return false
}

func flip(list []string, key string) []string {
	next := make([]string, 0, len(list)+1)
	for _, k := range list {
		if k != key {
			next = append(next, k)
		}
	}

	if !starredKey(list, key) {
		next = append(next, key)
	}

	return next
}

func main() {
	a := app.NewWithID("io.obxod.ui")
	a.Settings().SetTheme(obxodTheme{})
	window := a.NewWindow("Obxod")

	driverErr := unpackDriver()

	icon := fyne.NewStaticResource("icon.png", iconPNG)
	a.SetIcon(icon)
	window.SetIcon(icon)

	code := a.Preferences().String("lang")
	l := lang.Of(code)

	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayIcon(icon)
		desk.SetSystemTrayMenu(fyne.NewMenu("Obxod",
			fyne.NewMenuItem(l.T(lang.TrayShow), window.Show),
			fyne.NewMenuItem(l.T(lang.TrayQuit), a.Quit),
		))
	}

	// The cross closes to the tray instead of quitting, so the bypass keeps running.
	window.SetCloseIntercept(window.Hide)

	own, _ := sites.Load(sitesFile())
	book := logbook.New(200)
	keep := func(line string) {
		if logbook.Worth(line) {
			book.Add(line)
		}
	}

	dot := canvas.NewText("●", idle)
	word := widget.NewLabel(l.T(lang.StatusOff))
	count := widget.NewLabel("")
	speed := widget.NewLabel(fmt.Sprintf(l.T(lang.SpeedFmt), "—"))
	logView := widget.NewLabel("")
	logHead := widget.NewLabel("")
	progress := widget.NewLabel("")
	now := widget.NewLabel("")
	always := widget.NewLabel(strings.Join(preset.Hosts(), "\n"))
	ownRows := container.NewVBox()

	chosen := preset.All()[0]
	if saved, ok := preset.Named(a.Preferences().String("method")); ok {
		chosen = saved
	}

	favourite := []string{}
	if saved := a.Preferences().String("favourites"); saved != "" {
		favourite = strings.Split(saved, ",")
	}

	var session *runner.Session
	var rate meter.Rate
	searching := false
	var stopFlag atomic.Bool

	button := widget.NewButton(l.T(lang.TurnOn), nil)

	turnOff := func() {
		session.Stop()
		session = nil

		dot.Color = idle
		dot.Refresh()
		word.SetText(l.T(lang.StatusOff))
		button.SetText(l.T(lang.TurnOn))
	}

	turnOn := func() {
		started, err := start(chosen, own.List(), keep)
		if err != nil {
			dialog.ShowError(err, window)

			return
		}

		session = started
		rate = meter.Rate{}
		go session.Run()

		dot.Color = working
		dot.Refresh()
		word.SetText(l.T(lang.StatusOn))
		button.SetText(l.T(lang.TurnOff))
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
		count.SetText(fmt.Sprintf(l.T(lang.CountFmt), len(preset.HostsWith(own.List()))))

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

	choose := widget.NewSelect(preset.NamesByFavourite(code, favourite), nil)
	fav := widget.NewButton("", nil)

	refreshFav := func() {
		if starredKey(favourite, chosen.Key) {
			fav.SetText(l.T(lang.DropFav))

			return
		}

		fav.SetText(l.T(lang.AddFav))
	}

	choose.OnChanged = func(label string) {
		found, ok := preset.ByLabel(label, code)
		if !ok || found.Key == chosen.Key {
			return
		}

		chosen = found
		a.Preferences().SetString("method", found.Key)
		restart()
		refreshFav()
	}

	fav.OnTapped = func() {
		favourite = flip(favourite, chosen.Key)
		a.Preferences().SetString("favourites", strings.Join(favourite, ","))
		choose.Options = preset.NamesByFavourite(code, favourite)
		choose.SetSelected(preset.Label(chosen, code, favourite))
		refreshFav()
	}

	choose.SetSelected(preset.Label(chosen, code, favourite))
	refreshFav()

	pick := widget.NewButton(l.T(lang.AutoPick), nil)
	stopBtn := widget.NewButton(l.T(lang.Stop), nil)
	stopBtn.Disable()
	stopBtn.OnTapped = func() {
		stopFlag.Store(true)
		stopBtn.Disable()
	}

	apply := func(name string) error {
		found, ok := preset.ByName(name, code)
		if !ok {
			return errNoMethod
		}

		if session != nil {
			session.Stop()
			session = nil
		}

		started, err := start(found, own.List(), keep)
		if err != nil {
			return err
		}

		session = started
		chosen = found
		go session.Run()

		fyne.Do(func() {
			progress.SetText(fmt.Sprintf(l.T(lang.Searching), found.Name(code)))
		})

		// Let the fresh tunnel settle before the probe, or it reads a half-open state.
		time.Sleep(2 * time.Second)

		return nil
	}

	works := func() bool {
		verdict, _ := probe.Host(&net.Dialer{Timeout: 5 * time.Second}, "discord.com", 5*time.Second)

		return verdict == probe.Clear
	}

	pick.OnTapped = func() {
		if searching {
			return
		}

		searching = true
		stopFlag.Store(false)
		button.Disable()
		choose.Disable()
		pick.Disable()
		stopBtn.Enable()
		progress.SetText("")

		go func() {
			_, ok := autopick.Try(preset.Names(code), apply, works, func() bool { return stopFlag.Load() })

			fyne.Do(func() {
				searching = false
				button.Enable()
				choose.Enable()
				pick.Enable()
				stopBtn.Disable()

				if !ok {
					if stopFlag.Load() {
						progress.SetText("")
					} else {
						progress.SetText(l.T(lang.NoneWorked))
					}

					return
				}

				choose.SetSelected(preset.Label(chosen, code, favourite))
				a.Preferences().SetString("method", chosen.Key)
				progress.SetText("")
				refreshFav()
			})
		}()
	}

	caveat := widget.NewLabel(l.T(lang.Caveat))

	startup := widget.NewCheck(l.T(lang.Autostart), func(on bool) {
		if err := setAutostart(on); err != nil {
			dialog.ShowError(err, window)
		}
	})
	startup.Checked = autostartOn()

	entry := widget.NewEntry()
	entry.SetPlaceHolder("instagram.com")

	add := widget.NewButton(l.T(lang.AddSite), func() {
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

				if searching {
					return
				}

				status := l.T(lang.StatusOff)
				if session != nil {
					status = l.T(lang.StatusOn)
				}
				logHead.SetText(l.T(lang.MethodLabel) + " " + chosen.Name(code) + " · " + status)
				now.SetText(fmt.Sprintf(l.T(lang.Now), chosen.Name(code)))

				if session == nil {
					speed.SetText(fmt.Sprintf(l.T(lang.SpeedFmt), "—"))

					return
				}

				speed.SetText(fmt.Sprintf(l.T(lang.SpeedFmt), meter.Human(rate.Sample(session.Downloaded(), time.Now()))))
			})
		}
	}()

	language := widget.NewSelect([]string{"Русский", "English"}, func(name string) {
		picked := "ru"
		if name == "English" {
			picked = "en"
		}

		a.Preferences().SetString("lang", picked)
	})
	if code == "en" {
		language.SetSelected("English")
	} else {
		language.SetSelected("Русский")
	}

	obhod := container.NewVBox(
		container.NewHBox(dot, word),
		button,
		widget.NewLabel(l.T(lang.MethodLabel)),
		container.NewBorder(nil, nil, nil, fav, choose),
		now,
		container.NewBorder(nil, nil, nil, stopBtn, pick),
		widget.NewLabel(l.T(lang.AutoPickNote)),
		progress,
		caveat,
		startup,
		speed,
	)

	yourSites := container.NewBorder(
		container.NewVBox(
			container.NewBorder(nil, nil, nil, add, entry),
			count,
			widget.NewLabel(l.T(lang.YourSites)),
		),
		nil, nil, nil,
		container.NewVScroll(container.NewVBox(
			ownRows,
			widget.NewSeparator(),
			widget.NewLabel(l.T(lang.AlwaysSites)),
			always,
		)),
	)

	logsTab := container.NewBorder(
		container.NewVBox(
			logHead,
			container.NewHBox(
				widget.NewButton(l.T(lang.Copy), func() {
					if logView.Text == "" {
						return
					}

					window.Clipboard().SetContent(logView.Text)
				}),
				widget.NewButton(l.T(lang.Clear), func() {
					book.Clear()
					logView.SetText("")
				}),
			),
		),
		nil, nil, nil,
		container.NewScroll(logView),
	)

	about := container.NewVBox(
		widget.NewLabel(l.T(lang.AboutWhat)),
		widget.NewLabel(l.T(lang.AboutOpens)),
		widget.NewSeparator(),
		widget.NewLabel(l.T(lang.AboutMethods)),
		widget.NewSeparator(),
		widget.NewLabel(l.T(lang.AboutUnsigned)),
		widget.NewSeparator(),
		widget.NewLabel(l.T(lang.LanguageLabel)),
		language,
		widget.NewLabel(l.T(lang.RestartNote)),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem(l.T(lang.TabBypass), obhod),
		container.NewTabItem(l.T(lang.TabSites), yourSites),
		container.NewTabItem(l.T(lang.TabLogs), logsTab),
		container.NewTabItem(l.T(lang.TabAbout), about),
	)
	window.SetContent(tabs)
	window.Resize(fyne.NewSize(440, 620))

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
