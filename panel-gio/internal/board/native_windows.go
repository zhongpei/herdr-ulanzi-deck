//go:build windows

package board

import "gioui.org/app"

// nativeViewHandle extracts the HWND from a Gio ViewEvent on Windows.
func nativeViewHandle(e app.ViewEvent) uintptr {
	if m, ok := e.(app.Win32ViewEvent); ok && m.Valid() {
		return m.HWND
	}
	return 0
}
