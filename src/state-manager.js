// StateManager: manages unified workspace tree with sort + filter
// No pagination. Agents are sorted by priority: BLOCKED > DONE > WORKING > IDLE > UNKNOWN
// K1-K10 show top 10 after filtering by machine/space

const STATUS_PRIORITY = {
	blocked: 0,
	done: 1,
	working: 2,
	idle: 3,
	unknown: 4,
};

export class StateManager {
	constructor() {
		this.unified = []; // UnifiedWorkspace[]
		this.listeners = [];
	}

	init(unifiedWorkspaces) {
		this.unified = unifiedWorkspaces;
		this.notify("stateChanged");
	}

	// Get all agents flattened with their workspace/machine info
	getAllAgents() {
		const agents = [];
		for (const ws of this.unified) {
			for (const agent of ws.agents) {
				agents.push({
					...agent,
					connName: ws.connName,
					connAbbr: ws.connAbbr,
					connAbbrColor: ws.connAbbrColor,
					wsLabel: ws.label,
					wsId: ws.workspace_id,
				});
			}
		}
		return agents;
	}

	// Get sorted, filtered, truncated agent list
	getFilteredAgents(filterConnName, filterWsId) {
		let agents = this.getAllAgents();

		// Apply machine filter
		if (filterConnName) {
			agents = agents.filter((a) => a.connName === filterConnName);
		}

		// Apply space filter (intersection with machine filter)
		if (filterWsId) {
			agents = agents.filter((a) => a.wsId === filterWsId);
		}

		// Sort: by status priority, then by machine order
		agents.sort((a, b) => {
			const pa = STATUS_PRIORITY[a.agent_status] ?? 4;
			const pb = STATUS_PRIORITY[b.agent_status] ?? 4;
			if (pa !== pb) return pa - pb;
			// Same status: group by machine (use original unified order)
			return (a.connName || "").localeCompare(b.connName || "");
		});

		return agents.slice(0, 10); // K1-K10
	}

	// Get unique machine names in order
	getMachines() {
		const seen = new Set();
		const machines = [];
		for (const ws of this.unified) {
			if (!seen.has(ws.connName)) {
				seen.add(ws.connName);
				machines.push({
					connName: ws.connName,
					connAbbr: ws.connAbbr,
					connAbbrColor: ws.connAbbrColor,
				});
			}
		}
		return machines;
	}

	// Get unique spaces for a given machine
	getSpaces(connName) {
		const spaces = [];
		for (const ws of this.unified) {
			if (ws.connName === connName) {
				spaces.push({ wsId: ws.workspace_id, label: ws.label });
			}
		}
		return spaces;
	}

	// Global stats across ALL workspaces (K14)
	computeStats() {
		const stats = { done: 0, idle: 0, working: 0, blocked: 0, unknown: 0 };
		for (const ws of this.unified) {
			for (const agent of ws.agents) {
				const s = agent.agent_status;
				if (stats[s] !== undefined) stats[s]++;
				else stats.unknown++;
			}
		}
		return stats;
	}

	onChange(fn) {
		this.listeners.push(fn);
	}

	notify(event, data) {
		for (const fn of this.listeners) {
			fn(event, data);
		}
	}
}
