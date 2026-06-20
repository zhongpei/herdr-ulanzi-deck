// ButtonMapper: maps state tree → 14 key render commands
//
// D200X layout:
//   row 0: K1-K5  (key_0_0 ~ key_0_4)  ← first WS chunk agents
//   row 1: K6-K10 (key_1_0 ~ key_1_4)  ← second WS chunk agents
//   row 2: K11 (key_2_0) K12 (key_2_1) K13 (key_2_2) K14 (key_2_3, wide)

export class ButtonMapper {
	constructor(stateManager) {
		this.state = stateManager;
		this.currentPage = 0;
	}

	setPage(n) {
		this.currentPage = n;
	}

	getCurrentPage() {
		return this.currentPage;
	}

	prevPage() {
		if (this.currentPage > 0) {
			this.currentPage--;
			return true;
		}
		return false;
	}

	nextPage() {
		const totalPages = this.state.computePages().length;
		if (this.currentPage < totalPages - 1) {
			this.currentPage++;
			return true;
		}
		return false;
	}

	// Returns 14 key render descriptors (one per physical key)
	renderAll() {
		const pageData = this.state.getPage(this.currentPage);
		const totalPages = this.state.computePages().length;
		const stats = this.state.computeStats();

		if (!pageData) {
			return this.renderEmpty();
		}

		const keys = [
			// Row 1: K1-K5 (indices 0-4)
			...this.renderAgentRow(pageData.row1, 0, 4),
			// Row 2: K6-K10 (indices 5-9)
			...this.renderAgentRow(pageData.row2, 5, 9),
			// Row 3: K11=navPrev, K12=navCurrent, K13=navNext
			this.renderNavPrev(pageData, this.currentPage),
			this.renderNavCurrent(pageData, this.currentPage, totalPages),
			this.renderNavNext(pageData, this.currentPage, totalPages),
			// Row 3 last: K14=stats (wide)
			this.renderStats(stats),
		];

		return keys;
	}

	renderEmpty() {
		const empty = [];
		for (let i = 0; i < 14; i++) {
			empty.push({ keyId: `empty_${i}`, type: "empty" });
		}
		return empty;
	}

	// Render K1-K5 or K6-K10
	renderAgentRow(wsChunk, startIdx, endIdx) {
		const keys = [];
		const agents = wsChunk ? wsChunk.agents : [];

		for (let i = 0; i < 5; i++) {
			const agent = agents[i];
			if (agent) {
				keys.push({
					keyId: `agent_${startIdx + i}`,
					type: "agent",
					agentType: agent.agent,
					alias: agent.name || agent.agent || "",
					status: agent.agent_status,
					focused: !!agent.focused,
					paneId: agent.pane_id,
					connName: wsChunk.connName,
					connAbbr: wsChunk.connAbbr,
					customStatus: agent.custom_status,
				});
			} else {
				keys.push({ keyId: `empty_${startIdx + i}`, type: "empty" });
			}
		}
		return keys;
	}

	// K11 — Previous page
	renderNavPrev(pageData, pageIdx) {
		// Get first WS label from previous page
		let label = "";
		const enabled = pageIdx > 0;

		if (enabled) {
			const prevPage = this.state.getPage(pageIdx - 1);
			if (prevPage && prevPage.row1) {
				label = `${prevPage.row1.connAbbr}:${prevPage.row1.label}`;
			}
		}

		return { keyId: "nav_prev", type: "navPrev", label, enabled };
	}

	// K12 — Current page WS info
	renderNavCurrent(pageData, pageIdx, totalPages) {
		const row1 = pageData.row1;
		const row2 = pageData.row2;

		if (row1 && !row2) {
			// Single WS page (workspace spans multiple pages)
			return {
				keyId: "nav_current",
				type: "navCurrent",
				singleWs: true,
				label: `${row1.connAbbr}:${row1.label}`,
				sublabel:
					row1.totalChunks > 1
						? `${row1.chunkIndex + 1}/${row1.totalChunks}`
						: "",
				pageLabel: `Page ${pageIdx + 1}/${totalPages}`,
			};
		} else if (row1 && row2) {
			// Dual WS page
			return {
				keyId: "nav_current",
				type: "navCurrent",
				singleWs: false,
				rows: [
					{ abbr: row1.connAbbr, label: row1.label },
					{ abbr: row2.connAbbr, label: row2.label },
				],
				pageLabel: `Page ${pageIdx + 1}/${totalPages}`,
			};
		} else {
			return {
				keyId: "nav_current",
				type: "navCurrent",
				singleWs: true,
				label: "",
				sublabel: "",
				pageLabel: "",
			};
		}
	}

	// K13 — Next page
	renderNavNext(pageData, pageIdx, totalPages) {
		let label = "";
		const enabled = pageIdx < totalPages - 1;

		if (enabled) {
			const nextPage = this.state.getPage(pageIdx + 1);
			if (nextPage && nextPage.row1) {
				label = `${nextPage.row1.connAbbr}:${nextPage.row1.label}`;
			}
		}

		return { keyId: "nav_next", type: "navNext", label, enabled };
	}

	// K14 — Stats bar
	renderStats(stats) {
		return { keyId: "stats", type: "stats", stats };
	}
}
