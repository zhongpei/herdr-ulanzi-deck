package render

// AgentIcons returns the SVG path for each agent icon.
// All are white single-color paths for 200×200 viewBox.
func AgentIcons() map[string]string {
	return map[string]string{
		"pi": `<path d="M100 30 L100 170 M60 170 L140 170" stroke="white" stroke-width="14" fill="none" stroke-linecap="round"/>`,
		"claude": `<path d="M65 40 Q100 20 135 40 L100 160Z" fill="none" stroke="white" stroke-width="12" stroke-linejoin="round"/>
             <circle cx="100" cy="60" r="8" fill="white"/>
             <path d="M75 95 Q100 145 125 95" fill="none" stroke="white" stroke-width="10" stroke-linecap="round"/>`,
		"cursor": `<path d="M50 30 L50 170 L120 120 L170 170" fill="none" stroke="white" stroke-width="14" stroke-linejoin="round" stroke-linecap="round"/>
             <circle cx="120" cy="120" r="12" fill="white"/>`,
		"cline": `<path d="M40 145 L90 40 L140 145 L190 40" fill="none" stroke="white" stroke-width="14" stroke-linejoin="round" stroke-linecap="round"/>`,
		"codex": `<rect x="40" y="40" width="55" height="80" rx="6" fill="none" stroke="white" stroke-width="12"/>
            <path d="M120 60 L150 80 L120 100" fill="none" stroke="white" stroke-width="12" stroke-linecap="round" stroke-linejoin="round"/>`,
		"gemini": `<path d="M100 20 Q140 80 180 100 Q140 120 100 180 Q60 120 20 100 Q60 80 100 20Z" fill="none" stroke="white" stroke-width="12" stroke-linejoin="round"/>`,
		"copilot": `<path d="M50 100 Q50 30 100 20 Q150 30 150 100 Q150 170 100 180 Q50 170 50 100Z" fill="none" stroke="white" stroke-width="12"/>
                <path d="M70 80 L95 105 L130 80" fill="none" stroke="white" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>`,
		"devin": `<path d="M50 80 Q100 20 150 80" fill="none" stroke="white" stroke-width="14" stroke-linecap="round"/>
              <path d="M70 110 L70 170 M130 110 L130 170" fill="none" stroke="white" stroke-width="14" stroke-linecap="round"/>
              <rect x="45" y="145" width="110" height="10" rx="5" fill="white"/>`,
		"grok": `<circle cx="100" cy="80" r="45" fill="none" stroke="white" stroke-width="12"/>
               <circle cx="80" cy="75" r="8" fill="white"/>
               <circle cx="120" cy="75" r="8" fill="white"/>
               <path d="M70 105 Q100 135 130 105" fill="none" stroke="white" stroke-width="8" stroke-linecap="round"/>`,
		"kimi": `<path d="M100 20 Q30 80 100 160" fill="none" stroke="white" stroke-width="14" stroke-linecap="round"/>
               <path d="M100 20 Q170 80 100 160" fill="none" stroke="white" stroke-width="14" stroke-linecap="round"/>
               <circle cx="100" cy="30" r="10" fill="white"/>`,
		"kilo":     `<text x="100" y="145" text-anchor="middle" fill="white" font-size="130" font-weight="bold">K</text>`,
		"kiro":     `<path d="M50 20 L150 100 L70 100 L150 180" fill="none" stroke="white" stroke-width="14" stroke-linejoin="round" stroke-linecap="round"/>`,
		"opencode": `<text x="100" y="155" text-anchor="middle" fill="white" font-size="140" font-weight="bold">{</text>`,
		"qodercli": `<text x="100" y="155" text-anchor="middle" fill="white" font-size="110" font-weight="bold">&gt;_</text>`,
		"amp": `<circle cx="100" cy="100" r="65" fill="none" stroke="white" stroke-width="12"/>
               <path d="M65 55 L135 100 L65 100 L135 145" fill="none" stroke="white" stroke-width="14" stroke-linejoin="round" stroke-linecap="round"/>`,
		"antigravity": `<path d="M100 170 L100 50 M60 85 L100 50 L140 85" fill="none" stroke="white" stroke-width="14" stroke-linecap="round" stroke-linejoin="round"/>`,
		"droid": `<rect x="55" y="35" width="90" height="80" rx="18" fill="none" stroke="white" stroke-width="14"/>
                 <circle cx="78" cy="85" r="8" fill="white"/>
                 <circle cx="122" cy="85" r="8" fill="white"/>
                 <rect x="75" y="125" width="50" height="40" rx="6" fill="none" stroke="white" stroke-width="12"/>
                 <line x1="60" y1="118" x2="55" y2="160" stroke="white" stroke-width="10" stroke-linecap="round"/>
                 <line x1="140" y1="118" x2="145" y2="160" stroke="white" stroke-width="10" stroke-linecap="round"/>`,
		"hermes": `<path d="M40 40 L100 105 L160 40" fill="none" stroke="white" stroke-width="12" stroke-linecap="round" stroke-linejoin="round"/>
                 <rect x="35" y="40" width="130" height="100" rx="10" fill="none" stroke="white" stroke-width="12"/>
                 <path d="M40 105 L90 135 L100 145 L110 135 L160 105" fill="none" stroke="white" stroke-width="10" stroke-linejoin="round"/>`,
		"unknown": `<circle cx="100" cy="70" r="40" fill="none" stroke="white" stroke-width="14"/>
                  <text x="100" y="165" text-anchor="middle" fill="white" font-size="70">?</text>`,
	}
}

// StatusIcons returns hand-drawn SVG inner content for each status indicator.
// Positioned in the 200x200 key viewBox at bottom-right (visual center ~x=180, y=180),
// drawn in a ~24x24 area. Pure SVG primitives (no Unicode text) so the icons
// render reliably on the D200X without depending on system font glyph coverage.
//
// Style: white stroke, no fill (filled elements use white fill explicitly).
// Icons inlined directly by the caller — no <text> wrapper.
func StatusIcons() map[string]string {
	return map[string]string{
		// ✓ checkmark — two segments forming a check
		"done": `<polyline points="168,180 176,188 192,170" fill="none" stroke="white" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>`,
		// ‖ pause — two vertical bars
		"idle": `<line x1="174" y1="170" x2="174" y2="194" stroke="white" stroke-width="4" stroke-linecap="round"/>
<line x1="186" y1="170" x2="186" y2="194" stroke="white" stroke-width="4" stroke-linecap="round"/>`,
		// ↻ working — circle with motion notch
		"working": `<circle cx="180" cy="180" r="8" fill="none" stroke="white" stroke-width="3"/>
<line x1="180" y1="170" x2="180" y2="174" stroke="white" stroke-width="3" stroke-linecap="round"/>`,
		// ⚠ blocked — warning triangle + exclamation
		"blocked": `<polyline points="180,168 192,190 168,190 180,168" fill="none" stroke="white" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
<line x1="180" y1="178" x2="180" y2="184" stroke="white" stroke-width="3" stroke-linecap="round"/>
<circle cx="180" cy="187.5" r="1.5" fill="white"/>`,
		// ? unknown — circle + ASCII "?" (always renders in basic Latin font)
		"unknown": `<circle cx="180" cy="180" r="11" fill="none" stroke="white" stroke-width="3"/>
<text x="180" y="186" text-anchor="middle" fill="white" font-size="16" font-weight="800">?</text>`,
	}
}
