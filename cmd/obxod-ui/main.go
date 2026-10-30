package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"obxod/internal/lang"
	"obxod/internal/logbook"
	"obxod/internal/meter"
	"obxod/internal/preset"
	"obxod/internal/probe"
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

var (
	iconStar, _       = widget.NewIcon(icons.ToggleStar)
	iconStarBorder, _ = widget.NewIcon(icons.ToggleStarBorder)
	iconChevron, _    = widget.NewIcon(icons.NavigationChevronRight)
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Obxod"), app.Size(unit.Dp(420), unit.Dp(660)), app.MinSize(unit.Dp(360), unit.Dp(540)))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(manropeCollection()))
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
	var rate meter.Rate
	speed := ""
	var lastSpeed time.Time

	pr := loadPrefs()
	favourites := pr.Favourites
	uses := pr.Uses
	code := pr.Lang
	if code == "" {
		code = "ru"
	}

	persist := func() {
		prefs{Favourites: favourites, Uses: uses, Lang: code}.save()
	}

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
		rate = meter.Rate{}
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

	var znak, toMethod, clearLog, copyLog, langClick widget.Clickable
	var copiedAt time.Time
	const copiedShown = 1500 * time.Millisecond

	var checkBtn widget.Clickable
	checkResult := make(chan probe.Verdict, 1)
	var checkText string
	var checking bool
	var addSite widget.Clickable
	var delSites [64]widget.Clickable
	var siteEditor widget.Editor
	siteEditor.SingleLine = true
	siteEditor.Submit = true
	selectRows := make([]widget.Clickable, len(all))
	starRows := make([]widget.Clickable, len(all))
	var navClicks [4]widget.Clickable
	var methodList, sitesList, logList, settingsList widget.List
	methodList.Axis = layout.Vertical
	sitesList.Axis = layout.Vertical
	logList.Axis = layout.Vertical
	logList.ScrollToEnd = true
	settingsList.Axis = layout.Vertical

	var fakeBool, autoBool widget.Bool
	autoOn := autostartOn()
	autoBool.Value = autoOn

	navTargets := []int{scrMain, scrSites, scrLogs, scrSettings}

	var live atomic.Bool
	go func() {
		for range time.Tick(time.Second) {
			if live.Load() {
				w.Invalidate()
			}
		}
	}()

	var ops op.Ops
	var anim, navAnim float32
	var scrAnim float32 = 1
	var lastT time.Time
	lastScreen := screen

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
				copiedAt = gtx.Now
			}
			for i := range navClicks {
				if navClicks[i].Clicked(gtx) {
					screen = navTargets[i]
				}
			}
			for i := range all {
				if selectRows[i].Clicked(gtx) {
					chosen = all[i]
					uses[all[i].Key]++
					persist()
					restart()
					screen = scrMain
				}
				if starRows[i].Clicked(gtx) {
					favourites = toggleFav(favourites, all[i].Key)
					persist()
				}
			}
			if langClick.Clicked(gtx) {
				if code == "en" {
					code = "ru"
				} else {
					code = "en"
				}
				persist()
			}
			addCurrent := func() {
				name := strings.TrimSpace(siteEditor.Text())
				if name == "" {
					return
				}
				own.Add(name)
				_ = own.Save(sitesFile())
				siteEditor.SetText("")
				restart()
			}
			if addSite.Clicked(gtx) {
				addCurrent()
			}
			for {
				ev, ok := siteEditor.Update(gtx)
				if !ok {
					break
				}
				if _, ok := ev.(widget.SubmitEvent); ok {
					addCurrent()
				}
			}
			select {
			case v := <-checkResult:
				checking = false
				checkText = checkLine(code, v)
			default:
			}
			if checkBtn.Clicked(gtx) {
				host := strings.TrimSpace(siteEditor.Text())
				if host != "" && !checking {
					checking = true
					checkText = lang.Of(code).T(lang.CheckWorking)
					go func(name string) {
						v, _ := probe.Host(&net.Dialer{Timeout: 6 * time.Second}, name, 6*time.Second)
						checkResult <- v
						w.Invalidate()
					}(host)
				}
			}
			ownNames := own.List()
			for i := range ownNames {
				if i < len(delSites) && delSites[i].Clicked(gtx) {
					own.Remove(ownNames[i])
					_ = own.Save(sitesFile())
					restart()
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

			if screen != lastScreen {
				scrAnim = 0
				if screen == scrSites {
					gtx.Execute(key.FocusCmd{Tag: &siteEditor})
				}
				lastScreen = screen
			}

			onTarget := float32(0)
			if session != nil {
				onTarget = 1
			}
			navTarget := float32(activeNav(screen))
			if !lastT.IsZero() {
				dt := float32(gtx.Now.Sub(lastT).Seconds())
				anim = approach(anim, onTarget, dt, 9)
				navAnim = approach(navAnim, navTarget, dt, 12)
				scrAnim = approach(scrAnim, 1, dt, 7)
			}
			lastT = gtx.Now
			if anim != onTarget || navAnim != navTarget || scrAnim != 1 {
				gtx.Execute(op.InvalidateCmd{})
			}

			live.Store(session != nil)

			if session != nil {
				if gtx.Now.Sub(lastSpeed) >= time.Second {
					speed = humanRate(rate.Sample(session.Downloaded(), gtx.Now), code)
					lastSpeed = gtx.Now
				}
			} else {
				speed = ""
			}

			copied := !copiedAt.IsZero() && gtx.Now.Sub(copiedAt) < copiedShown
			if copied {
				gtx.Execute(op.InvalidateCmd{At: copiedAt.Add(copiedShown)})
			}

			fill(gtx, colBg)

			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return navBar(gtx, th, activeNav(screen), navAnim, code, navClicks[:])
				}),
				layout.Flexed(1, func(gtx C) D {
					dy := int((1 - scrAnim) * float32(gtx.Dp(12)))
					defer op.Offset(image.Pt(0, dy)).Push(gtx.Ops).Pop()
					defer paint.PushOpacity(gtx.Ops, scrAnim).Pop()

					dl := 0
					traffic := map[string]int{}
					if session != nil {
						dl = session.Downloaded()
						traffic = session.Traffic()
					}

					switch screen {
					case scrMethod:
						return layoutMethod(gtx, th, all, chosen, favourites, uses, code, &methodList, selectRows, starRows)
					case scrSites:
						return layoutSites(gtx, th, own.List(), code, &siteEditor, &addSite, &checkBtn, delSites[:], &sitesList, checkText)
					case scrLogs:
						return layoutLogs(gtx, th, book.Text(), code, dl, traffic, &logList, &copyLog, &clearLog, copied)
					case scrSettings:
						return layoutSettings(gtx, th, code, &fakeBool, &autoBool, &langClick, &settingsList)
					default:
						return layoutMain(gtx, th, icon, chosen, session != nil, anim, code, speed, driverErr, startErr, &znak, &toMethod)
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

func navBar(gtx C, th *material.Theme, active int, pos float32, code string, clicks []widget.Clickable) D {
	l := lang.Of(code)
	labels := []string{l.T(lang.TabBypass), l.T(lang.TabSites), l.T(lang.TabLogs), l.T(lang.TabAbout)}
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
		layout.Rigid(func(gtx C) D {
			return navUnderline(gtx, pos, len(labels))
		}),
	)
}

func navUnderline(gtx C, pos float32, tabs int) D {
	w := gtx.Constraints.Max.X
	h := gtx.Dp(2)
	tabW := w / tabs

	base := clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops)
	paint.ColorOp{Color: colSep}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	base.Pop()

	pad := gtx.Dp(18)
	seg := tabW - 2*pad
	x := int(pos*float32(tabW)) + pad

	off := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
	bar := clip.Rect{Max: image.Pt(seg, h)}.Push(gtx.Ops)
	paint.ColorOp{Color: colTeal}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bar.Pop()
	off.Pop()

	return D{Size: image.Pt(w, h)}
}

func layoutMain(gtx C, th *material.Theme, icon paint.ImageOp, chosen preset.Preset, on bool, anim float32, code, speed string, driverErr, startErr error, znak, toMethod *widget.Clickable) D {
	l := lang.Of(code)

	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return znak.Layout(gtx, func(gtx C) D {
					o := float32(1)
					if znak.Pressed() {
						o = 0.6
					}
					defer paint.PushOpacity(gtx.Ops, o).Pop()

					return layout.Stack{Alignment: layout.Center}.Layout(gtx,
						layout.Stacked(func(gtx C) D {
							return halo(gtx, anim)
						}),
						layout.Stacked(func(gtx C) D {
							side := gtx.Dp(132)
							gtx.Constraints = layout.Exact(image.Pt(side, side))

							return widget.Image{Src: icon, Fit: widget.Contain}.Layout(gtx)
						}),
					)
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
				text, c := l.T(lang.StatusOff), colDim
				if on {
					text, c = l.T(lang.StatusOn), colTeal
				}

				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: unit.Dp(7)}.Layout(gtx, func(gtx C) D {
							return dot(gtx, c)
						})
					}),
					layout.Rigid(func(gtx C) D {
						status := material.Body1(th, text)
						status.Color = c

						return status.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if !on || speed == "" {
					return D{}
				}

				s := material.Caption(th, speed)
				s.Color = colDim

				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, s.Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: unit.Dp(22)}.Layout(gtx, func(gtx C) D {
					return toMethod.Layout(gtx, func(gtx C) D {
						o := float32(1)
						if toMethod.Pressed() {
							o = 0.6
						}
						defer paint.PushOpacity(gtx.Ops, o).Pop()

						return methodCard(gtx, th, chosen, code)
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				note := ""
				if driverErr != nil {
					note = l.T(lang.DriverBad) + ": " + driverErr.Error()
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

func layoutMethod(gtx C, th *material.Theme, all []preset.Preset, chosen preset.Preset, favourites []string, uses map[string]int, code string, list *widget.List, selectRows, starRows []widget.Clickable) D {
	order := methodOrder(all, favourites, uses)

	return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx C) D {
		return material.List(th, list).Layout(gtx, len(order), func(gtx C, j int) D {
			i := order[j]

			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx C) D {
				return methodRow(gtx, th, all[i], code, all[i].Key == chosen.Key, starred(favourites, all[i].Key), &selectRows[i], &starRows[i])
			})
		})
	})
}

func methodCard(gtx C, th *material.Theme, chosen preset.Preset, code string) D {
	l := lang.Of(code)
	w := gtx.Dp(300)
	if w > gtx.Constraints.Max.X {
		w = gtx.Constraints.Max.X
	}
	gtx.Constraints.Min.X = w
	gtx.Constraints.Max.X = w

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			rrect(gtx, colPanel, gtx.Dp(12))

			return D{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X

			return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								hint := material.Caption(th, l.T(lang.MethodHint))
								hint.Color = colDim

								return hint.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
							layout.Rigid(func(gtx C) D {
								n := material.Body1(th, chosen.Name(code))
								n.Color = colCream
								n.Font.Weight = font.Bold

								return n.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						sz := gtx.Dp(24)
						gtx.Constraints = layout.Exact(image.Pt(sz, sz))

						return iconChevron.Layout(gtx, colDim)
					}),
				)
			})
		}),
	)
}

