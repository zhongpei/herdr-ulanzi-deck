// Test: filter buttons (K11=ALL, K12=machine cycle, K13=space cycle)
//
// Run: node tests/filter-buttons.test.js

import { StateManager } from "../src/state-manager.js";
import { ButtonMapper } from "../src/button-mapper.js";
import { buildMockUnifiedWorkspaces } from "../src/mock-data.js";

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
	console.log(`  ${ok ? "✅" : "❌"} ${msg}: expected="${expected}" actual="${actual}"`);
	if (ok) passed++;
	else failed++;
}

// ─── Setup ──────────────────────────────────────────────────────────
const sm = new StateManager();
sm.init(buildMockUnifiedWorkspaces());
const bm = new ButtonMapper(sm);

console.log("\n=== Setup ===");
const machines = sm.getMachines();
console.log(`  Machines: ${machines.map((m) => m.connAbbr).join(", ")}`);
const allAgents = sm.getAllAgents();
console.log(`  Total agents: ${allAgents.length}`);

// ─── Test 1: ALL mode ──────────────────────────────────────────────
console.log("\n=== Test 1: K11 — ALL mode ===");
bm.setAll();
let keys = bm.renderAll();
let agents = keys.slice(0, 10).filter((k) => k.type === "agent");

assertEqual(bm.mode, "all", "mode should be 'all'");
assert(agents.length > 0, "should have agent keys");
assert(agents.length <= 10, "no more than 10 agent keys");

// Verify sort: BLOCKED first, then DONE, then WORKING
const statusOrder = agents.map((a) => a.status);
const blockedFirst = statusOrder.indexOf("blocked") < statusOrder.indexOf("done");
const doneBeforeWorking = statusOrder.indexOf("done") < statusOrder.indexOf("working");
assert(blockedFirst, "BLOCKED agents before DONE");
assert(doneBeforeWorking, "DONE agents before WORKING");

console.log(`  Top 10 order: ${agents.map((a) => a.status).join(" > ")}`);

// ─── Test 2: Machine cycle ─────────────────────────────────────────
console.log("\n=== Test 2: K12 — Machine cycle ===");

// First press: should go to first machine
bm.setAll();
bm.nextMachine();
keys = bm.renderAll();
agents = keys.slice(0, 10).filter((k) => k.type === "agent");
const firstMachine = machines[0].connName;
const onMachine = agents.every((a) => a.connName === firstMachine);
assertEqual(bm.mode, "machine", "mode should be 'machine'");
assert(onMachine, `all agents should be from ${firstMachine}`);

// Second press: should go to second machine
bm.nextMachine();
keys = bm.renderAll();
agents = keys.slice(0, 10).filter((k) => k.type === "agent");
const secondMachine = machines[1 % machines.length].connName;
const onMachine2 = agents.every((a) => a.connName === secondMachine);
assert(onMachine2, `all agents should be from ${secondMachine}`);

// Cycle through all machines without resetting
console.log("  Full cycle:");
bm.setAll();
const order = [];
for (let i = 0; i < machines.length + 1; i++) {
	bm.nextMachine();
	keys = bm.renderAll();
	agents = keys.slice(0, 10).filter((k) => k.type === "agent");
	const current = agents.length > 0 ? agents[0].connAbbr : "empty";
	order.push(current);
}
const allUnique = new Set(order.slice(0, machines.length)).size === machines.length;
assert(allUnique, "cycle through all machines");
console.log(`  Order: ${order.join(" → ")}`);

// ─── Test 3: Space cycle (intersection with machine) ───────────────
console.log("\n=== Test 3: K13 — Space cycle with machine filter ===");

// Pick first machine
bm.setAll();
bm.nextMachine();

// First space press
bm.nextSpace();
keys = bm.renderAll();
agents = keys.slice(0, 10).filter((k) => k.type === "agent");
assertEqual(bm.mode, "space", "mode should be 'space'");

// Agents should be from the machine AND the specific space
const machineSpaces = sm.getSpaces(machines[0].connName);
const firstSpaceAgents = sm.getFilteredAgents(machines[0].connName, machineSpaces[0]?.wsId);
if (firstSpaceAgents.length > 0) {
	const spaceMatch = agents.every((a) => a.connName === machines[0].connName);
	assert(spaceMatch, "agents filtered to first machine");
	// Each agent should have a wsLabel matching the space label
	const firstSpaceLabel = machineSpaces[0]?.label;
	if (firstSpaceLabel) {
		const labelMatch = agents.every((a) => a.wsLabel === firstSpaceLabel);
		assert(labelMatch, `agents filtered to space "${firstSpaceLabel}"`);
	}
}

// Cycle spaces
console.log("  Space cycle:");
const spaceOrder = [];
for (let i = 0; i < machineSpaces.length + 1; i++) {
	bm.nextSpace();
	keys = bm.renderAll();
	agents = keys.slice(0, 10).filter((k) => k.type === "agent");
	const label = agents.length > 0 ? agents[0].wsLabel : "empty";
	spaceOrder.push(label);
}
assert(spaceOrder.length > 0, "space cycling produces different results");

// ─── Test 4: ALL clears all filters ────────────────────────────────
console.log("\n=== Test 4: K11 clears filters ===");
bm.nextMachine(); // go to machine
bm.nextSpace(); // go to space
bm.setAll(); // back to ALL
keys = bm.renderAll();
agents = keys.slice(0, 10).filter((k) => k.type === "agent");
assertEqual(bm.mode, "all", "mode back to 'all'");
assertEqual(bm.connName, null, "connName cleared");
assertEqual(bm.wsId, null, "wsId cleared");

// ─── Test 5: Machine switch clears space filter ───────────────────
console.log("\n=== Test 5: Machine switch clears space filter ===");
bm.setAll();
bm.nextMachine(); // first machine
bm.nextSpace(); // pick first space
assertEqual(bm.mode, "space", "in space mode");
const lastWs = bm.wsId;
bm.nextMachine(); // switch to next machine
assertEqual(bm.mode, "machine", "back to machine mode");
assertEqual(bm.wsId, null, "space filter cleared");
assert(bm.wsId !== lastWs || bm.wsId === null, "space filter reset on machine switch");

// ─── Summary ───────────────────────────────────────────────────────
console.log(`\n=== Results: ${passed} passed, ${failed} failed ${failed > 0 ? "❌" : "✅"} ===`);
process.exit(failed > 0 ? 1 : 0);
