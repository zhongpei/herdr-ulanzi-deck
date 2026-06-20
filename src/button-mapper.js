// ButtonMapper: maps state + filter → 14 key render commands
//
// Filter modes:
//   ALL         → show all machines (K11 active)
//   Machine     → show one machine's agents (K12 cycles)
//   Machine+WS  → show one workspace's agents (K12 + K13 intersection)
//
// Sort: BLOCKED(0) > DONE(1) > WORKING(2) > IDLE(3) > UNKNOWN(4)

export class ButtonMapper {
	constructor(stateManager) {
		this.state = stateManager;
		// Filter state
		this.mode = "all"; // "all" | "machine" | "space"
		this.connName = null; // current machine filter
		this.wsId = null; // current space filter
	}

	// ─── Filter operations ───────────────────────────────────────────
	setAll() {
		this.mode = "all";
		this.connName = null;
		this.wsId = null;
	}

	nextMachine() {
		const machines = this.state.getMachines();
		if (machines.length === 0) return;

		if (!this.connName) {
			// From ALL → first machine
			this.connName = machines[0].connName;
		} else {
			const idx = machines.findIndex((m) => m.connName === this.connName);
			this.connName = machines[(idx + 1) % machines.length].connName;
		}
		this.mode = "machine";
		this.wsId = null; // clear space filter
	}

	nextSpace() {
		if (!this.connName) return; // ALL mode: no space filtering

		const spaces = this.state.getSpaces(this.connName);
		if (spaces.length === 0) return;

		// If wsId is stale (not in current machine's spaces), reset
		if (this.wsId && !spaces.find((s) => s.wsId === this.wsId)) {
			this.wsId = null;
		}

		if (!this.wsId) {
			this.wsId = spaces[0].wsId;
		} else {
			const idx = spaces.findIndex((s) => s.wsId === this.wsId);
			this.wsId = spaces[(idx + 1) % spaces.length].wsId;
		}
		this.mode = "space";
	}

	// ─── Render ──────────────────────────────────────────────────────
	renderAll() {
		const agents = this.state.getFilteredAgents(this.connName, this.wsId);
		const machines = this.state.getMachines();
		const stats = this.state.computeStats();
		const currentMachine = machines.find((m) => m.connName === this.connName);

		// Current machine info for K12/K13 display
		const machineIdx = this.connName
			? machines.findIndex((m) => m.connName === this.connName)
			: -1;
		const nextMachine =
			machineIdx >= 0
				? machines[(machineIdx + 1) % machines.length]
				: machines.length > 1
					? machines[1]
					: null;
		const spaces = this.connName ? this.state.getSpaces(this.connName) : [];
		const spaceIdx = this.wsId
			? spaces.findIndex((s) => s.wsId === this.wsId)
			: -1;
		const nextSpace =
			spaceIdx >= 0
				? spaces[(spaceIdx + 1) % spaces.length]
				: spaces.length > 0
					? spaces[0]
					: null;

		const keys = [
			// K1-K10: agents
			...this.renderAgents(agents),
			// K11: ALL button
			this.renderAllButton(this.mode === "all"),
			// K12: machine cycle
			this.renderMachineButton(
				currentMachine,
				nextMachine,
				machines,
				this.mode === "machine" || this.mode === "space",
			),
			// K13: space cycle
			this.renderSpaceButton(spaces, nextSpace, this.mode === "space"),
			// K14: stats
			this.renderStats(stats),
		];

		return keys;
	}

	renderAgents(agents) {
		const keys = [];
		for (let i = 0; i < 10; i++) {
			const agent = agents[i];
			if (agent) {
				keys.push({
					keyId: `agent_${i}`,
					type: "agent",
					agentType: agent.agent,
					alias: agent.name || agent.agent || "",
					status: agent.agent_status,
					focused: !!agent.focused,
					paneId: agent.pane_id,
					connName: agent.connName,
					connAbbr: agent.connAbbr,
					connAbbrColor: agent.connAbbrColor || "#888888",
					wsLabel: agent.wsLabel || "",
				});
			} else {
				keys.push({ keyId: `empty_${i}`, type: "empty" });
			}
		}
		return keys;
	}

	renderAllButton(active) {
		return {
			keyId: "nav_all",
			type: "navAll",
			label: "ALL",
			active,
		};
	}

	renderMachineButton(current, next, _allMachines, active) {
		return {
			keyId: "nav_machine",
			type: "navMachine",
			currentAbbr: current ? current.connAbbr : "-",
			currentColor: current ? current.connAbbrColor : "#888",
			nextAbbr: next ? next.connAbbr : "-",
			active,
		};
	}

	renderSpaceButton(spaces, nextSpace, active) {
		return {
			keyId: "nav_space",
			type: "navSpace",
			count: spaces.length,
			nextLabel: nextSpace ? nextSpace.label : "-",
			active,
		};
	}

	renderStats(stats) {
		return { keyId: "stats", type: "stats", stats };
	}
}