func methodRow(gtx C, th *material.Theme, p preset.Preset, code string, selected, fav bool, selectClick, starClick *widget.Clickable) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			bg := colPanel
			if selected {
				bg = colTint
			}
			rrect(gtx, bg, gtx.Dp(12))

			return D{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X

			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return selectClick.Layout(gtx, func(gtx C) D {
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
											name := material.Body1(th, p.Name(code))
											name.Color = colCream
											name.Font.Weight = font.Bold

											return name.Layout(gtx)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
										layout.Rigid(func(gtx C) D {
											desc := material.Caption(th, p.Desc(code))
											desc.Color = colDim

											return desc.Layout(gtx)
										}),
									)
								}),
							)
						})
					})
				}),
				layout.Rigid(func(gtx C) D {
					return starClick.Layout(gtx, func(gtx C) D {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx C) D {
							sz := gtx.Dp(22)
							gtx.Constraints = layout.Exact(image.Pt(sz, sz))
							ic := iconStarBorder
							col := colDim
							if fav {
								ic = iconStar
								col = colTeal
							}

							return ic.Layout(gtx, col)
						})
					})
				}),
			)
		}),
	)
}

func layoutSites(gtx C, th *material.Theme, ownList []string, code string, editor *widget.Editor, addBtn, checkBtn *widget.Clickable, dels []widget.Clickable, list *widget.List, checkText string) D {
	l := lang.Of(code)

	content := func(gtx C) D {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				t := material.Body2(th, fmt.Sprintf(l.T(lang.CountFmt), len(preset.HostsWith(ownList))))
				t.Color = colDim

				return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, t.Layout)
			}),
			layout.Rigid(func(gtx C) D {
				t := material.Body2(th, l.T(lang.YourSites))
				t.Color = colDim

				return t.Layout(gtx)
			}),
		}

		for i, name := range ownList {
			i, name := i, name
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if i >= len(dels) {
								return D{}
							}

							x := material.Button(th, &dels[i], "×")
							x.Background = colPanel
							x.Color = colCream

							return layout.Inset{Right: unit.Dp(10)}.Layout(gtx, x.Layout)
						}),
						layout.Flexed(1, func(gtx C) D {
							n := material.Body1(th, name)
							n.Color = colCream

							return n.Layout(gtx)
						}),
					)
				})
			}))
		}

		children = append(children,
			layout.Rigid(func(gtx C) D {
				t := material.Body2(th, l.T(lang.AlwaysSites))
				t.Color = colDim

				return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(6)}.Layout(gtx, t.Layout)
			}),
			layout.Rigid(func(gtx C) D {
				t := material.Body2(th, strings.Join(preset.Hosts(), "  ·  "))
				t.Color = colDim

				return t.Layout(gtx)
			}),
		)

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							ed := material.Editor(th, editor, "instagram.com")
							ed.Color = colCream
							ed.HintColor = colDim

							return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx C) D {
								return widget.Border{Color: colSep, Width: unit.Dp(1), CornerRadius: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
									return layout.UniformInset(unit.Dp(11)).Layout(gtx, ed.Layout)
								})
							})
						}),
						layout.Rigid(func(gtx C) D {
							b := material.Button(th, addBtn, l.T(lang.AddSite))
							b.Background = colTeal
							b.Color = colBg

							return b.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							c := material.Button(th, checkBtn, l.T(lang.Check))
							c.Background = colPanel
							c.Color = colCream

							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, c.Layout)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if checkText == "" {
					return D{}
				}
				t := material.Caption(th, checkText)
				t.Color = colDim

				return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, t.Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				return material.List(th, list).Layout(gtx, 1, func(gtx C, _ int) D {
					return content(gtx)
				})
			}),
		)
	})
}

