//go:build darwin

package board

import "gioui.org/app"

// nativeViewHandle extracts the NSView pointer from a Gio ViewEvent on macOS.
func nativeViewHandle(e app.ViewEvent) uintptr {
	if m, ok := e.(app.AppKitViewEvent); ok && m.Valid() {
		return m.View
	}
	return 0
}
