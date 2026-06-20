// IconRenderer: generates base64 SVG data for each key type
// All SVGs use 200×200 viewBox (K14 uses 400×200 for wide key)

const STATUS_COLORS = {
	done: "#27AE60",
	idle: "#7F8C8D",
	working: "#F39C12",
	blocked: "#E74C3C",
	unknown: "#95A5A6",
};

// Text status indicators (avoid emoji - deck firmware may not render them)
const STATUS_TEXT = {
	done: "D",
	idle: "I",
	working: "W",
	blocked: "B",
	unknown: "?",
};

export class IconRenderer {
	constructor() {
		this.agentIcons = getAgentIconPaths();
	}

	// ─── Agent key (K1-K10) ──────────────────────────────────────
	renderAgentKey(data) {
		const bgColor = STATUS_COLORS[data.status] || "#555";
		const iconPath = this.agentIcons[data.agentType] || this.agentIcons.unknown;
		const statusText = STATUS_TEXT[data.status] || "?";
		const borderColor = data.focused ? "#FFFFFF" : "transparent";
		const borderWidth = data.focused ? "3" : "0";
		const alias = this.escapeXml(data.alias || "");

		const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
      <defs>
        <clipPath id="c"><rect width="200" height="200" rx="8"/></clipPath>
      </defs>
      <rect width="200" height="200" rx="8" fill="${bgColor}" opacity="0.88"/>
      <rect width="200" height="200" rx="8" fill="#000" opacity="0.12"/>
      <rect x="2" y="2" width="196" height="196" rx="8"
            fill="none" stroke="${borderColor}" stroke-width="${borderWidth}"
            opacity="${data.focused ? 1 : 0}"/>
      <g transform="translate(45, 25) scale(0.55)" fill="white" opacity="0.92">
        ${iconPath}
      </g>
      <text x="100" y="155" text-anchor="middle" fill="white"
            font-family="sans-serif" font-size="20" font-weight="500">${alias}</text>
      <text x="175" y="185" text-anchor="end" fill="white" font-size="32" font-weight="bold">${statusText}</text>
    </svg>`;

		return this.toDataUri(svg);
	}

	// ─── Navigation keys (K11, K13) ──────────────────────────────
	renderNavKey(type, label, enabled) {
		const arrow = type === "navPrev" ? "◀" : "▶";
		const opacity = enabled ? "1" : "0.3";
		const escaped = this.escapeXml(label || "");

		const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
      <rect width="200" height="200" rx="8" fill="#3a3a3a"/>
      <text x="100" y="70" text-anchor="middle" fill="white"
            font-size="32" opacity="${opacity}">${arrow}</text>
      <text x="100" y="135" text-anchor="middle" fill="#aaa"
            font-family="sans-serif" font-size="18" opacity="${opacity}">${escaped}</text>
    </svg>`;

		return this.toDataUri(svg);
	}

	// ─── Current page info (K12) ─────────────────────────────────
	renderCurrentKey(data) {
		let svg;
		if (data.singleWs) {
			const label = this.escapeXml(data.label || "");
			const sub = this.escapeXml(data.sublabel || "");
			const page = this.escapeXml(data.pageLabel || "");

			svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
        <rect width="200" height="200" rx="8" fill="#2a2a2a"/>
        <text x="100" y="90" text-anchor="middle" fill="white"
              font-family="sans-serif" font-size="22" font-weight="600">${label}</text>
        ${
					sub
						? `<text x="100" y="130" text-anchor="middle" fill="#888"
              font-family="sans-serif" font-size="16">${sub}</text>`
						: ""
				}
        <text x="100" y="170" text-anchor="middle" fill="#666"
              font-family="sans-serif" font-size="14">${page}</text>
      </svg>`;
		} else {
			const r1 = data.rows?.[0];
			const r2 = data.rows?.[1];
			const page = this.escapeXml(data.pageLabel || "");

			svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
        <rect width="200" height="200" rx="8" fill="#2a2a2a"/>
        ${
					r1
						? `<text x="100" y="60" text-anchor="middle" fill="white"
              font-family="sans-serif" font-size="19">${this.escapeXml(r1.abbr)}:${this.escapeXml(r1.label)}</text>`
						: ""
				}
        ${
					r2
						? `<text x="100" y="100" text-anchor="middle" fill="#aaa"
              font-family="sans-serif" font-size="17">· ${this.escapeXml(r2.abbr)}:${this.escapeXml(r2.label)}</text>`
						: ""
				}
        <text x="100" y="165" text-anchor="middle" fill="#666"
              font-family="sans-serif" font-size="14">${page}</text>
      </svg>`;
		}

		return this.toDataUri(svg);
	}

