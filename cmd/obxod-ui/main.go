package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"obxod/internal/logbook"
	"obxod/internal/preset"
	"obxod/internal/runner"
	"obxod/internal/sites"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const fakeQUICName = "www.google.com"

const (
	scrMain = iota
	scrMethod
	scrSites
	scrLogs
	scrSettings
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Obxod"), app.Size(unit.Dp(420), unit.Dp(660)))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette.Bg = colBg
	th.Palette.Fg = colCream
	th.Palette.ContrastBg = colTeal
	th.Palette.ContrastFg = colBg

	img, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		return err
	}
	icon := paint.NewImageOp(img)

	driverErr := unpackDriver()
	own, _ := sites.Load(sitesFile())
	book := logbook.New(200)
	keep := func(line string) {
		if logbook.Worth(line) {
			book.Add(line)
		}
	}

	all := preset.All()
	chosen := all[0]
	useFake := false

	var session *runner.Session
	var startErr error

	launch := func() {
		started, err := start(chosen, own.List(), keep, useFake)
		if err != nil {
			startErr = err

			return
		}

		startErr = nil
		session = started
		go session.Run()
	}

	toggle := func() {
		if session != nil {
			session.Stop()
			session = nil

			return
		}

		launch()
	}

	restart := func() {
		if session == nil {
			return
		}

		session.Stop()
		session = nil
		launch()
	}

	screen := scrMain

	var znak, toMethod, clearLog, copyLog widget.Clickable
	var methodRows = make([]widget.Clickable, len(all))
	var navClicks [4]widget.Clickable
	var methodList, sitesList, logList widget.List
	methodList.Axis = layout.Vertical
	sitesList.Axis = layout.Vertical
	logList.Axis = layout.Vertical

	var fakeBool, autoBool widget.Bool
	autoOn := autostartOn()
	autoBool.Value = autoOn

	navTargets := []int{scrMain, scrSites, scrLogs, scrSettings}

	go func() {
		for range time.Tick(time.Second) {
			w.Invalidate()
		}
	}()

	var ops op.Ops

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			if session != nil {
				session.Stop()
			}

			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			if znak.Clicked(gtx) {
				toggle()
			}
			if toMethod.Clicked(gtx) {
				screen = scrMethod
			}
			if clearLog.Clicked(gtx) {
				book.Clear()
			}
			if copyLog.Clicked(gtx) {
				gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(book.Text()))})
			}
			for i := range navClicks {
				if navClicks[i].Clicked(gtx) {
					screen = navTargets[i]
				}
			}
			for i := range methodRows {
				if methodRows[i].Clicked(gtx) {
					chosen = all[i]
					restart()
					screen = scrMain
				}
			}
			if fakeBool.Value != useFake {
				useFake = fakeBool.Value
				restart()
			}
			if autoBool.Value != autoOn {
				autoOn = autoBool.Value
				_ = setAutostart(autoOn)
			}

			fill(gtx, colBg)

			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return navBar(gtx, th, activeNav(screen), navClicks[:])
				}),
				layout.Flexed(1, func(gtx C) D {
					switch screen {
					case scrMethod:
						return layoutMethod(gtx, th, all, chosen, &methodList, methodRows)
					case scrSites:
						return layoutSites(gtx, th, own.List(), &sitesList)
					case scrLogs:
						return layoutLogs(gtx, th, book.Text(), &logList, &copyLog, &clearLog)
					case scrSettings:
						return layoutSettings(gtx, th, &fakeBool, &autoBool)
					default:
						return layoutMain(gtx, th, icon, chosen, session != nil, driverErr, startErr, &znak, &toMethod)
					}
				}),
			)

			e.Frame(gtx.Ops)
		}
	}
}

func activeNav(screen int) int {
	switch screen {
	case scrSites:
		return 1
	case scrLogs:
		return 2
	case scrSettings:
		return 3
	default:
		return 0
	}
}

func navBar(gtx C, th *material.Theme, active int, clicks []widget.Clickable) D {
	labels := []string{"Обход", "Сайты", "Логи", "Настройки"}
	items := make([]layout.FlexChild, len(labels))
	for i := range labels {
		i := i
		items[i] = layout.Flexed(1, func(gtx C) D {
			return clicks[i].Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				lbl := material.Body2(th, labels[i])
				lbl.Alignment = text.Middle
				lbl.Color = colDim
				if i == active {
					lbl.Color = colTeal
				}

				return layout.UniformInset(unit.Dp(12)).Layout(gtx, lbl.Layout)
			})
		})
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, items...)
		}),
		layout.Rigid(hline),
	)
}

func layoutMain(gtx C, th *material.Theme, icon paint.ImageOp, chosen preset.Preset, on bool, driverErr, startErr error, znak, toMethod *widget.Clickable) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return znak.Layout(gtx, func(gtx C) D {
					side := gtx.Dp(132)
					gtx.Constraints = layout.Exact(image.Pt(side, side))

					return widget.Image{Src: icon, Fit: widget.Contain}.Layout(gtx)
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
			layout.Rigid(func(gtx C) D {
				title := material.H6(th, "Obxod")
				title.Color = colCream

				return title.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx C) D {
				status := material.Body1(th, "Выключено")
				status.Color = colDim
				if on {
					status = material.Body1(th, "Работает")
					status.Color = colTeal
				}

				return status.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return toMethod.Layout(gtx, func(gtx C) D {
					lbl := material.Body2(th, "Способ:  "+chosen.Name("ru"))
					lbl.Color = colCream

					return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, lbl.Layout)
				})
			}),
			layout.Rigid(func(gtx C) D {
				note := ""
				if driverErr != nil {
					note = "драйвер не загрузился: " + driverErr.Error()
				}
				if startErr != nil {
					note = startErr.Error()
				}
				if note == "" {
					return D{}
				}

				lbl := material.Caption(th, note)
				lbl.Color = colDim

				return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, lbl.Layout)
			}),
		)
	})
}

