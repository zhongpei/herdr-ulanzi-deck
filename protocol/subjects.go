package protocol

// NATS subject constants for inter-process communication.
const (
	// SubjectSnapshot carries the full fleet snapshot (herdr.v1.snapshot.full).
	// Published by collector whenever herdr data changes (and every 2s as keep-alive).
	// Subscribed by deck and pet for display updates.
	SubjectSnapshot = "herdr.v1.snapshot.full"

	// SubjectHeartbeat is a lightweight liveness signal from collector (herdr.v1.collector.heartbeat).
	// Published every 1s. Consumers detect collector offline after 5s without heartbeat.
	SubjectHeartbeat = "herdr.v1.collector.heartbeat"
)