	// ─── Stats bar (K14 - wide) ──────────────────────────────────
	renderStatsKey(stats) {
		const items = [
			{ emoji: "✅", count: stats.done, color: "#27AE60" },
			{ emoji: "⏸", count: stats.idle, color: "#7F8C8D" },
			{ emoji: "⏳", count: stats.working, color: "#F39C12" },
			{ emoji: "❌", count: stats.blocked, color: "#E74C3C" },
			{ emoji: "❓", count: stats.unknown, color: "#95A5A6" },
		];

		// Long bar: 400×200, 5 evenly spaced items
		let inner = "";
		const spacing = 400 / items.length;
		items.forEach((item, i) => {
			const cx = spacing * i + spacing / 2;
			inner += `
        <text x="${cx}" y="115" text-anchor="middle" font-size="38">${item.emoji}</text>
        <text x="${cx}" y="165" text-anchor="middle" fill="white"
              font-family="sans-serif" font-size="30" font-weight="700">${item.count}</text>`;
		});

		const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 200">
      <rect width="400" height="200" rx="8" fill="#1a1a1a" opacity="0.9"/>
      ${inner}
    </svg>`;

		return this.toDataUri(svg);
	}

	// ─── Empty key ───────────────────────────────────────────────
	renderEmptyKey() {
		const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
      <rect width="200" height="200" rx="8" fill="#2a2a2a" opacity="0.25"/>
    </svg>`;
		return this.toDataUri(svg);
	}

	// ─── Helpers ─────────────────────────────────────────────────
	toDataUri(svg) {
		return "data:image/svg+xml;base64," + Buffer.from(svg).toString("base64");
	}

