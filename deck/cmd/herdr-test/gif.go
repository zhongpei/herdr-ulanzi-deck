package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"log"
	"os"
	"os/signal"
	"time"
)

// runSendGIF generates an animated GIF and sends it to one key.
// Uses SDK setGifDataIcon format: type:3 + gifdata (raw base64).
func runSendGIF(key string) {
	c := connect()
	defer c.close()

	gifRaw := generateTestGIF(196, 196, 8, 12)
	gifB64 := base64.StdEncoding.EncodeToString(gifRaw)
	c.sendGifData(key, gifB64)
	log.Printf("sent GIF type:3 (%d bytes) to key=%s", len(gifRaw), key)
	time.Sleep(3 * time.Second)
}

// runAnimateGIF repeatedly sends GIFs to one key.
func runAnimateGIF(key string, fps int) {
	delay := time.Second / time.Duration(fps)
	log.Printf("sending GIFs to key=%s at %d fps (%dms)", key, fps, delay/time.Millisecond)

	c := connect()
	defer c.close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-sig:
			log.Printf("stopped after %d frames", frame)
			return
		case <-ticker.C:
			gifRaw := generateTestGIF(196, 196, 4, 12)
			c.sendGifData(key, base64.StdEncoding.EncodeToString(gifRaw))
			frame++
			if frame%20 == 0 {
				log.Printf("sent %d GIFs", frame)
			}
		}
	}
}

// generateTestGIF creates an animated GIF with pulsing colors.
func generateTestGIF(width, height, nframes, delayCs int) []byte {
	out := &gif.GIF{}
	for i := 0; i < nframes; i++ {
		img := image.NewPaletted(image.Rect(0, 0, width, height), palette.Plan9)
		r := uint8((255 * i / nframes) % 256)
		g := uint8((255 * (i + nframes/3) / nframes) % 256)
		b := uint8((255 * (i + 2*nframes/3) / nframes) % 256)
		draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{r, g, b, 255}), image.Point{}, draw.Src)

		// White circle that changes size per frame
		radius := 20 + i*8
		for y := -radius; y <= radius; y++ {
			for x := -radius; x <= radius; x++ {
				if x*x+y*y <= radius*radius {
					cx, cy := width/2, height/2
					if cx+x >= 0 && cx+x < width && cy+y >= 0 && cy+y < height {
						img.SetColorIndex(cx+x, cy+y, 0) // index 0 ≈ white in Plan9
					}
				}
			}
		}

		out.Image = append(out.Image, img)
		out.Delay = append(out.Delay, delayCs)
	}

	out.Disposal = make([]byte, nframes)
	for i := range out.Disposal {
		out.Disposal[i] = gif.DisposalBackground
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, out); err != nil {
		log.Fatalf("gif encode: %v", err)
	}
	return buf.Bytes()
}
