// DeckClient: WebSocket connection to UlanziDeck host
// Handles key rendering (state command) and input events

import WebSocket from "ws";
import sharp from "sharp";

const PLUGIN_UUID = "com.ulanzi.herdr.agentview";
const ACTION_UUID = "com.ulanzi.herdr.agentview.monitor";
const DEFAULT_PORT = 3906;
const DEFAULT_ADDR = "127.0.0.1";

export class DeckClient {
	constructor(onAddAction, onKeyDown) {
		this.ws = null;
		this.connected = false;
		this.address = DEFAULT_ADDR;
		this.port = DEFAULT_PORT;

		// key → actionid mapping (populated from profile or add events)
		this.keyActions = new Map();
		this.readyKeys = false;
		this._logAll = true;
		// Callbacks
		this.onAddAction = onAddAction || (() => {});
		this.onKeyDown = onKeyDown || (() => {});
	}

	async connect() {
		const args = process.argv.slice(2);
		this.address = args[0] || DEFAULT_ADDR;
		this.port = parseInt(args[1], 10) || DEFAULT_PORT;

		return new Promise((resolve, reject) => {
			this.ws = new WebSocket(`ws://${this.address}:${this.port}`);

			this.ws.on("open", () => {
				// Must match SDK format: code=0 + cmd=connected + uuid
				this.WS_SEND({
					code: 0,
					cmd: "connected",
					uuid: ACTION_UUID,
				});
				this.connected = true;
				console.log(`[deck] connected with action UUID: ${ACTION_UUID}`);
				resolve();
			});

			this.ws.on("message", (raw) => {
				try {
					const msg = JSON.parse(raw.toString());
					if (this._logAll) {
						// Log EVERY incoming message (truncate long data fields)
						const logMsg = JSON.parse(JSON.stringify(msg));
						if (logMsg.param?.statelist) {
							for (const s of logMsg.param.statelist) {
								if (s.data && s.data.length > 80)
									s.data = s.data.slice(0, 80) + "...";
							}
						}
						console.log("[deck] RECV:", JSON.stringify(logMsg));
					}
					this.handleMessage(msg);
				} catch (e) {
					console.error("[deck] parse error:", e.message);
				}
			});

			this.ws.on("close", () => {
				this.connected = false;
				console.log("[deck] disconnected, reconnecting in 2s...");
				setTimeout(() => this.connect().catch(() => {}), 2000);
			});

			this.ws.on("error", (err) => {
				console.error("[deck] ws error:", err.message);
				reject(err);
			});
		});
	}

	handleMessage(msg) {
		// Log what we receive
		if (msg.cmd || msg.code !== undefined) {
			// Continue processing - code fields are normal protocol responses
		}

		switch (msg.cmd) {
			case "connected":
				console.log(
					`[deck] connected: key=${msg.key || "?"} actionid=${msg.actionid || "?"}`,
				);
				break;

			case "add":
				// Store key→actionid mapping
				if (msg.key && msg.actionid) {
					this.keyActions.set(msg.key, msg.actionid);
					console.log(
						`[deck] add: key=${msg.key} actionid=${msg.actionid} (total: ${this.keyActions.size})`,
					);
					this.readyKeys = true;
					this.onAddAction(msg.key, msg.actionid);
				}
				break;

			case "clear":
				// Action removed from key(s)
				if (msg.param) {
					for (const item of msg.param) {
						const k = item.key;
						if (k) {
							this.keyActions.delete(k);
							console.log(`[deck] clear: key=${k}`);
						}
					}
				}
				break;

			case "keydown":
				this.onKeyDown(msg);
				break;

			case "keyup":
			case "run":
			case "setactive":
				break;
		}
	}

	// Seed key→actionid from profile (fallback if add events don't arrive)
	seedKeyActions(keyActionMap) {
		for (const [key, actionid] of Object.entries(keyActionMap)) {
			if (!this.keyActions.has(key)) {
				this.keyActions.set(key, actionid);
			}
		}
		this.readyKeys = this.keyActions.size > 0;
		console.log(
			`[deck] seeded ${Object.keys(keyActionMap).length} keys from profile`,
		);
	}

	// Render a key with base64 image (SVG → PNG via sharp)
	async setKeyImage(key, base64SvgData) {
		const actionid = this.keyActions.get(key) || "";

		try {
			// Convert SVG data URI → PNG buffer
			const svgBase64 = base64SvgData.replace(
				/^data:image\/svg\+xml;base64,/,
				"",
			);
			const svgBuffer = Buffer.from(svgBase64, "base64");
			const pngBuffer = await sharp(svgBuffer)
				.resize(196, 196, {
					fit: "contain",
					background: { r: 0, g: 0, b: 0, alpha: 0 },
				})
				.png()
				.toBuffer();
			const pngBase64 = `data:image/png;base64,${pngBuffer.toString("base64")}`;

			this.send("state", {
				param: {
					statelist: [
						{
							uuid: ACTION_UUID,
							key: key,
							actionid: actionid,
							type: 1,
							data: pngBase64,
							textData: "",
							showtext: false,
						},
					],
				},
			});
		} catch (err) {
			console.error(`[deck] svg→png failed for ${key}:`, err.message);
		}
	}

	_getFirstKeyAction() {
		// Get first available key→actionid for outer envelope
		const entries = Array.from(this.keyActions.entries());
		return entries.length > 0
			? { key: entries[0][0], actionid: entries[0][1] }
			: { key: "", actionid: "" };
	}

	// Raw send matching SDK format (no wrapping)
	WS_SEND(obj) {
		if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
		if (this._logAll) {
			const log = JSON.parse(JSON.stringify(obj));
			if (log.data && typeof log.data === "string" && log.data.length > 80)
				log.data = log.data.slice(0, 80) + "...";
			console.log("[deck] SEND:", JSON.stringify(log));
		}
		this.ws.send(JSON.stringify(obj));
	}

	send(cmd, params) {
		if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
		const { key: outerKey, actionid: outerActionId } =
			this._getFirstKeyAction();
		const payload = {
			cmd,
			uuid: PLUGIN_UUID,
			key: outerKey,
			actionid: outerActionId,
			...params,
		};
		// Log sent messages (truncate long data)
		if (this._logAll) {
			const logPayload = JSON.parse(JSON.stringify(payload));
			if (logPayload.param?.statelist) {
				for (const s of logPayload.param.statelist) {
					if (s.data && s.data.length > 80)
						s.data = s.data.slice(0, 80) + "...";
				}
			}
			console.log("[deck] SEND:", JSON.stringify(logPayload));
		}
		this.ws.send(JSON.stringify(payload));
	}
}
