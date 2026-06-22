package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type textTestCase struct {
	Label  string `json:"label"`
	Status string `json:"status"`
	Text   string `json:"text"`
}

type textTestData struct {
	Tests []textTestCase `json:"text_tests"`
}

// runTextCN sends SVGs with Chinese/emoji text to test rendering.
// Test cases are loaded from testdata.json in the same directory as the binary.
func runTextCN(key string) {
	c := connect()
	defer c.close()

	tests := loadTextTests()
	if len(tests) == 0 {
		log.Fatal("no text tests found in testdata.json")
	}

	for i, t := range tests {
		svg := testTextSVG(t.Status, t.Label, t.Text)
		c.sendState(key, toDataURI(svg))
		log.Printf("[%d/%d] label=%q status=%s text=%s", i+1, len(tests), t.Label, t.Status, t.Text)
		time.Sleep(3 * time.Second)
	}

	log.Printf("text test complete — cycled %d patterns", len(tests))
}

// loadTextTests reads test cases from testdata.json.
// Searches: binary directory → working directory.
func loadTextTests() []textTestCase {
	paths := []string{
		filepath.Join(filepath.Dir(os.Args[0]), "testdata.json"),
		"testdata.json",
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var td textTestData
		if err := json.Unmarshal(data, &td); err != nil {
			log.Printf("warning: %s: %v", p, err)
			continue
		}
		log.Printf("loaded %d text tests from %s", len(td.Tests), p)
		return td.Tests
	}
	log.Printf("warning: testdata.json not found (checked: %v)", paths)
	return nil
}

// testTextSVG generates an SVG with customizable text content.
func testTextSVG(status, label, content string) string {
	bgColor := statusColors[status]
	if bgColor == "" {
		bgColor = "#6B7280"
	}

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="%[1]s"/>
  <rect width="200" height="200" rx="8" fill="#000" opacity="0.10"/>
  <rect x="0" y="0" width="100" height="48" fill="#1a1a2e"/>
  <rect x="100" y="0" width="100" height="48" fill="#2d2d44"/>
  <rect x="0" y="48" width="200" height="1" fill="#fff" opacity="0.20"/>
  <text x="50" y="32" text-anchor="middle" fill="white" font-family="sans-serif" font-size="20" font-weight="900">%[2]s</text>
  <text x="150" y="32" text-anchor="middle" fill="white" font-family="sans-serif" font-size="20" font-weight="900">中文</text>
  <text x="100" y="90" text-anchor="middle" fill="white" font-family="sans-serif" font-size="28" font-weight="700">%[3]s</text>
  <text x="100" y="135" text-anchor="middle" fill="white" font-family="sans-serif" font-size="18" font-weight="400">%[4]s</text>
  <text x="100" y="180" text-anchor="middle" fill="#ccc" font-family="sans-serif" font-size="16">herdr-test</text>
</svg>`, bgColor, label, label, content)
}
