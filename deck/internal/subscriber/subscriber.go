// Package subscriber subscribes to NATS subjects (snapshot + heartbeat) and
// delivers FleetSnapshot and heartbeat events to the deck via channels.
package subscriber

import (
	"encoding/json"
	"time"

	"github.com/herdr-deck/herdrdeck/protocol"
	"github.com/nats-io/nats.go"
)

// Subscriber manages NATS subscriptions for the deck process.
type Subscriber struct {
	nc       *nats.Conn
	snapshotCh chan *protocol.FleetSnapshot
	heartbeatCh chan time.Time
}

// New connects to a NATS server and starts subscriptions.
// With RetryOnFailedConnect, it returns a valid Subscriber even when NATS
// is not yet reachable — subscriptions are queued until connected.
func New(natsAddr string) (*Subscriber, error) {
	nc, err := nats.Connect(natsAddr,
		nats.RetryOnFailedConnect(true),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, err // still can fail for invalid addr format
	}

	s := &Subscriber{
		nc:          nc,
		snapshotCh:  make(chan *protocol.FleetSnapshot, 4),
		heartbeatCh: make(chan time.Time, 4),
	}

	// Subscribe to snapshot (drain channel → push latest)
	if _, err := nc.Subscribe(protocol.SubjectSnapshot, func(msg *nats.Msg) {
		var snap protocol.FleetSnapshot
		if err := json.Unmarshal(msg.Data, &snap); err != nil {
			return
		}
		// Drain any unconsumed snapshots, then push latest
		for {
			select {
			case <-s.snapshotCh:
			default:
				goto pushSnap
			}
		}
	pushSnap:
		s.snapshotCh <- &snap
	}); err != nil {
		nc.Close()
		return nil, err
	}

	// Subscribe to heartbeat
	if _, err := nc.Subscribe(protocol.SubjectHeartbeat, func(msg *nats.Msg) {
		select {
		case s.heartbeatCh <- time.Now():
		default:
		}
	}); err != nil {
		nc.Close()
		return nil, err
	}

	// Flush subscriptions to confirm they are registered on the server.
	// Ignores error when NATS is not yet connected (retry is ongoing).
	if err := nc.FlushTimeout(2 * time.Second); err != nil && nc.Status() != nats.CONNECTING {
		nc.Close()
		return nil, err
	}

	return s, nil
}

// SnapshotCh returns the channel for receiving FleetSnapshot updates.
func (s *Subscriber) SnapshotCh() <-chan *protocol.FleetSnapshot {
	return s.snapshotCh
}

// HeartbeatCh returns the channel for receiving heartbeat events.
func (s *Subscriber) HeartbeatCh() <-chan time.Time {
	return s.heartbeatCh
}

// Close drains and closes the NATS connection.
func (s *Subscriber) Close() {
	if s.nc != nil {
		s.nc.Drain()
		s.nc.Close()
	}
}