func checkLine(code string, v probe.Verdict) string {
	l := lang.Of(code)

	switch v {
	case probe.Clear:
		return l.T(lang.CheckClear)
	case probe.NoConnect:
		return l.T(lang.CheckByIP)
	default:
		return l.T(lang.CheckByName)
	}
}

func layoutLogs(gtx C, th *material.Theme, body, code string, downloaded int, traffic map[string]int, list *widget.List, copyBtn, clear *widget.Clickable, copied bool) D {
	l := lang.Of(code)
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
				children := []layout.FlexChild{
					layout.Rigid(func(gtx C) D {
						t := material.Body1(th, fmt.Sprintf(l.T(lang.TrafficFmt), humanBytes(downloaded, code)))
						t.Color = colTeal
						t.Font.Weight = font.Bold

						return t.Layout(gtx)
					}),
				}

				for _, row := range byService(traffic) {
					row := row
					children = append(children, layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, func(gtx C) D {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx C) D {
												return dot(gtx, serviceColor(row.name))
											})
										}),
										layout.Rigid(func(gtx C) D {
											n := material.Body2(th, row.name)
											n.Color = colCream

											return n.Layout(gtx)
										}),
									)
								}),
								layout.Rigid(func(gtx C) D {
									v := material.Body2(th, humanBytes(row.bytes, code))
									v.Color = colDim

									return v.Layout(gtx)
								}),
							)
						})
					}))
				}

				return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							copyB := ghost(copyBtn, l.T(lang.Copy))
							if copied {
								copyB = ghost(copyBtn, l.T(lang.Copied))
								copyB.Background = colTint
								copyB.Color = colTeal
							}

							return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, copyB.Layout)
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, ghost(clear, l.T(lang.Clear)).Layout)
						}),
					)
				})
			}),
			layout.Flexed(1, func(gtx C) D {
				if strings.TrimSpace(body) == "" {
					p := material.Body2(th, l.T(lang.LogsEmpty))
					p.Color = colDim
					p.Alignment = text.Middle

					return layout.Center.Layout(gtx, p.Layout)
				}

				return material.List(th, list).Layout(gtx, len(lines), func(gtx C, i int) D {
					lbl := material.Body2(th, lines[i])
					lbl.Color = logColor(lines[i])

					return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, lbl.Layout)
				})
			}),
		)
	})
}

