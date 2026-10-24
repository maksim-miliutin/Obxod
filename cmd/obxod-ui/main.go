package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
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
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Obxod"), app.Size(unit.Dp(420), unit.Dp(640)))
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

	var session *runner.Session
	var startErr error

	launch := func() {
		started, err := start(chosen, own.List(), keep, false)
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

	var ops op.Ops
	var znak widget.Clickable
	var toMethod widget.Clickable
	var back widget.Clickable
	var methodList widget.List
	methodList.Axis = layout.Vertical
	rows := make([]widget.Clickable, len(all))

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
			if back.Clicked(gtx) {
				screen = scrMain
			}
			for i := range rows {
				if rows[i].Clicked(gtx) {
					chosen = all[i]
					restart()
					screen = scrMain
				}
			}

			fill(gtx, colBg)

			switch screen {
			case scrMethod:
				layoutMethod(gtx, th, all, chosen, &methodList, rows, &back)
			default:
				layoutMain(gtx, th, icon, chosen, session != nil, driverErr, startErr, &znak, &toMethod)
			}

			e.Frame(gtx.Ops)
		}
	}
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

func layoutMethod(gtx C, th *material.Theme, all []preset.Preset, chosen preset.Preset, list *widget.List, rows []widget.Clickable, back *widget.Clickable) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return back.Layout(gtx, func(gtx C) D {
				lbl := material.Body1(th, "‹  Способ")
				lbl.Color = colDim

				return layout.UniformInset(unit.Dp(14)).Layout(gtx, lbl.Layout)
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			return material.List(th, list).Layout(gtx, len(all), func(gtx C, i int) D {
				return methodRow(gtx, th, all[i], all[i].Key == chosen.Key, &rows[i])
			})
		}),
	)
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

				return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx C) D {
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
				})
			}),
		)
	})
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
