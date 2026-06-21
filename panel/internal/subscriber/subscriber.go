// Package subscriber subscribes to NATS subjects (snapshot + heartbeat) and
// delivers them to the panel via channels. Adapted from deck/internal/subscriber.
package subscriber

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/herdr-deck/herdrdeck/protocol"
	"github.com/nats-io/nats.go"
)

// Subscriber manages NATS subscriptions for the panel process.
type Subscriber struct {
	nc          *nats.Conn
	snapshotCh  chan *protocol.FleetSnapshot
	heartbeatCh chan time.Time
}

// New connects to a NATS server and starts subscriptions.
// Retries the initial connection so panel can start before collector/NATS.
func New(natsAddr string) (*Subscriber, error) {
	var nc *nats.Conn
	var err error

	for i := 0; i < 30; i++ {
		nc, err = nats.Connect(natsAddr,
			nats.RetryOnFailedConnect(true),
			nats.ReconnectWait(2*time.Second),
			nats.MaxReconnects(-1),
			nats.Timeout(1*time.Second),
		)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "format") ||
			strings.Contains(err.Error(), "no servers") && i > 5 {
			return nil, fmt.Errorf("connect: %w", err)
		}
		if i < 29 {
			time.Sleep(1 * time.Second)
		}
	}
	if err != nil || nc == nil {
		return nil, fmt.Errorf("connect after retry: %w", err)
	}

	s := &Subscriber{
		nc:          nc,
		snapshotCh:  make(chan *protocol.FleetSnapshot, 4),
		heartbeatCh: make(chan time.Time, 4),
	}

	if _, err := nc.Subscribe(protocol.SubjectSnapshot, func(msg *nats.Msg) {
		var snap protocol.FleetSnapshot
		if err := json.Unmarshal(msg.Data, &snap); err != nil {
			return
		}
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

	if _, err := nc.Subscribe(protocol.SubjectHeartbeat, func(msg *nats.Msg) {
		select {
		case s.heartbeatCh <- time.Now():
		default:
		}
	}); err != nil {
		nc.Close()
		return nil, err
	}

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
