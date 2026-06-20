// IconRenderer: generates base64 SVG data for each key type
// All SVGs use 200×200 viewBox (K14 uses 400×200 for wide key)
// 196×196 D200X key resolution

// Agent brand colors (as RGB hex for SVG fill)
const AGENT_COLORS = {
	pi: "#7C3AED", // purple
	claude: "#D97757", // warm brown
	cursor: "#00B884", // teal
	cline: "#2563EB", // blue
	codex: "#1E293B", // dark slate
	gemini: "#4285F4", // google blue
	copilot: "#8957E5", // purple
	devin: "#FF6B35", // orange
	grok: "#1DA1F2", // twitter blue
	kimi: "#FF6B6B", // coral
	kilo: "#10B981", // emerald
	kiro: "#F59E0B", // amber
	opencode: "#6366F1", // indigo
	qodercli: "#8B5CF6", // violet
	amp: "#EC4899", // pink
	antigravity: "#06B6D4", // cyan
	droid: "#84CC16", // lime
	hermes: "#F97316", // orange
	unknown: "#6B7280", // gray
};

// Status indicator colors
const STATUS_COLORS = {
	done: "#27AE60",
	idle: "#7F8C8D",
	working: "#F39C12",
	blocked: "#E74C3C",
	unknown: "#95A5A6",
};

export class IconRenderer {
	constructor() {
		this.agentIcons = getAgentIconPaths();
	}

	// ─── Agent key (K1-K10) ──────────────────────────────────────
	// Layout (200x200 canvas):
	//   ┌──────────────────────┐
	//   │ ▓▓▓ PI ▓▓▓  ▓▓ LCL ▓│  ← 48px top bar
	//   │──────────────────────│  ← 1px white separator
	//   │                      │
	//   │       review         │  ← alias (36px BOLD white)
	//   │                      │
	//   │          W           │  ← status letter (20px)
	//   │       main-proj      │  ← workspace name (14px)
	//   └──────────────────────┘
	//   Remaining bg = status color + black 0.15 overlay
	renderAgentKey(data) {
		const agentColor = AGENT_COLORS[data.agentType] || AGENT_COLORS.unknown;
		const statusColor = STATUS_COLORS[data.status] || STATUS_COLORS.unknown;
		const statusLetter = (data.status || "?")[0].toUpperCase();
		const alias = this.escapeXml(data.alias || "");
		const agentName = this.escapeXml(data.agentType || "");
		const machineAbbr = this.escapeXml(data.connAbbr || "");
		const machineColor = data.connAbbrColor || "#888888";
		const wsLabel = this.escapeXml(data.wsLabel || "");
		const borderColor = data.focused ? "#FFFFFF" : "transparent";
		const borderWidth = data.focused ? "3" : "0";

		// Truncation
		const displayAlias = alias.length > 9 ? alias.slice(0, 8) + "…" : alias;
		const displayWs =
			wsLabel.length > 12 ? wsLabel.slice(0, 11) + "…" : wsLabel;

		const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="${statusColor}"/>
  <rect width="200" height="200" rx="8" fill="#000" opacity="0.15"/>
  <rect x="0" y="0" width="100" height="48" fill="${agentColor}"/>
  <rect x="100" y="0" width="100" height="48" fill="${machineColor}"/>
  <rect x="0" y="48" width="200" height="1" fill="#fff" opacity="0.25"/>
  <rect x="2" y="2" width="196" height="196" rx="8"
        fill="none" stroke="${borderColor}" stroke-width="${borderWidth}"
        opacity="${data.focused ? 1 : 0}"/>
  <text x="50" y="32" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="24" font-weight="800">${agentName}</text>
  <text x="150" y="32" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="24" font-weight="800">${machineAbbr}</text>
  <text x="100" y="90" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="36" font-weight="700">${displayAlias}</text>
  <text x="100" y="125" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="20" font-weight="800">${statusLetter}</text>
  <text x="100" y="155" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="26" font-weight="700">${displayWs}</text>
</svg>`;

		return this.toDataUri(svg);
	}

	// ─── Navigation keys (K11, K13) ──────────────────────────────
	renderNavKey(type, label, enabled) {
		const arrow = type === "navPrev" ? "◀" : "▶";
		const opacity = enabled ? "1" : "0.3";
		const escaped = this.escapeXml(label || "");
		const truncated =
			escaped.length > 12 ? escaped.slice(0, 11) + "…" : escaped;

		const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="#3a3a3a"/>
  <text x="100" y="70" text-anchor="middle" fill="white"
        font-size="32" opacity="${opacity}">${arrow}</text>
  <text x="100" y="135" text-anchor="middle" fill="#aaa"
        font-family="sans-serif" font-size="18" opacity="${opacity}">${truncated}</text>
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
			const r1label = r1
				? `${this.escapeXml(r1.abbr)}:${this.escapeXml(r1.label)}`
				: "";
			const r2label = r2
				? `· ${this.escapeXml(r2.abbr)}:${this.escapeXml(r2.label)}`
				: "";

			svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <rect width="200" height="200" rx="8" fill="#2a2a2a"/>
  <text x="100" y="60" text-anchor="middle" fill="white"
        font-family="sans-serif" font-size="19">${r1label}</text>
  <text x="100" y="100" text-anchor="middle" fill="#aaa"
        font-family="sans-serif" font-size="17">${r2label}</text>
  <text x="100" y="165" text-anchor="middle" fill="#666"
        font-family="sans-serif" font-size="14">${page}</text>
</svg>`;
		}

