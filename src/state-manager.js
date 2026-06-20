// StateManager: manages unified workspace tree and pagination logic
//
// Each page = 2 workspace chunks (row1 + row2)
// Each chunk ≤ 5 agents

export class StateManager {
	constructor() {
		this.unified = [];
		this.listeners = [];
	}

	// Initialize or update the unified workspace list
	// unifiedWorkspaces: UnifiedWorkspace[] from mock or real data
	init(unifiedWorkspaces) {
		this.unified = unifiedWorkspaces;
		this.notify("stateChanged");
	}

	// Compute paginated view
	// returns { pages: [[chunk, chunk?], ...] }
	computePages() {
		// Each WS is split into ≤5 agent slices (chunks)
		const chunks = [];
		for (const ws of this.unified) {
			const slices = this.sliceArray(ws.agents, 5);
			for (let i = 0; i < slices.length; i++) {
				chunks.push({
					...ws,
					agents: slices[i],
					chunkIndex: i,
					totalChunks: slices.length,
					isPartial: slices.length > 1, // being split across pages
				});
			}
		}
		// Group into pages: 2 chunks per page
		return this.sliceArray(chunks, 2);
	}

	// Get a specific page
	getPage(pageIndex) {
		const pages = this.computePages();
		if (pageIndex < 0 || pageIndex >= pages.length) return null;
		const page = pages[pageIndex];
		return {
			page: pageIndex,
			totalPages: pages.length,
			row1: page[0] || null, // K1-K5 chunk
			row2: page[1] || null, // K6-K10 chunk
		};
	}

	// Global stats across ALL workspaces
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

	sliceArray(arr, size) {
		const result = [];
		for (let i = 0; i < arr.length; i += size) {
			result.push(arr.slice(i, i + size));
		}
		return result;
	}
}