func layoutSettings(gtx C, th *material.Theme, code string, fake, auto *widget.Bool, langClick *widget.Clickable, list *widget.List) D {
	l := lang.Of(code)

	card := func(label, sub string, b *widget.Bool) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						rrect(gtx, colPanel, gtx.Dp(12))

						return D{Size: gtx.Constraints.Min}
					}),
					layout.Stacked(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X

						return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											title := material.Body1(th, label)
											title.Color = colCream

											return title.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											if sub == "" {
												return D{}
											}
											s := material.Caption(th, sub)
											s.Color = colDim

											return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, s.Layout)
										}),
									)
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
								layout.Rigid(material.Switch(th, b, label).Layout),
							)
						})
					}),
				)
			})
		})
	}

	langName := "Русский"
	if code == "en" {
		langName = "English"
	}

	langRow := layout.Rigid(func(gtx C) D {
		return langClick.Layout(gtx, func(gtx C) D {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					rrect(gtx, colPanel, gtx.Dp(12))

					return D{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X

					return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D {
								t := material.Body1(th, l.T(lang.LanguageLabel))
								t.Color = colCream

								return t.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								v := material.Body1(th, langName)
								v.Color = colTeal

								return v.Layout(gtx)
							}),
						)
					})
				}),
			)
		})
	})

	about := func(text string, c color.NRGBA, top int) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			t := material.Caption(th, text)
			t.Color = c

			return layout.Inset{Top: unit.Dp(top)}.Layout(gtx, t.Layout)
		})
	}

	separator := layout.Rigid(func(gtx C) D {
		return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(14)}.Layout(gtx, func(gtx C) D {
			sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
			defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
			fill(gtx, colSep)

			return D{Size: sz}
		})
	})

	return material.List(th, list).Layout(gtx, 1, func(gtx C, _ int) D {
		return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				card(l.T(lang.FastQUICShort), l.T(lang.FastQUICSub), fake),
				card(l.T(lang.Autostart), l.T(lang.AutostartSub), auto),
				langRow,
				separator,
				about(l.T(lang.AboutWhat), colCream, 0),
				about(l.T(lang.AboutOpens), colDim, 6),
				about(l.T(lang.AboutUnsigned), colDim, 6),
			)
		})
	})
}

