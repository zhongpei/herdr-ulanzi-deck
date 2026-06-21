// Package protocol defines shared data structures and NATS subject constants
// used by herdr-collector, herdr-deck, and herdr-pet.
//
// This module MUST remain small: only types, enums, and subject constants.
// No business logic, no I/O, no external dependencies.
package protocol

// AgentStatus represents the current agent lifecycle state.
type AgentStatus string

const (
	StatusIdle    AgentStatus = "idle"
	StatusWorking AgentStatus = "working"
	StatusBlocked AgentStatus = "blocked"
	StatusDone    AgentStatus = "done"
	StatusUnknown AgentStatus = "unknown"
)

// StatusPriority maps status to sort rank (lower = higher priority).
// Used by fleet manager for K1-K10 ordering.
var StatusPriority = map[AgentStatus]int{
	StatusBlocked: 0,
	StatusDone:    1,
	StatusWorking: 2,
	StatusIdle:    3,
	StatusUnknown: 4,
}
