//go:build !darwin && !windows

package tray

import (
	"fmt"

	"fyne.io/systray"
)

func updateTray(blocked, working, done, idle, unknown int) {
	// Linux: plain SetTitle (no colour support in system tray)
	title := fmt.Sprintf("B%d W%d D%d", blocked, working, done)
	systray.SetTitle(title)
	tip := fmt.Sprintf("Blocked: %d  Working: %d  Done: %d  Idle: %d  Unknown: %d",
		blocked, working, done, idle, unknown)
	systray.SetTooltip(tip)
}

func nativeHide(uintptr) {}
func nativeShow(uintptr) {}
