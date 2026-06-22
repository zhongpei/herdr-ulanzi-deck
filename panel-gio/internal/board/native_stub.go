//go:build !darwin && !windows

package board

import "gioui.org/app"

// nativeViewHandle is a no-op on platforms without native window hide/show.
func nativeViewHandle(e app.ViewEvent) uintptr { return 0 }
