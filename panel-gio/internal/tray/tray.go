// Package tray manages the system tray icon and window visibility for herdr-panel.
//
// Platform rendering:
//
//	macOS: attributedTitle with colored "B3 W5 D8" text on NSStatusItem
//	Windows: colored dot icon + tooltip (no rich text in tray)
//	Linux:  plain SetTitle("B3 W5 D8") + tooltip
package tray

import (
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"github.com/herdr-deck/herdrdeck/panel-gio/internal/store"
	"github.com/rs/zerolog/log"
)

// Tray manages system tray lifecycle and count display.
type Tray struct {
	store  *store.Store
	view   atomic.Uintptr
	hidden atomic.Bool
	endFn  func()
}

// New creates a Tray reading agent state from the given store.
func New(st *store.Store) *Tray {
	return &Tray{store: st, hidden: atomic.Bool{}}
}

// Start initialises the system tray. Must be called from the main goroutine
// before app.Main() on macOS, because it creates NSStatusItem via Cocoa.
func (t *Tray) Start() {
	start, end := systray.RunWithExternalLoop(t.onReady, t.onExit)
	t.endFn = end
	start()
	log.Debug().Msg("systray started")
}

// Stop tears down the system tray. Call after app.Main() returns.
func (t *Tray) Stop() {
	if t.endFn != nil {
		t.endFn()
		log.Debug().Msg("systray stopped")
	}
}

// SetView stores the native window handle (NSView on macOS, HWND on Windows).
func (t *Tray) SetView(v uintptr) {
	t.view.Store(v)
}

// Hide hides the application window via native OS bridge.
func (t *Tray) Hide() {
	v := t.view.Load()
	if v == 0 {
		log.Warn().Msg("tray hide skipped: no native view handle")
		return
	}
	nativeHide(v)
	t.hidden.Store(true)
	log.Debug().Msg("window hidden to tray")
}

// Show makes the application window visible.
func (t *Tray) Show() {
	v := t.view.Load()
	if v == 0 {
		log.Warn().Msg("tray show skipped: no native view handle")
		return
	}
	nativeShow(v)
	t.hidden.Store(false)
	log.Debug().Msg("window shown from tray")
}

// --- internal ---

func (t *Tray) onReady() {
	log.Debug().Msg("systray ready")
	systray.SetTooltip("Herdr Fleet Board")

	// Initial display (platform-specific: colored text / icon / plain text)
	updateTray(0, 0, 0, 0, 0)

	// Left-click toggles window
	systray.SetOnTapped(func() {
		if t.hidden.Load() {
			t.Show()
		} else {
			t.Hide()
		}
	})

	// Menu: toggle
	mToggle := systray.AddMenuItem("Show/Hide Fleet Board", "Toggle window visibility")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit herdr-panel")

	go func() {
		for range mToggle.ClickedCh {
			if t.hidden.Load() {
				t.Show()
			} else {
				t.Hide()
			}
		}
	}()
	go func() {
		for range mQuit.ClickedCh {
			log.Debug().Msg("quit requested from tray menu")
			systray.Quit()
		}
	}()

	go t.updateLoop()
}

func (t *Tray) onExit() {
	log.Debug().Msg("systray exited")
}

func (t *Tray) updateLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		snap := t.store.Snapshot()
		if snap == nil {
			continue
		}
		s := snap.Stats
		updateTray(s.Blocked, s.Working, s.Done, s.Idle, s.Unknown)
	}
}
