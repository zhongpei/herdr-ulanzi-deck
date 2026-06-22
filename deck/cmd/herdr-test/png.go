package main

import (
	"log"
	"time"
)

// runSendPNG sends a static PNG to one key (baseline test for protocol acceptance).
func runSendPNG(key string) {
	c := connect()
	defer c.close()

	// Minimal valid 1×1 red PNG
	pngB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	c.sendState(key, "data:image/png;base64,"+pngB64)
	log.Printf("sent PNG to key=%s", key)
	time.Sleep(1 * time.Second)
}
