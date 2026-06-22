package main

import (
	"log"
	"time"
)

// runAllKeys sends SVGs to all 14 keys with various statuses.
func runAllKeys() {
	agentKeys := []string{"0_0", "1_0", "2_0", "3_0", "4_0", "0_1", "1_1", "2_1", "3_1", "4_1"}
	navKeys := []string{"0_2", "1_2", "2_2"}
	wideKey := "3_2"

	c := connect()
	defer c.close()

	statuses := []string{"working", "done", "idle", "blocked", "unknown",
		"working", "done", "idle", "blocked", "unknown"}
	for i, key := range agentKeys {
		svg := testAgentSVG(statuses[i], statuses[i], 0)
		c.sendState(key, toDataURI(svg))
		time.Sleep(50 * time.Millisecond)
	}

	c.sendState(navKeys[0], toDataURI(testNavSVG("ALL", "ACT")))
	time.Sleep(50 * time.Millisecond)
	c.sendState(navKeys[1], toDataURI(testNavSVG("MACH", "LCL>DEV")))
	time.Sleep(50 * time.Millisecond)
	c.sendState(navKeys[2], toDataURI(testNavSVG("SPACE", "main")))
	time.Sleep(50 * time.Millisecond)
	c.sendState(wideKey, toDataURI(testStatsSVG()))

	log.Printf("all 14 keys sent")
	time.Sleep(500 * time.Millisecond)
}
