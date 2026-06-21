// Package publisher sends FleetSnapshot and heartbeat on NATS.
package publisher

import (
	"encoding/json"
	"time"

	"github.com/herdr-deck/herdrdeck/protocol"
	"github.com/nats-io/nats.go"
)

// Publisher manages NATS connection and periodic publishing.
type Publisher struct {
	nc       *nats.Conn
	natsAddr string
}

// New connects to the embedded NATS server at addr (e.g. "nats://127.0.0.1:4222").
func New(natsAddr string) (*Publisher, error) {
	nc, err := nats.Connect(natsAddr,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(c *nats.Conn, err error) {
			// NATS disconnected — will auto-reconnect
		}),
	)
	if err != nil {
		return nil, err
	}
	return &Publisher{nc: nc, natsAddr: natsAddr}, nil
}

// PublishSnapshot marshals and publishes a FleetSnapshot on SubjectSnapshot.
func (p *Publisher) PublishSnapshot(snap *protocol.FleetSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return p.nc.Publish(protocol.SubjectSnapshot, data)
}

// PublishHeartbeat sends a lightweight heartbeat message on SubjectHeartbeat.
func (p *Publisher) PublishHeartbeat() error {
	data, _ := json.Marshal(map[string]string{
		"ts": time.Now().UTC().Format(time.RFC3339),
	})
	return p.nc.Publish(protocol.SubjectHeartbeat, data)
}

// Close drains and closes the NATS connection.
func (p *Publisher) Close() {
	if p.nc != nil {
		p.nc.Drain()
		p.nc.Close()
	}
}