func layoutMethod(gtx C, th *material.Theme, all []preset.Preset, chosen preset.Preset, list *widget.List, rows []widget.Clickable) D {
	return material.List(th, list).Layout(gtx, len(all), func(gtx C, i int) D {
		return methodRow(gtx, th, all[i], all[i].Key == chosen.Key, &rows[i])
	})
}

func methodRow(gtx C, th *material.Theme, p preset.Preset, selected bool, click *widget.Clickable) D {
	return click.Layout(gtx, func(gtx C) D {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx C) D {
				if selected {
					rrect(gtx, colTint, gtx.Dp(10))
				}

				return D{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X

				return layout.UniformInset(unit.Dp(13)).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Right: unit.Dp(11)}.Layout(gtx, func(gtx C) D {
								return tick(gtx, selected)
							})
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									name := material.Body1(th, p.Name("ru"))
									name.Color = colCream
									name.Font.Weight = font.Bold

									return name.Layout(gtx)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
								layout.Rigid(func(gtx C) D {
									desc := material.Caption(th, p.Desc("ru"))
									desc.Color = colDim

									return desc.Layout(gtx)
								}),
							)
						}),
					)
				})
			}),
		)
	})
}

func layoutSites(gtx C, th *material.Theme, ownList []string, list *widget.List) D {
	lines := []string{fmt.Sprintf("Обходится сайтов: %d", len(preset.HostsWith(ownList)))}
	lines = append(lines, "", "Ваши сайты:")
	if len(ownList) == 0 {
		lines = append(lines, "—")
	} else {
		lines = append(lines, ownList...)
	}
	lines = append(lines, "", "Всегда обходятся:")
	lines = append(lines, preset.Hosts()...)

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
		return material.List(th, list).Layout(gtx, len(lines), func(gtx C, i int) D {
			lbl := material.Body2(th, lines[i])
			lbl.Color = colCream
			if strings.HasSuffix(lines[i], ":") || lines[i] == "—" {
				lbl.Color = colDim
			}

			return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, lbl.Layout)
		})
	})
}

func layoutLogs(gtx C, th *material.Theme, body string, list *widget.List, copyBtn, clear *widget.Clickable) D {
	lines := strings.Split(body, "\n")

	ghost := func(click *widget.Clickable, label string) material.ButtonStyle {
		b := material.Button(th, click, label)
		b.Background = colPanel
		b.Color = colCream

		return b
	}

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, ghost(copyBtn, "Скопировать").Layout)
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, ghost(clear, "Очистить").Layout)
						}),
					)
				})
			}),
			layout.Flexed(1, func(gtx C) D {
				return material.List(th, list).Layout(gtx, len(lines), func(gtx C, i int) D {
					lbl := material.Body2(th, lines[i])
					lbl.Color = colDim

					return lbl.Layout(gtx)
				})
			}),
		)
	})
}

func layoutSettings(gtx C, th *material.Theme, fake, auto *widget.Bool) D {
	row := func(label string, b *widget.Bool) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						lbl := material.Body1(th, label)
						lbl.Color = colCream

						return lbl.Layout(gtx)
					}),
					layout.Rigid(material.Switch(th, b, label).Layout),
				)
			})
		})
	}

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			row("Фейковать QUIC (быстрее, но не везде)", fake),
			row("Запускать при старте Windows", auto),
		)
	})
}

func hline(gtx C) D {
	sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
	defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: colSep}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	return D{Size: sz}
}

func tick(gtx C, selected bool) D {
	d := gtx.Dp(16)
	sz := image.Pt(d, d)
	c := colSep
	if selected {
		c = colTeal
	}

	defer clip.Ellipse{Max: sz}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	return D{Size: sz}
}

func rrect(gtx C, c color.NRGBA, r int) {
	rr := clip.RRect{Rect: image.Rectangle{Max: gtx.Constraints.Min}, SE: r, SW: r, NE: r, NW: r}
	defer rr.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func fill(gtx C, c color.NRGBA) {
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func sitesFile() string {
	exe, err := os.Executable()
	if err != nil {
		return "sites.txt"
	}

	return filepath.Join(filepath.Dir(exe), "sites.txt")
}

func start(p preset.Preset, extra []string, report func(string), fakeQUIC bool) (*runner.Session, error) {
	set, err := p.RulesVoice(extra)
	if err != nil {
		return nil, err
	}

	cfg := runner.Config{
		Rules:  set,
		Ports:  runner.DefaultPorts(),
		Voice:  runner.DefaultVoice(),
		Voiced: voiceDatagram,
		Wet:    true,
		Report: report,
	}

	// Faking the Initial keeps the real handshake alive, so http/3 goes on working;
	// dropping is the fallback for a dpi the fake does not fool.
	if fakeQUIC {
		cfg.FakeQUIC = fakeQUICName
	} else {
		cfg.DropQUIC = true
	}

	return runner.Open(cfg)
}
