// Test: HerdrClient — Unix socket and TCP request/response
//
// Run: node tests/herdr-client.test.js

import net from "net";
import { HerdrClient } from "../src/herdr-client.js";

let passed = 0;
let failed = 0;

function assert(condition, msg) {
	if (condition) {
		console.log(`  ✅ ${msg}`);
		passed++;
	} else {
		console.log(`  ❌ ${msg}`);
		failed++;
	}
}

function assertEqual(actual, expected, msg) {
	const ok = actual === expected;
	console.log(
		`  ${ok ? "✅" : "❌"} ${msg}: expected=${JSON.stringify(expected)} actual=${JSON.stringify(actual)}`,
	);
	if (ok) passed++;
	else failed++;
}

// ─── Helper: start a mock JSON-line server ─────────────────────
function startMockServer(port, response) {
	return new Promise((resolve, reject) => {
		const server = net.createServer((sock) => {
			sock.on("data", (data) => {
				const reqStr = data.toString().trim();
				let resp;
				if (typeof response === "function") {
					resp = response(JSON.parse(reqStr));
				} else {
					resp = response;
				}
				sock.write(JSON.stringify(resp) + "\n");
				sock.end();
			});
		});
		server.listen(port, "127.0.0.1", () => resolve(server));
		server.on("error", reject);
	});
}

// ─── Test 1: TCP request gets response ─────────────────────────
console.log("\n=== Test 1: TCP request gets response ===");
let server1;
try {
	const expectedWorkspaces = [
		{ workspace_id: "ws-1", label: "test-ws", number: 1, pane_count: 2 },
	];
	server1 = await startMockServer(19994, {
		id: "test:1",
		result: { workspaces: expectedWorkspaces },
	});

	const client = new HerdrClient({ host: "127.0.0.1", port: 19994 });
	const resp = await client.request("workspace.list", {});
	assertEqual(resp.result.workspaces.length, 1, "should return 1 workspace");
	assertEqual(
		resp.result.workspaces[0].workspace_id,
		"ws-1",
		"workspace_id matches",
	);
} finally {
	if (server1) server1.close();
}

// ─── Test 2: listWorkspaces convenience method ──────────────────
console.log("\n=== Test 2: listWorkspaces() convenience method ===");
let server2;
try {
	server2 = await startMockServer(19995, {
		id: "test:2",
		result: {
			workspaces: [
				{ workspace_id: "ws-a", label: "project-a", number: 1, pane_count: 1 },
				{ workspace_id: "ws-b", label: "project-b", number: 2, pane_count: 3 },
			],
		},
	});
	const client = new HerdrClient({ host: "127.0.0.1", port: 19995 });
	const workspaces = await client.listWorkspaces();
	assertEqual(workspaces.length, 2, "should return 2 workspaces");
	assertEqual(workspaces[0].workspace_id, "ws-a", "first workspace id");
} finally {
	if (server2) server2.close();
}

// ─── Test 3: listAgents convenience method ──────────────────────
console.log("\n=== Test 3: listAgents() convenience method ===");
let server3;
try {
	server3 = await startMockServer(19996, {
		id: "test:3",
		result: {
			agents: [
				{ pane_id: "p1", agent: "pi", agent_status: "working" },
				{ pane_id: "p2", agent: "claude", agent_status: "idle" },
			],
		},
	});
	const client = new HerdrClient({ host: "127.0.0.1", port: 19996 });
	const agents = await client.listAgents();
	assertEqual(agents.length, 2, "should return 2 agents");
	assertEqual(agents[0].agent, "pi", "first agent is pi");
} finally {
	if (server3) server3.close();
}

// ─── Test 4: Error response handling ────────────────────────────
console.log("\n=== Test 4: Error response handling ===");
let server4;
try {
	server4 = await startMockServer(19997, {
		id: "test:4",
		error: { code: -1, message: "method not found" },
	});
	const client = new HerdrClient({ host: "127.0.0.1", port: 19997 });
	try {
		await client.request("unknown.method", {});
		assert(false, "should have thrown");
	} catch (err) {
		assert(err.message.includes("method not found"), `error message matches: "${err.message}"`);
	}
} finally {
	if (server4) server4.close();
}

// ─── Test 5: Connection refused (no server) ────────────────────
console.log("\n=== Test 5: Connection refused error ===");
const client = new HerdrClient({ host: "127.0.0.1", port: 19998 });
try {
	await client.request("workspace.list", {});
	assert(false, "should have thrown on connection refused");
} catch (err) {
	assert(err.code === "ECONNREFUSED" || err.message.includes("connect"),
		`connection error: code=${err.code} msg="${err.message?.slice(0, 60)}"`);
}

// ─── Summary ────────────────────────────────────────────────────
console.log(
	`\n=== Results: ${passed} passed, ${failed} failed ${failed > 0 ? "❌" : "✅"} ===`,
);
process.exit(failed > 0 ? 1 : 0);
