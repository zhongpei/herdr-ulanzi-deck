// herdr-deck: UlanziDeck plugin for Herdr agent status display
//
// Entry point. Uses mock data for initial display testing.
// To test: copy plugin folder to UlanziDeckSimulator/plugins or use real deck.

import { StateManager } from "./state-manager.js";
import { ButtonMapper } from "./button-mapper.js";
import { IconRenderer } from "./icon-renderer.js";
import { DeckClient } from "./deck-client.js";
import { ProfileManager } from "./profile-manager.js";
import { buildMockUnifiedWorkspaces } from "./mock-data.js";

// Module-level state (initialized by main())
let stateManager;
let buttonMapper;
let deckClient;

// ─── Physical key map for D200X ─────────────────────────────────
// Row 0: K1-K5  → key_0_0 ~ key_0_4  (indices 0-4)
// Row 1: K6-K10 → key_1_0 ~ key_1_4  (indices 5-9)
// D200X key layout: col_row format
// Row 0 (col 0-4): K1-K5  → 0_0 1_0 2_0 3_0 4_0
// Row 1 (col 0-4): K6-K10 → 0_1 1_1 2_1 3_1 4_1
// Row 2 (col 0-2): K11-K13 → 0_2 1_2 2_2
//        (col 3, span 2): K14 (large) → 3_2

const KEY_MAP = {
	"0_0": 0,
	"1_0": 1,
	"2_0": 2,
	"3_0": 3,
	"4_0": 4,
	"0_1": 5,
	"1_1": 6,
	"2_1": 7,
	"3_1": 8,
	"4_1": 9,
	"0_2": 10,
	"1_2": 11,
	"2_2": 12,
	"3_2": 13,
};

// Reverse map: key descriptor → physical key
function physicalKeyForDescriptor(keyId) {
	const map = {
		nav_prev: "0_2", // K11
		nav_current: "1_2", // K12
		nav_next: "2_2", // K13
		stats: "3_2", // K14 (large, spans col 3-4 row 2)
	};
	if (map[keyId]) return map[keyId];

	if (keyId.startsWith("agent_")) {
		const idx = parseInt(keyId.split("_")[1]);
		if (idx >= 0 && idx <= 4) return `${idx}_0`; // K1-K5 → col 0-4, row 0
		if (idx >= 5 && idx <= 9) return `${idx - 5}_1`; // K6-K10 → col 0-4, row 1
	}
	return "0_0";
}

// ─── App ─────────────────────────────────────────────────────────
async function main() {
	stateManager = new StateManager();
	const iconRenderer = new IconRenderer();
	buttonMapper = new ButtonMapper(stateManager);

	// Init with mock data
	const mockData = buildMockUnifiedWorkspaces();
	stateManager.init(mockData);

	// Create dedicated profile and extract key→actionid mapping
	let profileKeyActions = {};
	const args = process.argv.slice(2);
	const deckPort = parseInt(args[1], 10) || 3906;
	if (deckPort === 3906) {
		try {
			const pm = new ProfileManager();
			const profileDir = pm.ensure("02d04a045u3673881");
			if (profileDir) {
				profileKeyActions = pm.getKeyActionMap();
				console.log(
					`[main] profile ready, ${Object.keys(profileKeyActions).length} key actions mapped`,
				);
			}
		} catch (err) {
			console.error("[main] profile setup failed:", err.message);
		}
	}

	// Connect to deck
	deckClient = new DeckClient(
		(key, actionid) => {
			console.log(`[main] action added: key=${key} actionid=${actionid}`);
			// Re-render when new keys are assigned
			renderAll(buttonMapper, iconRenderer, deckClient);
		},
		(msg) => {
			handleKeyDown(msg, buttonMapper, iconRenderer);
		},
	);

	// Seed key actions from profile (before first render)
	if (Object.keys(profileKeyActions).length > 0) {
		deckClient.seedKeyActions(profileKeyActions);
	}

	try {
		await deckClient.connect();
	} catch (err) {
		console.error("[main] failed to connect to deck:", err.message);
		console.log("[main] falling back to console output for debugging");
	}

	// Render initial state
	renderAll(buttonMapper, iconRenderer, deckClient);

	// Auto-refresh on state change
	stateManager.onChange(() => {
		renderAll(buttonMapper, iconRenderer, deckClient);
	});

	// Log page info for debugging
	logPageInfo(buttonMapper, stateManager);

	// Keep alive
	console.log("[main] herdr-deck running. Press Ctrl+C to stop.");
}

