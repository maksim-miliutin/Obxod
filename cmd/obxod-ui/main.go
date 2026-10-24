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
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
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

	chosen := preset.All()[0]

	var session *runner.Session
	var startErr error

	toggle := func() {
		if session != nil {
			session.Stop()
			session = nil

			return
		}

		started, err := start(chosen, own.List(), keep, false)
		if err != nil {
			startErr = err

			return
		}

		startErr = nil
		session = started
		go session.Run()
	}

	var ops op.Ops
	var znak widget.Clickable

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

			fill(gtx, colBg)

			layout.Center.Layout(gtx, func(gtx C) D {
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
						if session != nil {
							status = material.Body1(th, "Работает")
							status.Color = colTeal
						}

						return status.Layout(gtx)
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

			e.Frame(gtx.Ops)
		}
	}
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
