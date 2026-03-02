package launcher

import (
	"fyne.io/systray"
)

// Minimal 1x1 transparent PNG as placeholder tray icon.
var defaultIcon = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02,
	0x00, 0x01, 0xe2, 0x21, 0xbc, 0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// RunTray starts the system tray icon. It blocks until the tray exits.
// Must be called from the main goroutine on macOS (AppKit requirement).
func RunTray(onOpen, onQuit func()) {
	systray.Run(func() {
		systray.SetIcon(defaultIcon)
		systray.SetTitle("Fllint")
		systray.SetTooltip("Fllint — Local AI")

		mOpen := systray.AddMenuItem("Open Fllint", "Open in browser")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit Fllint")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					if onOpen != nil {
						onOpen()
					}
				case <-mQuit.ClickedCh:
					if onQuit != nil {
						onQuit()
					}
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}

// QuitTray signals the system tray to exit.
func QuitTray() {
	systray.Quit()
}