		return this.toDataUri(svg);
	}

	// ─── Stats bar (K14 - wide) ──────────────────────────────────
	renderStatsKey(stats) {
		const items = [
			{ label: "D", count: stats.done, color: "#27AE60" },
			{ label: "I", count: stats.idle, color: "#7F8C8D" },
			{ label: "W", count: stats.working, color: "#F39C12" },
			{ label: "B", count: stats.blocked, color: "#E74C3C" },
			{ label: "?", count: stats.unknown, color: "#95A5A6" },
		];

		let inner = "";
		const spacing = 400 / items.length;
		items.forEach((item, i) => {
			const cx = spacing * i + spacing / 2;
			inner += `
  <text x="${cx}" y="115" text-anchor="middle" font-size="38">${item.label}</text>
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

// ─── Agent SVG paths (simplified line-art, white, for small rendering) ───
function getAgentIconPaths() {
	return {
		pi: `<path d="M100 30 L100 170 M60 170 L140 170" stroke="white" stroke-width="14" fill="none" stroke-linecap="round"/>`,
		claude: `<path d="M65 40 Q100 20 135 40 L100 160Z" fill="none" stroke="white" stroke-width="12" stroke-linejoin="round"/>
             <circle cx="100" cy="60" r="8" fill="white"/>
             <path d="M75 95 Q100 145 125 95" fill="none" stroke="white" stroke-width="10" stroke-linecap="round"/>`,
		cursor: `<path d="M50 30 L50 170 L120 120 L170 170" fill="none" stroke="white" stroke-width="14" stroke-linejoin="round" stroke-linecap="round"/>
             <circle cx="120" cy="120" r="12" fill="white"/>`,
		cline: `<path d="M40 145 L90 40 L140 145 L190 40" fill="none" stroke="white" stroke-width="14" stroke-linejoin="round" stroke-linecap="round"/>`,
		codex: `<rect x="40" y="40" width="55" height="80" rx="6" fill="none" stroke="white" stroke-width="12"/>
            <path d="M120 60 L150 80 L120 100" fill="none" stroke="white" stroke-width="12" stroke-linecap="round" stroke-linejoin="round"/>`,
		gemini: `<path d="M100 20 Q140 80 180 100 Q140 120 100 180 Q60 120 20 100 Q60 80 100 20Z" fill="none" stroke="white" stroke-width="12" stroke-linejoin="round"/>`,
		copilot: `<path d="M50 100 Q50 30 100 20 Q150 30 150 100 Q150 170 100 180 Q50 170 50 100Z" fill="none" stroke="white" stroke-width="12"/>
                <path d="M70 80 L95 105 L130 80" fill="none" stroke="white" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>`,
		devin: `<path d="M50 80 Q100 20 150 80" fill="none" stroke="white" stroke-width="14" stroke-linecap="round"/>
              <path d="M70 110 L70 170 M130 110 L130 170" fill="none" stroke="white" stroke-width="14" stroke-linecap="round"/>
              <rect x="45" y="145" width="110" height="10" rx="5" fill="white"/>`,
		grok: `<circle cx="100" cy="80" r="45" fill="none" stroke="white" stroke-width="12"/>
               <circle cx="80" cy="75" r="8" fill="white"/>
               <circle cx="120" cy="75" r="8" fill="white"/>
               <path d="M70 105 Q100 135 130 105" fill="none" stroke="white" stroke-width="8" stroke-linecap="round"/>`,
		kimi: `<path d="M100 20 Q30 80 100 160" fill="none" stroke="white" stroke-width="14" stroke-linecap="round"/>
               <path d="M100 20 Q170 80 100 160" fill="none" stroke="white" stroke-width="14" stroke-linecap="round"/>
               <circle cx="100" cy="30" r="10" fill="white"/>`,
		kilo: `<text x="100" y="145" text-anchor="middle" fill="white" font-size="130" font-weight="bold">K</text>`,
		kiro: `<path d="M50 20 L150 100 L70 100 L150 180" fill="none" stroke="white" stroke-width="14" stroke-linejoin="round" stroke-linecap="round"/>`,
		opencode: `<text x="100" y="155" text-anchor="middle" fill="white" font-size="140" font-weight="bold">{</text>`,
		qodercli: `<text x="100" y="155" text-anchor="middle" fill="white" font-size="110" font-weight="bold">&gt;_</text>`,
		amp: `<circle cx="100" cy="100" r="65" fill="none" stroke="white" stroke-width="12"/>
               <path d="M65 55 L135 100 L65 100 L135 145" fill="none" stroke="white" stroke-width="14" stroke-linejoin="round" stroke-linecap="round"/>`,
		antigravity: `<path d="M100 170 L100 50 M60 85 L100 50 L140 85" fill="none" stroke="white" stroke-width="14" stroke-linecap="round" stroke-linejoin="round"/>`,
		droid: `<rect x="55" y="35" width="90" height="80" rx="18" fill="none" stroke="white" stroke-width="14"/>
                 <circle cx="78" cy="85" r="8" fill="white"/>
                 <circle cx="122" cy="85" r="8" fill="white"/>
                 <rect x="75" y="125" width="50" height="40" rx="6" fill="none" stroke="white" stroke-width="12"/>
                 <line x1="60" y1="118" x2="55" y2="160" stroke="white" stroke-width="10" stroke-linecap="round"/>
                 <line x1="140" y1="118" x2="145" y2="160" stroke="white" stroke-width="10" stroke-linecap="round"/>`,
		hermes: `<path d="M40 40 L100 105 L160 40" fill="none" stroke="white" stroke-width="12" stroke-linecap="round" stroke-linejoin="round"/>
                 <rect x="35" y="40" width="130" height="100" rx="10" fill="none" stroke="white" stroke-width="12"/>
                 <path d="M40 105 L90 135 L100 145 L110 135 L160 105" fill="none" stroke="white" stroke-width="10" stroke-linejoin="round"/>`,
		unknown: `<circle cx="100" cy="70" r="40" fill="none" stroke="white" stroke-width="14"/>
                  <text x="100" y="165" text-anchor="middle" fill="white" font-size="70">?</text>`,
	};
}