	escapeXml(str) {
		if (!str) return "";
		return str
			.replace(/&/g, "&amp;")
			.replace(/</g, "&lt;")
			.replace(/>/g, "&gt;")
			.replace(/"/g, "&quot;")
			.replace(/'/g, "&apos;");
	}
}

// ─── Agent SVG paths (inline, file-free) ───────────────────────
function getAgentIconPaths() {
	return {
		pi: `<path d="M100 30 L100 170 M60 170 L140 170" stroke="white" stroke-width="12" fill="none" stroke-linecap="round"/>`,
		claude: `<ellipse cx="100" cy="70" rx="35" ry="28" fill="none" stroke="white" stroke-width="9"/>
             <path d="M65 85 Q85 145 100 165 Q115 145 135 85" fill="none" stroke="white" stroke-width="9" stroke-linecap="round"/>
             <path d="M50 50 Q75 15 100 35 Q125 15 150 50" fill="none" stroke="white" stroke-width="7" stroke-linecap="round"/>`,
		cursor: `<path d="M55 35 L55 170 L125 120 L170 165" fill="none" stroke="white" stroke-width="11" stroke-linejoin="round" stroke-linecap="round"/>
             <circle cx="125" cy="120" r="8" fill="white"/>`,
		cline: `<path d="M45 145 L90 55 L135 145 L180 55" fill="none" stroke="white" stroke-width="10" stroke-linejoin="round" stroke-linecap="round"/>`,
		codex: `<rect x="45" y="50" width="45" height="95" rx="6" fill="none" stroke="white" stroke-width="8"/>
            <path d="M115 75 L135 95 L115 115" fill="none" stroke="white" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M140 75 L160 95 L140 115" fill="none" stroke="white" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>`,
		gemini: `<path d="M100 25 Q135 80 180 100 Q135 120 100 175 Q65 120 20 100 Q65 80 100 25Z" fill="none" stroke="white" stroke-width="9" stroke-linejoin="round"/>`,
		copilot: `<path d="M55 100 Q55 40 100 30 Q145 40 145 100 Q145 160 100 170 Q55 160 55 100Z" fill="none" stroke="white" stroke-width="9"/>
              <path d="M75 80 L95 102 L125 80" fill="none" stroke="white" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"/>`,
		devin: `<path d="M55 80 Q100 30 145 80" fill="none" stroke="white" stroke-width="10" stroke-linecap="round"/>
            <path d="M75 105 L75 170 M125 105 L125 170" fill="none" stroke="white" stroke-width="10" stroke-linecap="round"/>
            <rect x="55" y="145" width="90" height="8" rx="4" fill="white"/>`,
		grok: `<ellipse cx="100" cy="85" rx="40" ry="28" fill="none" stroke="white" stroke-width="9"/>
           <circle cx="78" cy="80" r="7" fill="white"/>
           <circle cx="122" cy="80" r="7" fill="white"/>
           <path d="M72 105 Q100 130 128 105" fill="none" stroke="white" stroke-width="6" stroke-linecap="round"/>`,
		kimi: `<path d="M100 25 Q40 85 100 160" fill="none" stroke="white" stroke-width="11" stroke-linecap="round"/>
           <path d="M100 25 Q160 85 100 160" fill="none" stroke="white" stroke-width="11" stroke-linecap="round"/>
           <circle cx="100" cy="35" r="9" fill="white"/>`,
		kilo: `<text x="100" y="140" text-anchor="middle" fill="white" font-size="110" font-weight="bold">K</text>`,
		kiro: `<path d="M55 25 L145 100 L75 100 L145 175" fill="none" stroke="white" stroke-width="12" stroke-linejoin="round" stroke-linecap="round"/>`,
		opencode: `<text x="100" y="145" text-anchor="middle" fill="white" font-size="110" font-weight="bold">{</text>`,
		qodercli: `<text x="100" y="145" text-anchor="middle" fill="white" font-size="90" font-weight="bold">&gt;_</text>`,
		amp: `<circle cx="100" cy="100" r="55" fill="none" stroke="white" stroke-width="9"/>
          <path d="M75 60 L125 100 L75 100 L125 140" fill="none" stroke="white" stroke-width="10" stroke-linejoin="round" stroke-linecap="round"/>`,
		antigravity: `<path d="M100 170 L100 55 M65 85 L100 55 L135 85" fill="none" stroke="white" stroke-width="11" stroke-linecap="round" stroke-linejoin="round"/>`,
		droid: `<rect x="60" y="45" width="80" height="75" rx="16" fill="none" stroke="white" stroke-width="9"/>
            <circle cx="78" cy="85" r="6" fill="white"/>
            <circle cx="122" cy="85" r="6" fill="white"/>
            <rect x="82" y="125" width="36" height="30" rx="5" fill="none" stroke="white" stroke-width="7"/>
            <line x1="70" y1="118" x2="65" y2="152" stroke="white" stroke-width="7" stroke-linecap="round"/>
            <line x1="130" y1="118" x2="135" y2="152" stroke="white" stroke-width="7" stroke-linecap="round"/>`,
		hermes: `<path d="M45 50 L100 110 L155 50" fill="none" stroke="white" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/>
             <rect x="40" y="50" width="120" height="90" rx="9" fill="none" stroke="white" stroke-width="9"/>
             <path d="M45 105 L90 132 L100 140 L110 132 L155 105" fill="none" stroke="white" stroke-width="7" stroke-linejoin="round"/>`,
		unknown: `<circle cx="100" cy="75" r="32" fill="none" stroke="white" stroke-width="9"/>
              <text x="100" y="155" text-anchor="middle" fill="white" font-size="55">?</text>`,
	};
}