// ─── Render all 14 keys ──────────────────────────────────────────
async function renderAll(mapper, renderer, deck) {
	const keyData = mapper.renderAll();

	const promises = [];
	for (const kd of keyData) {
		let svg;

		switch (kd.type) {
			case "agent":
				svg = renderer.renderAgentKey(kd);
				break;
			case "navPrev":
			case "navNext":
				svg = renderer.renderNavKey(kd.type, kd.label, kd.enabled);
				break;
			case "navCurrent":
				svg = renderer.renderCurrentKey(kd);
				break;
			case "stats":
				svg = renderer.renderStatsKey(kd.stats);
				break;
			default:
				svg = renderer.renderEmptyKey();
		}

		const physKey = physicalKeyForDescriptor(kd.keyId);

		if (deck && deck.connected && kd.type !== "empty") {
			const isWide = physKey === "3_2";
			promises.push(deck.setKeyImage(physKey, svg, isWide));
		}

		// Debug output to console
		if (kd.type !== "empty") {
			console.log(`[render] ${physKey} (${kd.type}) → ${svg.slice(0, 50)}...`);
		}
	}

	// Wait for all PNG conversions to complete
	await Promise.all(promises);

	// Log all key images in a compact view (D200X: col_row format)
	console.log("[render] --- key map ---");
	const lines = [
		["0_0", "1_0", "2_0", "3_0", "4_0"],
		["0_1", "1_1", "2_1", "3_1", "4_1"],
		["0_2", "1_2", "2_2", "3_2"],
	];
	for (const row of lines) {
		const labels = row.map((k) => {
			const kd = keyData[KEY_MAP[k]];
			if (!kd || kd.type === "empty") return "·";
			if (kd.type === "agent") {
				const s = kd.status?.[0]?.toUpperCase() || "?";
				return `${kd.agentType?.[0]?.toUpperCase() || "?"}${s}`;
			}
			if (kd.type === "navPrev") return "<";
			if (kd.type === "navCurrent") return "≡";
			if (kd.type === "navNext") return ">";
			if (kd.type === "stats") return "∑";
			return "·";
		});
		console.log(`[render]   ${labels.join(" │ ")}`);
	}
	console.log(
		`[render] --- page ${mapper.getCurrentPage() + 1}/${stateManager.computePages().length} ---`,
	);
}

// ─── Key press handler ───────────────────────────────────────────
function handleKeyDown(msg, mapper, iconRenderer) {
	const physKey = msg.key;

	if (physKey === "0_2") {
		// K11 — previous page
		if (mapper.prevPage()) {
			renderAll(mapper, iconRenderer, deck);
		}
	} else if (physKey === "2_2") {
		// K13 — next page
		if (mapper.nextPage()) {
			renderAll(mapper, iconRenderer, deck);
		}
	} else if (physKey === "1_2") {
		// K12 — current page info (no action)
	} else {
		// Agent key — could focus agent in future (requires herdr connection)
		const idx = KEY_MAP[physKey];
		if (idx !== undefined && idx < 10) {
			const keyData = mapper.renderAll();
			const agentData = keyData[idx];
			if (agentData && agentData.type === "agent") {
				console.log(
					`[action] focus agent: ${agentData.connName}/${agentData.paneId} (${agentData.alias})`,
				);
			}
		}
	}
}

// ─── Debug ───────────────────────────────────────────────────────
function logPageInfo(_mapper, stateManager) {
	const pages = stateManager.computePages();
	console.log(`[info] ${pages.length} page(s) total`);

	for (let p = 0; p < pages.length; p++) {
		const page = pages[p];
		const desc = page
			.map((chunk, i) => {
				if (!chunk) return `row${i + 1}: empty`;
				const agentCount = chunk.agents.length;
				const label = `${chunk.connAbbr}:${chunk.label}`;
				return `row${i + 1}: ${label} (${agentCount} agents)`;
			})
			.join(", ");
		console.log(`[info]   page ${p + 1}: ${desc}`);
	}

	const stats = stateManager.computeStats();
	console.log(
		`[info] stats: ✅${stats.done} ⏸${stats.idle} ⏳${stats.working} ❌${stats.blocked} ❓${stats.unknown}`,
	);
}

// ─── Boot ────────────────────────────────────────────────────────
main().catch((err) => {
	console.error("[main] fatal:", err);
	process.exit(1);
});
