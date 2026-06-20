// ConnectionManager: manages multiple herdr connections (local + SSH)
// For SSH: spawns ssh -NL tunnel (TCP port forward), connects herdr-client via TCP
// Matches Go implementation: go/pkg/herdr/tunnel.go

import { spawn } from "child_process";
import net from "net";
import { HerdrClient } from "./herdr-client.js";
import { findLocalSocket } from "./config.js";

export class ConnectionManager {
	constructor() {
		this.connections = [];
		this.tunnels = [];
	}

	async startAll(config) {
		for (const conn of config.connections) {
			await this.startConnection(conn);
		}
	}

	async startConnection(cfg) {
		let target;

		if (cfg.type === "local") {
			const socketPath = findLocalSocket();
			if (!socketPath) {
				console.warn(`[conn] local socket not found for "${cfg.name}"`);
				return;
			}
			target = socketPath; // string → HerdrClient uses Unix socket
		} else if (cfg.type === "ssh") {
			const localPort = cfg.localPort || 19999;
			target = await this._startSSHTunnel(cfg, localPort);
			if (!target) {
				console.warn(`[conn] SSH tunnel failed for "${cfg.name}"`);
				return;
			}
		} else {
			console.warn(`[conn] unknown connection type: ${cfg.type}`);
			return;
		}

		const client = new HerdrClient(target);
		this.connections.push({
			name: cfg.name,
			abbr: cfg.abbr,
			color: cfg.color,
			client,
		});
	}

	// SSH tunnel via TCP port forwarding (matches Go tunnel.go)
	// ssh -NL <localPort>:<remoteSocket> <host>
	async _startSSHTunnel(cfg, localPort) {
		const addr = `127.0.0.1:${localPort}`;

		const proc = spawn("ssh", [
			"-NL",
			`${localPort}:${cfg.remoteSocket}`,
			cfg.host,
		], { stdio: ["ignore", "pipe", "pipe"] });

		const ready = await waitForPort(localPort, 10000);
		if (!ready) {
			proc.kill("SIGTERM");
			return null;
		}

		this.tunnels.push(proc);
		console.log(`[conn] SSH tunnel ${cfg.name} → ${addr}`);
		return { host: "127.0.0.1", port: localPort };
	}

	stopAll() {
		for (const t of this.tunnels) {
			t.kill("SIGINT");
			// Fallback SIGKILL after 1s (matches Go tunnel.go Close)
			setTimeout(() => {
				try { t.kill("SIGKILL"); } catch { /* already dead */ }
			}, 1000);
		}
		this.tunnels = [];
		this.connections = [];
	}
}

// Poll a TCP port until it accepts connections, or timeout.
// Matches Go tunnel.go WaitReady.
export function waitForPort(port, timeoutMs = 10000) {
	const start = Date.now();
	return new Promise((resolve) => {
		function poll() {
			const sock = new net.Socket();
			sock.setTimeout(300);
			sock.on("connect", () => {
				sock.destroy();
				resolve(true);
			});
			sock.on("error", () => {
				sock.destroy();
				if (Date.now() - start < timeoutMs) {
					setTimeout(poll, 100);
				} else {
					resolve(false);
				}
			});
			sock.on("timeout", () => {
				sock.destroy();
				if (Date.now() - start < timeoutMs) {
					setTimeout(poll, 100);
				} else {
					resolve(false);
				}
			});
			sock.connect(port, "127.0.0.1");
		}
		poll();
	});
}
