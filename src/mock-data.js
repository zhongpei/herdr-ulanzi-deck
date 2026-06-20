// Mock data for deck display testing
// Simulates multiple machines, workspaces, and agents

export const mockConnections = [
	{ name: "local", abbr: "LCL" },
	{ name: "dev-server", abbr: "DEV" },
];

export function buildMockUnifiedWorkspaces() {
	return [
		// LCL machine - 2 workspaces
		{
			connName: "local",
			connAbbr: "LCL",
			workspace_id: "ws-1",
			label: "main-proj",
			agents: buildMockAgents(
				3,
				["pi", "cursor", "pi"],
				["working", "blocked", "idle"],
				["review", "fix-bug", "idle"],
			),
		},
		{
			connName: "local",
			connAbbr: "LCL",
			workspace_id: "ws-2",
			label: "web-app",
			agents: buildMockAgents(
				2,
				["claude", "pi"],
				["done", "working"],
				["api-done", "feat-auth"],
			),
		},
		// DEV machine - 1 workspace with many agents
		{
			connName: "dev-server",
			connAbbr: "DEV",
			workspace_id: "ws-3",
			label: "backend",
			agents: buildMockAgents(
				4,
				["gemini", "copilot", "devin", "cursor"],
				["idle", "working", "blocked", "done"],
				["waiting", "deploy", "test-fail", "done"],
			),
		},
		{
			connName: "dev-server",
			connAbbr: "DEV",
			workspace_id: "ws-4",
			label: "infra",
			agents: buildMockAgents(1, ["cline"], ["working"], ["tf-plan"]),
		},
	];
}

function buildMockAgents(count, types, statuses, names) {
	const agents = [];
	for (let i = 0; i < count; i++) {
		agents.push({
			pane_id: `pane-${Math.random().toString(36).slice(2, 8)}`,
			terminal_id: `term-${Math.random().toString(36).slice(2, 8)}`,
			workspace_id: "",
			tab_id: "tab-1",
			agent: types[i] || "unknown",
			name: names[i] || `agent-${i}`,
			agent_status: statuses[i] || "idle",
			custom_status: null,
			state_labels: {},
			title: null,
			display_agent: null,
			focused: i === 0,
			revision: 1,
		});
	}
	return agents;
}

// All agents for stats count
export function buildMockUnifiedWorkspacesForStats() {
	return [
		{
			connName: "local",
			connAbbr: "LCL",
			workspace_id: "ws-1",
			label: "main-proj",
			agents: [
				{ agent_status: "working", agent: "pi" },
				{ agent_status: "blocked", agent: "cursor" },
				{ agent_status: "idle", agent: "pi" },
			],
		},
		{
			connName: "local",
			connAbbr: "LCL",
			workspace_id: "ws-2",
			label: "web-app",
			agents: [
				{ agent_status: "done", agent: "claude" },
				{ agent_status: "working", agent: "pi" },
			],
		},
		{
			connName: "dev-server",
			connAbbr: "DEV",
			workspace_id: "ws-3",
			label: "backend",
			agents: [
				{ agent_status: "idle", agent: "gemini" },
				{ agent_status: "working", agent: "copilot" },
				{ agent_status: "blocked", agent: "devin" },
				{ agent_status: "done", agent: "cursor" },
				{ agent_status: "unknown", agent: "cline" },
			],
		},
	];
}
