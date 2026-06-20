// herdr-deck: UlanziDeck plugin for Herdr agent status display

import { StateManager } from "./state-manager.js";
import { ButtonMapper } from "./button-mapper.js";
import { IconRenderer } from "./icon-renderer.js";
import { DeckClient } from "./deck-client.js";
import { ProfileManager } from "./profile-manager.js";
import { HerdrBridge } from "./herdr-bridge.js";
import { buildMockUnifiedWorkspaces } from "./mock-data.js";
import fs from "fs";

// Module-level state
let stateManager;
let buttonMapper;
let deckClient;
let herdrBridge; // for agent.focus calls

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
		nav_all: "0_2", // K11
		nav_machine: "1_2", // K12
		nav_space: "2_2", // K13
		stats: "3_2", // K14
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

	// Try connecting to herdr for real data; fall back to mock
	const args = process.argv.slice(2);
	const deckPort = parseInt(args[1], 10) || 3906;

	let herdrSocket = null;
	for (const p of [
		process.env.HERDR_SOCKET_PATH,
		"/Users/fofo/.config/herdr/herdr.sock",
		process.env.HOME + "/.local/share/herdr/herdr.sock",
	]) {
		try {
			if (p && fs.statSync(p).isSocket()) { herdrSocket = p; break; }
		} catch {}
	}

	if (herdrSocket) {
		try {
			const bridge = new HerdrBridge(herdrSocket);
			herdrBridge = bridge;
			const unified = await bridge.fetchUnifiedWorkspaces();
			stateManager.init(unified);
			console.log(`[main] herdr: ${unified.length} ws, ${stateManager.getAllAgents().length} agents`);

			// Subscribe to real-time events
			bridge.subscribeToEvents((event) => {
				if (event.type === "agent_status_changed" || event.type === "workspace_changed") {
					refreshFromHerdr(bridge);
				}
			}).catch(() => {});
		} catch (err) {
			console.warn(`[main] herdr fail (${err.message}), using mock`);
			herdrBridge = null;
			stateManager.init(buildMockUnifiedWorkspaces());
		}
	} else {
		console.warn("[main] no herdr socket, using mock");
		stateManager.init(buildMockUnifiedWorkspaces());
	}

	// Create dedicated profile and extract key→actionid mapping
	let profileKeyActions = {};
	if (deckPort === 3906) {
		try {
			const pm = new ProfileManager();
			const profileDir = pm.ensure("02d04a045u3673881");
			if (profileDir) {
				profileKeyActions = pm.getKeyActionMap();
				console.log(`[main] profile ready, ${Object.keys(profileKeyActions).length} keys`);
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

	// Log filter info for debugging
	logFilterInfo(buttonMapper, stateManager);

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
			case "navAll":
				svg = renderer.renderNavAll(kd);
				break;
			case "navMachine":
				svg = renderer.renderNavMachine(kd);
				break;
			case "navSpace":
				svg = renderer.renderNavSpace(kd);
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
			if (kd.type === "navAll") return "A";
			if (kd.type === "navMachine") return "M";
			if (kd.type === "navSpace") return "S";
			if (kd.type === "stats") return "∑";
			return "·";
		});
		console.log(`[render]   ${labels.join(" │ ")}`);
	}
	console.log(
		`[render] --- mode ${mapper.mode} conn=${mapper.connName || "-"} ws=${mapper.wsId || "-"} ---`,
	);
}

// ─── Key press handler ───────────────────────────────────────────
function handleKeyDown(msg, mapper, iconRenderer) {
	const physKey = msg.key;
	console.log("[input] keydown: key=" + physKey + " full=" + JSON.stringify(msg));

	switch (physKey) {
		case "0_2": // K11 — ALL
			console.log("[nav] ALL pressed");
			mapper.setAll();
			renderAll(mapper, iconRenderer, deckClient);
			break;
		case "0_3": // hardware prev page → ALL
			console.log("[nav] hw prev → ALL");
			mapper.setAll();
			renderAll(mapper, iconRenderer, deckClient);
			break;
		case "1_2": // K12 — next machine
			console.log("[nav] machine cycle pressed");
			mapper.nextMachine();
			renderAll(mapper, iconRenderer, deckClient);
			break;
		case "1_3": // hardware next page → next machine
			console.log("[nav] hw next → machine cycle");
			mapper.nextMachine();
			renderAll(mapper, iconRenderer, deckClient);
			break;
		case "2_2": // K13 — next space
			console.log("[nav] space cycle pressed");
			mapper.nextSpace();
			renderAll(mapper, iconRenderer, deckClient);
			break;
		default: {
			// Agent key
			const idx = KEY_MAP[physKey];
			if (idx !== undefined && idx < 10) {
				const keyData = mapper.renderAll();
				const agentData = keyData[idx];
				if (agentData && agentData.type === "agent") {
					console.log(`[action] focus: ${agentData.connName}/${agentData.paneId}`);
					if (herdrBridge) {
						herdrBridge.client.request("agent.focus", { target: agentData.paneId }).catch(() => {});
					}
				}
			}
		}
	}
}

// ─── Herdr refresh ──────────────────────────────────────────────
async function refreshFromHerdr(bridge) {
	try {
		const unified = await bridge.fetchUnifiedWorkspaces();
		stateManager.init(unified);
	} catch (err) {
		console.warn("[herdr] refresh failed:", err.message);
	}
}

// ─── Debug ───────────────────────────────────────────────────────
function logFilterInfo(_mapper, stateManager) {
	const machines = stateManager.getMachines();
	const allAgents = stateManager.getAllAgents();
	const stats = stateManager.computeStats();

	console.log(
		`[info] ${machines.length} machine(s), ${allAgents.length} total agents`,
	);
	console.log(
		`[info] stats: ✅${stats.done} ⏸${stats.idle} ⏳${stats.working} ❌${stats.blocked} ❓${stats.unknown}`,
	);

	// Show top 10 sorted agents
	const top10 = stateManager.getFilteredAgents(null, null);
	console.log("[info] top 10 all:");
	top10.forEach((a, i) => {
		console.log(
			`  ${i + 1}. [${a.connAbbr}] ${a.agent}/${a.name || "?"} = ${a.agent_status}`,
		);
	});
}

// ─── Boot ────────────────────────────────────────────────────────
main().catch((err) => {
	console.error("[main] fatal:", err);
	process.exit(1);
});
