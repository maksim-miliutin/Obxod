package main

import (
	"os"
	"sync/atomic"

	"fyne.io/systray"
)

// systrayLoop puts an icon in the notification area. A failure here must never
// take the window down, so it recovers and leaves up false, and the close button
// falls back to quitting.
func systrayLoop(show chan<- struct{}, up *atomic.Bool) {
	defer func() { _ = recover() }()

	onReady := func() {
		systray.SetTitle("Obxod")
		systray.SetTooltip("Obxod")
		systray.SetIcon(iconPNG)

		showItem := systray.AddMenuItem("Показать", "")
		quitItem := systray.AddMenuItem("Выход", "")
		up.Store(true)

		go func() {
			for {
				select {
				case <-showItem.ClickedCh:
					select {
					case show <- struct{}{}:
					default:
					}
				case <-quitItem.ClickedCh:
					systray.Quit()
					os.Exit(0)
				}
			}
		}()
	}

	systray.Run(onReady, func() {})
}
