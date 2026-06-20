// HerdrClient: Unix socket JSON-line protocol client
// Each request opens a new connection (matching herdr's Rust client behavior)

import net from "net";

let reqId = 0;

export class HerdrClient {
	constructor(socketPath) {
		this.socketPath = socketPath;
	}

	async request(method, params = {}) {
		const id = `deck:${++reqId}`;
		const req = JSON.stringify({ id, method, params }) + "\n";

		return new Promise((resolve, reject) => {
			const sock = net.createConnection({ path: this.socketPath });
			let buffer = "";
			let responded = false;

			const timeout = setTimeout(() => {
				if (!responded) {
					responded = true;
					sock.destroy();
					reject(new Error(`timeout: ${method}`));
				}
			}, 10000);

			sock.on("connect", () => {
				sock.write(req, "utf-8");
			});

			sock.on("data", (chunk) => {
				buffer += chunk.toString();
				const lines = buffer.split("\n");
				buffer = lines.pop() || "";
				for (const line of lines) {
					if (!line.trim()) continue;
					try {
						const msg = JSON.parse(line);
						if (!responded) {
							responded = true;
							clearTimeout(timeout);
							sock.end();
							if (msg.error) {
								reject(new Error(msg.error.message));
							} else {
								resolve(msg);
							}
						}
					} catch (e) {
						console.error("[herdr] parse:", e.message);
					}
				}
			});

			sock.on("error", (err) => {
				if (!responded) {
					responded = true;
					clearTimeout(timeout);
					reject(err);
				}
			});

			sock.on("close", () => {
				if (!responded) {
					responded = true;
					clearTimeout(timeout);
					reject(new Error("connection closed"));
				}
			});
		});
	}

	// Typed convenience methods
	async listWorkspaces() {
		const res = await this.request("workspace.list", {});
		return res.result.workspaces;
	}

	async listAgents() {
		const res = await this.request("agent.list", {});
		return res.result.agents;
	}

	async listPanes() {
		const res = await this.request("pane.list", {});
		return res.result.panes;
	}

	// Subscribe is a long-lived connection, handled separately
	async subscribe(params, onEvent) {
		const id = `deck:sub`;
		const req =
			JSON.stringify({ id, method: "events.subscribe", params }) + "\n";

		const sock = net.createConnection({ path: this.socketPath });
		let buffer = "";
		let ackReceived = false;

		return new Promise((resolve, reject) => {
			sock.on("connect", () => {
				sock.write(req, "utf-8");
			});

			sock.on("data", (chunk) => {
				buffer += chunk.toString();
				const lines = buffer.split("\n");
				buffer = lines.pop() || "";
				for (const line of lines) {
					if (!line.trim()) continue;
					try {
						const msg = JSON.parse(line);
						// First response is subscription ack
						if (!ackReceived) {
							ackReceived = true;
							if (msg.error) {
								sock.end();
								reject(new Error(msg.error.message));
							} else {
								resolve({ unsubscribe: () => sock.end() });
							}
							continue;
						}
						// Subsequent messages are events
						onEvent(msg);
					} catch (e) {
						console.error("[herdr] subscribe parse:", e.message);
					}
				}
			});

			sock.on("error", reject);

			sock.on("close", () => {
				if (!ackReceived) {
					reject(new Error("connection closed before ack"));
				}
			});
		});
	}
}