func approach(cur, tgt, dt, rate float32) float32 {
	d := tgt - cur
	if d < 0 {
		d = -d
	}
	if d < 0.001 {
		return tgt
	}

	f := dt * rate
	if f > 1 {
		f = 1
	}

	return cur + (tgt-cur)*f
}

func halo(gtx C, t float32) D {
	d := gtx.Dp(130) + int(t*float32(gtx.Dp(72)))
	sz := image.Pt(d, d)
	c := color.NRGBA{R: 0x2f, G: 0x7d, B: 0x64, A: uint8(t * 0x3a)}

	defer clip.Ellipse{Max: sz}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	return D{Size: sz}
}

type serviceRow struct {
	name  string
	bytes int
}

func dot(gtx C, c color.NRGBA) D {
	d := gtx.Dp(9)
	sz := image.Pt(d, d)

	defer clip.Ellipse{Max: sz}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	return D{Size: sz}
}

func serviceColor(name string) color.NRGBA {
	switch name {
	case "Discord":
		return color.NRGBA{R: 0x58, G: 0x65, B: 0xf2, A: 0xff}
	case "YouTube":
		return color.NRGBA{R: 0xff, G: 0x3b, B: 0x30, A: 0xff}
	case "Instagram":
		return color.NRGBA{R: 0xe1, G: 0x30, B: 0x6c, A: 0xff}
	case "Telegram":
		return color.NRGBA{R: 0x3a, G: 0xa0, B: 0xdc, A: 0xff}
	case "X":
		return colCream
	default:
		return colDim
	}
}

