//go:build windows

package tray

import (
	"fmt"
	"syscall"

	"fyne.io/systray"
)

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	procShowWindow = user32.NewProc("ShowWindow")
)

const (
	swHide = 0
	swShow = 5
)

func updateTray(blocked, working, done, idle, unknown int) {
	// Windows: colored dot icon + tooltip (tray doesn't support rich text)
	png := severityDot(blocked, working, done)
	systray.SetIcon(png)
	tip := fmt.Sprintf("B%d  W%d  D%d  Idle:%d  Unknown:%d",
		blocked, working, done, idle, unknown)
	systray.SetTooltip(tip)
}

func nativeHide(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swHide)
}

func nativeShow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swShow)
}
