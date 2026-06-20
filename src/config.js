// Config: read connections.json from ~/.config/herdr-deck/
// Defines local and remote herdr instances

import fs from "fs";
import path from "path";
import os from "os";

const CONFIG_DIR = path.join(os.homedir(), ".config", "herdr-deck");
const CONFIG_PATH = path.join(CONFIG_DIR, "connections.json");

const DEFAULT_CONFIG = {
	connections: [
		{
			name: "local",
			abbr: "LCL",
			color: "#4ADE80",
			type: "local",
		},
	],
};

export function loadConfig() {
	try {
		if (!fs.existsSync(CONFIG_PATH)) {
			// Create default config
			fs.mkdirSync(CONFIG_DIR, { recursive: true });
			fs.writeFileSync(CONFIG_PATH, JSON.stringify(DEFAULT_CONFIG, null, "\t"));
			console.log(`[config] created default at ${CONFIG_PATH}`);
			return DEFAULT_CONFIG;
		}
		const raw = fs.readFileSync(CONFIG_PATH, "utf-8");
		const cfg = JSON.parse(raw);
		return cfg;
	} catch (err) {
		console.warn(`[config] error loading ${CONFIG_PATH}: ${err.message}`);
		return DEFAULT_CONFIG;
	}
}

// Find a local herdr socket by checking common paths
export function findLocalSocket() {
	const candidates = [
		process.env.HERDR_SOCKET_PATH,
		path.join(os.homedir(), ".config", "herdr", "herdr.sock"),
		path.join(os.homedir(), ".local", "share", "herdr", "herdr.sock"),
		"/tmp/herdr.sock",
	];
	for (const p of candidates) {
		if (!p) continue;
		try {
			if (fs.statSync(p).isSocket()) return p;
		} catch {}
		// Some herdr sockets are regular files with special permissions
		try {
			if (fs.statSync(p).isFile()) return p;
		} catch {}
	}
	return null;
}
