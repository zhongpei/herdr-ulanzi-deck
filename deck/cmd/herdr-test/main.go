// herdr-test — CLI tool to test SVG/PNG/animation delivery to Ulanzi D200X.
//
// Uses the exact UUIDs from the installed herdr-deck plugin.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

const (
	PluginUUID  = "com.ulanzi.herdr.agentview"
	ActionUUID  = "com.ulanzi.herdr.agentview.monitor"
	DefaultAddr = "127.0.0.1"
	DefaultPort = 3906
)

var statusColors = map[string]string{
	"done":    "#27AE60",
	"idle":    "#7F8C8D",
	"working": "#F39C12",
	"blocked": "#E74C3C",
	"unknown": "#95A5A6",
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `herdr-test — test SVG/PNG/animation delivery to Ulanzi D200X

Usage:
  herdr-test <command> [args...]

Commands:
  svg <key>              Send a static test SVG to one key
  png <key>              Send a static test PNG to one key
  gif <key>              Send a test animated GIF to one key
  gif-loop <key> [fps]   Repeatedly send animated GIFs (default 4 fps)
  anim <key> [fps]       Animate key with blink+pulse (default 6 fps)
  all                    Test all 14 keys with SVGs
  anim-all [fps]         Animate all agent positions (0_0-4_1) in sync
  text-cn <key>          Cycle Chinese text patterns on one key

Key format: col_row (e.g. 0_0, 1_2, 3_2)
            14 keys visible: 0_0..4_1 (col 0-4, row 0-1)
            plus wide key 3_2

Uses installed plugin:
  plugin=%s  action=%s
`, PluginUUID, ActionUUID)
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	args := flag.Args()[1:]

	switch cmd {
	case "svg":
		if len(args) < 1 {
			log.Fatal("usage: herdr-test svg <key>")
		}
		runSendSVG(args[0])

	case "png":
		if len(args) < 1 {
			log.Fatal("usage: herdr-test png <key>")
		}
		runSendPNG(args[0])

	case "gif":
		key := "0_0"
		if len(args) >= 1 {
			key = args[0]
		}
		runSendGIF(key)

	case "gif-loop":
		key := "0_0"
		fps := 4
		if len(args) >= 1 {
			key = args[0]
		}
		if len(args) >= 2 {
			fmt.Sscanf(args[1], "%d", &fps)
		}
		runAnimateGIF(key, fps)

	case "anim":
		key := "0_0"
		fps := 6
		if len(args) >= 1 {
			key = args[0]
		}
		if len(args) >= 2 {
			fmt.Sscanf(args[1], "%d", &fps)
		}
		runAnimateKey(key, fps)

	case "text-cn":
		key := "0_0"
		if len(args) >= 1 {
			key = args[0]
		}
		runTextCN(key)

	case "all":
		runAllKeys()

	case "anim-all":
		fps := 6
		if len(args) >= 1 {
			fmt.Sscanf(args[0], "%d", &fps)
		}
		runAnimateAll(fps)

	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}