func serviceOf(host string) string {
	switch {
	case strings.Contains(host, "discord"):
		return "Discord"
	case strings.Contains(host, "youtube") || strings.Contains(host, "googlevideo") || strings.Contains(host, "ytimg"):
		return "YouTube"
	case strings.Contains(host, "twitter") || strings.Contains(host, "twimg") || strings.Contains(host, "x.com") || strings.Contains(host, "t.co"):
		return "X"
	case strings.Contains(host, "instagram") || strings.Contains(host, "cdninstagram") || strings.Contains(host, "fbcdn"):
		return "Instagram"
	case strings.Contains(host, "telegram"):
		return "Telegram"
	default:
		return ""
	}
}

func byService(traffic map[string]int) []serviceRow {
	agg := map[string]int{}
	for host, bytes := range traffic {
		name := serviceOf(host)
		if name == "" {
			continue
		}
		agg[name] += bytes
	}

	rows := make([]serviceRow, 0, len(agg))
	for name, bytes := range agg {
		rows = append(rows, serviceRow{name: name, bytes: bytes})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].bytes > rows[j].bytes
	})

	return rows
}

func humanRate(perSecond float64, code string) string {
	suffix := "/с"
	if code == "en" {
		suffix = "/s"
	}

	return humanBytes(int(perSecond), code) + suffix
}

func humanBytes(n int, code string) string {
	kb, mb, gb := "КБ", "МБ", "ГБ"
	if code == "en" {
		kb, mb, gb = "KB", "MB", "GB"
	}

	f := float64(n)
	switch {
	case f >= 1<<30:
		return fmt.Sprintf("%.1f %s", f/(1<<30), gb)
	case f >= 1<<20:
		return fmt.Sprintf("%.1f %s", f/(1<<20), mb)
	case f >= 1<<10:
		return fmt.Sprintf("%.0f %s", f/(1<<10), kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func logColor(line string) color.NRGBA {
	switch {
	case strings.Contains(line, "ответил") || strings.Contains(line, "отправл") || strings.Contains(line, "✓"):
		return colTeal
	case strings.Contains(line, ".") && !strings.HasPrefix(strings.TrimSpace(line), "→"):
		return colCream
	default:
		return colDim
	}
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
