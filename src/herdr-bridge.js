// HerdrBridge: reads herdr data → UnifiedWorkspace format
// Maps herdr workspaces + agents to our display format

import { HerdrClient } from "./herdr-client.js";

// Default machine config for local herdr
const LOCAL_MACHINE = {
	connName: "local",
	connAbbr: "LCL",
	connAbbrColor: "#4ADE80",
};

export class HerdrBridge {
	constructor(socketPath, machineConfig) {
		this.client = new HerdrClient(socketPath);
		this.machine = machineConfig || LOCAL_MACHINE;
	}

	async fetchUnifiedWorkspaces() {
		const [workspaces, agents] = await Promise.all([
			this.client.listWorkspaces(),
			this.client.listAgents(),
		]);

		// Build a map: workspace_id → list of agents
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

		// Build UnifiedWorkspace list
		const unified = [];
		for (const ws of workspaces) {
			unified.push({
				connName: this.machine.connName,
				connAbbr: this.machine.connAbbr,
				connAbbrColor: this.machine.connAbbrColor,
				workspace_id: ws.workspace_id,
				label: ws.label || `ws-${ws.number}`,
				number: ws.number,
				agent_status: ws.agent_status,
				tab_count: ws.tab_count,
				pane_count: ws.pane_count,
				agents: agentMap[ws.workspace_id] || [],
			});
		}

		return unified;
	}

	async subscribeToEvents(onEvent) {
		try {
			await this.client.subscribe(
				{
					subscriptions: [
						{ type: "pane.agent_status_changed" },
						{ type: "pane.agent_detected" },
						{ type: "workspace.created" },
						{ type: "workspace.closed" },
						{ type: "workspace.focused" },
						{ type: "workspace.renamed" },
					],
				},
				(msg) => {
					// Map herdr event → our event format
					const event = msg.event;
					const data = msg.data;
					if (event === "pane_agent_status_changed" && data) {
						onEvent({
							type: "agent_status_changed",
							paneId: data.pane_id,
							workspaceId: data.workspace_id,
							status: data.agent_status,
							agent: data.agent,
							customStatus: data.custom_status,
							stateLabels: data.state_labels,
						});
					}
					if (
						event === "workspace_created" ||
						event === "workspace_closed" ||
						event === "workspace_focused"
					) {
						onEvent({ type: "workspace_changed" });
					}
				},
			);
		} catch (err) {
			console.warn("[bridge] subscribe failed:", err.message);
		}
	}
}

// Agent status → our status (same names, just mapping for clarity)
export function mapStatus(s) {
	return s || "unknown";
}
