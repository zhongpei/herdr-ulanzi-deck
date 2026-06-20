// ConnectionManager: manages multiple herdr connections (local + SSH)
// For SSH: spawns ssh -L tunnel (Unix socket forward), connects herdr-client

import { spawn } from "child_process";
import fs from "fs";
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
		let socketPath = null;

		if (cfg.type === "local") {
			socketPath = findLocalSocket();
			if (!socketPath) {
				console.warn(`[conn] local socket not found for "${cfg.name}"`);
				return;
			}
		} else if (cfg.type === "ssh") {
			socketPath = await this.startSSHTunnel(cfg);
			if (!socketPath) {
				console.warn(`[conn] SSH tunnel failed for "${cfg.name}"`);
				return;
			}
		} else {
			console.warn(`[conn] unknown type: ${cfg.type}`);
			return;
		}

		const client = new HerdrClient(socketPath);
		try {
			const [workspaces, agents] = await Promise.all([
				client.listWorkspaces(),
				client.listAgents(),
			]);
			console.log(
				`[conn] ${cfg.name}: ${workspaces.length} ws, ${agents.length} agents`,
			);
			this.connections.push({
				name: cfg.name,
				abbr: cfg.abbr,
				color: cfg.color,
				client,
				workspaces,
				agents,
			});
		} catch (err) {
			console.warn(
				`[conn] herdr connect failed for "${cfg.name}": ${err.message}`,
			);
		}
	}

	// SSH tunnel via Unix socket forwarding
	async startSSHTunnel(cfg) {
		const localSocket = `/tmp/herdr-${cfg.name}.sock`;
		try {
			fs.unlinkSync(localSocket);
		} catch {}

		const proc = spawn(
			"ssh",
			[
				"-L",
				`${localSocket}:${cfg.remoteSocket}`,
				cfg.host,
				"-N",
				"-o",
				"ExitOnForwardFailure=yes",
				"-o",
				"ServerAliveInterval=30",
			],
			{ stdio: ["ignore", "pipe", "pipe"] },
		);

		return new Promise((resolve, reject) => {
			let resolved = false;
			const check = setInterval(() => {
				try {
					if (fs.statSync(localSocket).isSocket()) {
						resolved = true;
						clearInterval(check);
						this.tunnels.push(proc);
						console.log(`[conn] SSH tunnel ${cfg.name} → ${localSocket}`);
						resolve(localSocket);
					}
				} catch {}
			}, 200);

			setTimeout(() => {
				if (!resolved) {
					resolved = true;
					clearInterval(check);
					proc.kill();
					reject(new Error("SSH tunnel timeout"));
				}
			}, 15000);

			proc.on("error", (err) => {
				if (!resolved) {
					resolved = true;
					clearInterval(check);
					reject(err);
				}
			});
			proc.on("exit", (code) => {
				if (!resolved) {
					resolved = true;
					clearInterval(check);
					reject(new Error(`ssh exit ${code}`));
				}
			});
		});
	}

	async stopAll() {
		for (const t of this.tunnels) t.kill();
		this.tunnels = [];
		this.connections = [];
	}
}
