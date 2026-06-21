// ProfileManager: creates a dedicated UlanziDeck profile for herdr-deck
// All 14 D200X keys are assigned to our action so we can render them all.

import fs from "fs";
import path from "path";
import crypto from "crypto";

const PROFILES_DIR = path.join(
	process.env.HOME || process.env.USERPROFILE,
	"Library/Application Support/Ulanzi/UlanziDeck/ProfilesV2",
);

const PLUGIN_UUID = "com.ulanzi.herdr.agentview";
const ACTION_UUID = "com.ulanzi.herdr.agentview.monitor";
const PROFILE_NAME = "Herdr Deck";

// D200X key positions in col_row format (all 14 visible keys)
const D200X_KEYS = [
	"0_0",
	"1_0",
	"2_0",
	"3_0",
	"4_0", // row 0
	"0_1",
	"1_1",
	"2_1",
	"3_1",
	"4_1", // row 1
	"0_2",
	"1_2",
	"2_2",
	"3_2", // row 2 (3_2 = large)
];

export class ProfileManager {
	constructor() {
		this.profileDir = null;
	}

	// Find our profile or return null
	findOurProfile() {
		if (!fs.existsSync(PROFILES_DIR)) return null;

		const entries = fs.readdirSync(PROFILES_DIR, { withFileTypes: true });
		for (const entry of entries) {
			if (!entry.isDirectory() || !entry.name.endsWith(".ulanziProfile")) {
				continue;
			}
			const manifestPath = path.join(PROFILES_DIR, entry.name, "manifest.json");
			if (!fs.existsSync(manifestPath)) continue;

			try {
				const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf-8"));
				if (manifest.Name === PROFILE_NAME) {
					this.profileDir = path.join(PROFILES_DIR, entry.name);
					return this.profileDir;
				}
			} catch {
				// skip invalid
			}
		}
		return null;
	}

	// Create our dedicated profile with all 14 keys assigned, 4 pages
	createProfile(deviceUuid) {
		const profileUuid = crypto.randomUUID();
		this.profileDir = path.join(PROFILES_DIR, `${profileUuid}.ulanziProfile`);

		fs.mkdirSync(path.join(this.profileDir, "Profiles"), { recursive: true });

		// Build page manifest helper (14 keys + encoders)
		const buildPageManifest = () => {
			const keypadActions = {};
			for (const key of D200X_KEYS) {
				keypadActions[key] = {
					Action: ACTION_UUID,
					ActionID: crypto.randomUUID(),
					ActionParam: {},
					LinkedTitle: false,
					Name: "Agent",
					Plugin: {
						Name: "Herdr Agent View",
						UUID: PLUGIN_UUID,
						Version: "0.1.0",
					},
					State: 0,
					ViewParam: [{ Icon: "", IconRel: "" }],
				};
			}
			return {
				Controllers: [
					{ Actions: {}, Type: "Encoder" },
					{ Actions: keypadActions, Type: "Keypad" },
				],
			};
		};

		// Create 4 pages
		const pageUuids = [];
		for (let i = 0; i < 4; i++) {
			const puid = crypto.randomUUID();
			pageUuids.push(puid);
			const pageDir = path.join(this.profileDir, "Profiles", puid);
			fs.mkdirSync(path.join(pageDir, "Files"), { recursive: true });
			fs.mkdirSync(path.join(pageDir, "Images"), { recursive: true });
			fs.writeFileSync(
				path.join(pageDir, "manifest.json"),
				JSON.stringify(buildPageManifest(), null, "\t"),
			);
		}

		// Profile manifest with 4 pages
		fs.writeFileSync(
			path.join(this.profileDir, "manifest.json"),
			JSON.stringify(
				{
					Device: { Model: "D200X", UUID: deviceUuid },
					Icon: "icon_default_profile.png",
					Name: PROFILE_NAME,
					Pages: { Current: pageUuids[0], Pages: pageUuids },
					Version: 2,
				},
				null,
				"\t",
			),
		);

		fs.writeFileSync(
			path.join(this.profileDir, "icon_default_profile.png"),
			Buffer.alloc(0),
		);

		console.log(
			`[profile] created "${PROFILE_NAME}" (${pageUuids.length} pages)`,
		);
		return this.profileDir;
	}

	// Activate our profile by updating setting.json
	activateProfile(deviceUuid) {
		if (!this.profileDir) return;

		// Read current device settings
		const settingPath = path.join(
			path.dirname(PROFILES_DIR),
			"config",
			"setting.json",
		);
		if (!fs.existsSync(settingPath)) {
			console.warn("[profile] setting.json not found");
			return;
		}

		try {
			const setting = JSON.parse(fs.readFileSync(settingPath, "utf-8"));
			const profileName = PROFILE_NAME;

			// Update CurrentProfile for D200X device
			if (setting.CurrentProfile && !setting.CurrentProfile.includes("D200X")) {
				// Simple config - just update
				setting.CurrentProfile = profileName;
			}

			// Also update device-specific settings if present
			if (setting.Devices) {
				for (const dev of setting.Devices) {
					if (dev.DeviceType === "D200X" || dev.CurrentDevice === deviceUuid) {
						dev.CurrentProfile = profileName;
					}
				}
			}

			fs.writeFileSync(settingPath, JSON.stringify(setting, null, "\t"));
			console.log(`[profile] activated "${PROFILE_NAME}"`);
		} catch (err) {
			console.error("[profile] failed to activate:", err.message);
		}
	}

	// Read the key→actionid map from our profile
	getKeyActionMap() {
		if (!this.profileDir) return {};

		const pagesDir = path.join(this.profileDir, "Profiles");
		if (!fs.existsSync(pagesDir)) return {};

		const pages = fs
			.readdirSync(pagesDir, { withFileTypes: true })
			.filter((e) => e.isDirectory());

		if (pages.length === 0) return {};

		// Read first page's manifest
		const pageManifestPath = path.join(
			pagesDir,
			pages[0].name,
			"manifest.json",
		);
		if (!fs.existsSync(pageManifestPath)) return {};

		try {
			const manifest = JSON.parse(fs.readFileSync(pageManifestPath, "utf-8"));
			const keypad = manifest.Controllers.find((c) => c.Type === "Keypad");
			if (!keypad) return {};

			const map = {};
			for (const [key, action] of Object.entries(keypad.Actions)) {
				map[key] = action.ActionID;
			}
			return map;
		} catch {
			return {};
		}
	}

	// Ensure our profile exists and return the profile dir
	ensure(deviceUuid) {
		const existing = this.findOurProfile();
		if (existing) {
			console.log(`[profile] found existing "${PROFILE_NAME}"`);
			this.profileDir = existing;
			return existing;
		}

		return this.createProfile(deviceUuid);
	}
}
