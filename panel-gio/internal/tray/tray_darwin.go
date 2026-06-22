//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// Forward declaration — the actual variable lives in fyne.io/systray's
// systray_darwin.m as "SystrayAppDelegate *owner".  CGo links all C/ObjC
// symbols into the final binary so the linker resolves it.
extern id owner;

void darwinSetTrayTitle(int blocked, int working, int done) {
	// Must dispatch to main thread because this is called from a goroutine.
	dispatch_async(dispatch_get_main_queue(), ^{
		NSStatusItem *item = [owner valueForKey:@"statusItem"];
		if (!item || !item.button) return;

		NSButton *btn = item.button;

		// Remove any icon SetIcon() may have placed
		btn.image = nil;
		btn.title = @"";

		// Palette matching Fleet Board
		// Blocked  = #E74C3C  (red)
		// Working  = #F39C12  (gold)
		// Done     = #27AE60  (green)
		// Inactive = #7F8C8D  (gray)
		NSColor *red   = [NSColor colorWithSRGBRed:0.906 green:0.298 blue:0.235 alpha:1.0];
		NSColor *gold  = [NSColor colorWithSRGBRed:0.953 green:0.612 blue:0.071 alpha:1.0];
		NSColor *green = [NSColor colorWithSRGBRed:0.153 green:0.682 blue:0.376 alpha:1.0];
		NSColor *gray  = [NSColor colorWithSRGBRed:0.498 green:0.549 blue:0.553 alpha:1.0];

		NSFont *font = [NSFont monospacedSystemFontOfSize:12 weight:NSFontWeightRegular];

		// Helper: one coloured segment
		NSString* (^seg)(NSString*, int, NSColor*) = ^NSString*(NSString *label, int n, NSColor *active) {
			// Not used — we build NSAttributedString directly below
			(void)label; (void)n; (void)active;
			return @"";
		};
		(void)seg;

		NSMutableAttributedString *attr = [[NSMutableAttributedString alloc] init];

		// "B" + count
		NSColor *bCol = (blocked > 0) ? red : gray;
		[attr appendAttributedString:[[NSAttributedString alloc] initWithString:@"B" attributes:@{
			NSFontAttributeName: font, NSForegroundColorAttributeName: bCol
		}]];
		[attr appendAttributedString:[[NSAttributedString alloc] initWithString:[NSString stringWithFormat:@"%d", blocked] attributes:@{
			NSFontAttributeName: font, NSForegroundColorAttributeName: bCol
		}]];
		[attr appendAttributedString:[[NSAttributedString alloc] initWithString:@" " attributes:@{
			NSFontAttributeName: font, NSForegroundColorAttributeName: gray
		}]];

		// "W" + count
		NSColor *wCol = (working > 0) ? gold : gray;
		[attr appendAttributedString:[[NSAttributedString alloc] initWithString:@"W" attributes:@{
			NSFontAttributeName: font, NSForegroundColorAttributeName: wCol
		}]];
		[attr appendAttributedString:[[NSAttributedString alloc] initWithString:[NSString stringWithFormat:@"%d", working] attributes:@{
			NSFontAttributeName: font, NSForegroundColorAttributeName: wCol
		}]];
		[attr appendAttributedString:[[NSAttributedString alloc] initWithString:@" " attributes:@{
			NSFontAttributeName: font, NSForegroundColorAttributeName: gray
		}]];

		// "D" + count
		NSColor *dCol = (done > 0) ? green : gray;
		[attr appendAttributedString:[[NSAttributedString alloc] initWithString:@"D" attributes:@{
			NSFontAttributeName: font, NSForegroundColorAttributeName: dCol
		}]];
		[attr appendAttributedString:[[NSAttributedString alloc] initWithString:[NSString stringWithFormat:@"%d", done] attributes:@{
			NSFontAttributeName: font, NSForegroundColorAttributeName: dCol
		}]];

		btn.attributedTitle = attr;
		[btn sizeToFit];
	});
}

void hidWindow(uintptr_t viewPtr) {
	NSView *view = (__bridge NSView *)(void*)viewPtr;
	[[view window] orderOut:nil];
}

void showWindowDarwin(uintptr_t viewPtr) {
	NSView *view = (__bridge NSView *)(void*)viewPtr;
	NSWindow *win = [view window];
	[win makeKeyAndOrderFront:nil];
	[NSApp activateIgnoringOtherApps:YES];
}
*/
import "C"

func updateTray(blocked, working, done, idle, unknown int) {
	C.darwinSetTrayTitle(C.int(blocked), C.int(working), C.int(done))
}

func nativeHide(view uintptr) {
	if view == 0 {
		return
	}
	C.hidWindow(C.uintptr_t(view))
}

func nativeShow(view uintptr) {
	if view == 0 {
		return
	}
	C.showWindowDarwin(C.uintptr_t(view))
}
