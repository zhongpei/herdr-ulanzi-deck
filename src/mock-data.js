// Mock data for deck display testing
// Simulates multiple machines, workspaces, and agents

export const mockConnections = [
	{ name: "local", abbr: "LCL" },
	{ name: "dev-server", abbr: "DEV" },
];

export function buildMockUnifiedWorkspaces() {
	return [
		// ── LCL: 4 workspaces ──
		{
			connName: "local",
			connAbbr: "LCL",
			connAbbrColor: "#4ADE80",
			workspace_id: "ws-1",
			label: "main-proj",
			agents: buildMockAgents(
				3, ["pi", "cursor", "pi"],
				["working", "blocked", "idle"],
				["review", "fix-bug", "idle"],
			),
		},
		{
			connName: "local",
			connAbbr: "LCL",
			connAbbrColor: "#4ADE80",
			workspace_id: "ws-2",
			label: "web-app",
			agents: buildMockAgents(
				2, ["claude", "pi"],
				["done", "working"],
				["api-done", "feat-auth"],
			),
		},
		{
			connName: "local",
			connAbbr: "LCL",
			connAbbrColor: "#4ADE80",
			workspace_id: "ws-5",
			label: "alpha-svc",
			agents: buildMockAgents(
				2, ["kimi", "grok"],
				["idle", "working"],
				["waiting", "research"],
			),
		},
		{
			connName: "local",
			connAbbr: "LCL",
			connAbbrColor: "#4ADE80",
			workspace_id: "ws-6",
			label: "tools",
			agents: buildMockAgents(
				1, ["codex"], ["done"], ["fmt-complete"],
			),
		},
		// ── DEV: 5 workspaces ──
		{
			connName: "dev-server",
			connAbbr: "DEV",
			connAbbrColor: "#60A5FA",
			workspace_id: "ws-3",
			label: "backend",
			agents: buildMockAgents(
				4, ["gemini", "copilot", "devin", "cursor"],
				["idle", "working", "blocked", "done"],
				["waiting", "deploy", "test-fail", "done"],
			),
		},
		{
			connName: "dev-server",
			connAbbr: "DEV",
			connAbbrColor: "#60A5FA",
			workspace_id: "ws-4",
			label: "infra",
			agents: buildMockAgents(1, ["cline"], ["working"], ["tf-plan"]),
		},
		{
			connName: "dev-server",
			connAbbr: "DEV",
			connAbbrColor: "#60A5FA",
			workspace_id: "ws-7",
			label: "staging",
			agents: buildMockAgents(
				3, ["opencode", "kilo", "amp"],
				["idle", "idle", "blocked"],
				["standby", "standby", "oom-kill"],
			),
		},
		{
			connName: "dev-server",
			connAbbr: "DEV",
			connAbbrColor: "#60A5FA",
			workspace_id: "ws-8",
			label: "mobile",
			agents: buildMockAgents(
				2, ["claude", "cursor"],
				["working", "done"],
				["ui-review", "build-fix"],
			),
		},
		{
			connName: "dev-server",
			connAbbr: "DEV",
			connAbbrColor: "#60A5FA",
			workspace_id: "ws-9",
			label: "api-gw",
			agents: buildMockAgents(
				2, ["devin", "gemini"],
				["working", "idle"],
				["rate-limit", "waiting"],
			),
		},
		// ── PRD: 2 workspaces ──
		{
			connName: "prod",
			connAbbr: "PRD",
			connAbbrColor: "#F87171",
			workspace_id: "ws-10",
			label: "prod-site",
			agents: buildMockAgents(
				3, ["pi", "cursor", "pi"],
				["done", "working", "idle"],
				["deployed", "monitor", "standby"],
			),
		},
		{
			connName: "prod",
			connAbbr: "PRD",
			connAbbrColor: "#F87171",
			workspace_id: "ws-11",
			label: "monitoring",
			agents: buildMockAgents(1, ["grok"], ["blocked"], ["alert-45"]),
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

// All agents for stats count (stats function only, not used in main flow)
export function buildMockUnifiedWorkspacesForStats() {
	return [
		{
			connName: "local",
			connAbbr: "LCL",
			connAbbrColor: "#4ADE80",
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
			connAbbrColor: "#4ADE80",
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
			connAbbrColor: "#60A5FA",
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
