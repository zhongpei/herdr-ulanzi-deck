// HerdrBridge: reads herdr data → UnifiedWorkspace format
// Handles multiple machines (local + SSH tunnel)

import { HerdrClient } from "./herdr-client.js";

export class HerdrBridge {
	constructor() {
		this.clients = []; // { name, abbr, color, client }
	}

	// Add a connection from ConnectionManager
	addConnection(name, abbr, color, socketOrClient) {
		const client =
			typeof socketOrClient === "string"
				? new HerdrClient(socketOrClient)
				: socketOrClient;
		this.clients.push({ name, abbr, color, client });
	}

	// Fetch data from ALL connections and merge into UnifiedWorkspace[]
	async fetchAll() {
		const allWorkspaces = [];

		for (const conn of this.clients) {
			try {
				const [workspaces, agents] = await Promise.all([
					conn.client.listWorkspaces().catch(() => []),
					conn.client.listAgents().catch(() => []),
				]);
				// Build agent map per workspace
				const agentMap = {};
				for (const a of agents) {
					const wid = a.workspace_id;
					if (!agentMap[wid]) agentMap[wid] = [];
					agentMap[wid].push({
						pane_id: a.pane_id,
						terminal_id: a.terminal_id,
						workspace_id: a.workspace_id,
						tab_id: a.tab_id,
						agent: a.agent || "unknown",
						name: a.name || a.agent || "",
						agent_status: a.agent_status,
						custom_status: a.custom_status || null,
						state_labels: a.state_labels || {},
						title: a.title || null,
						display_agent: a.display_agent || null,
						focused: !!a.focused,
						revision: a.revision || 0,
					});
				}
				for (const ws of workspaces) {
					allWorkspaces.push({
						connName: conn.name,
						connAbbr: conn.abbr,
						connAbbrColor: conn.color,
						workspace_id: ws.workspace_id,
						label: ws.label || `ws-${ws.number}`,
						number: ws.number,
						agent_status: ws.agent_status,
						tab_count: ws.tab_count,
						pane_count: ws.pane_count,
						agents: agentMap[ws.workspace_id] || [],
					});
				}
			} catch (err) {
				console.warn(`[bridge] fetch failed for ${conn.name}: ${err.message}`);
			}
		}
		return allWorkspaces;
	}

	// Focus agent on a specific connection
	async focusAgent(connName, paneId) {
		const conn = this.clients.find((c) => c.name === connName);
		if (!conn) return;
		try {
			await conn.client.request("agent.focus", { target: paneId });
		} catch (err) {
			console.warn(`[bridge] focus failed: ${err.message}`);
		}
	}
}
