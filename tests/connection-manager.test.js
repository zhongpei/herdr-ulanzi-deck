// Test: ConnectionManager — SSH tunnel, waitForPort, connection types
//
// Run: node tests/connection-manager.test.js

import net from "net";
import { waitForPort } from "../src/connection-manager.js";

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

// ─── Helper: start a local TCP echo server ──────────────────────
function startServer(port) {
	return new Promise((resolve, reject) => {
		const server = net.createServer((sock) => {
			sock.on("data", (data) => {
				// Echo back a JSON-line response
				const response =
					JSON.stringify({
						id: "test:resp",
						result: { ok: true },
					}) + "\n";
				sock.write(response);
			});
		});
		server.listen(port, "127.0.0.1", () => resolve(server));
		server.on("error", reject);
	});
}

// ─── Test 1: waitForPort succeeds on open port ──────────────────
console.log("\n=== Test 1: waitForPort succeeds on open port ===");
let server;
try {
	server = await startServer(19990);
	const result = await waitForPort(19990, 3000);
	assert(result === true, "waitForPort should return true when port is open");
} finally {
	if (server) server.close();
}

// ─── Test 2: waitForPort times out on closed port ───────────────
console.log("\n=== Test 2: waitForPort times out on closed port ===");
const result = await waitForPort(19991, 1000);
assert(result === false, "waitForPort should return false on timeout");

// ─── Test 3: waitForPort returns false on invalid port (0) ──────
console.log("\n=== Test 3: waitForPort on invalid port ===");
const result2 = await waitForPort(0, 500);
assert(result2 === false, "waitForPort should return false for port 0");

// ─── Test 4: waitForPort respects shorter timeout ──────────────
console.log("\n=== Test 4: waitForPort respects short timeout ===");
const start = Date.now();
await waitForPort(19992, 200);
const elapsed = Date.now() - start;
assert(elapsed < 500, `should timeout in ~200ms, took ${elapsed}ms`);

// ─── Test 5: waitForPort with port that opens later ─────────────
console.log(
	"\n=== Test 5: waitForPort detects port that opens after delay ===",
);
(async () => {
	// Start server after 300ms delay
	setTimeout(async () => {
		const s = await startServer(19993);
		// Close after test
		setTimeout(() => s.close(), 500);
	}, 300);
	const result = await waitForPort(19993, 5000);
	assert(result === true, "should detect port opened after delay");
})();

// Wait for deferred test to complete
await new Promise((r) => setTimeout(r, 1000));

// ─── Summary ────────────────────────────────────────────────────
console.log(
	`\n=== Results: ${passed} passed, ${failed} failed ${failed > 0 ? "❌" : "✅"} ===`,
);
process.exit(failed > 0 ? 1 : 0);
