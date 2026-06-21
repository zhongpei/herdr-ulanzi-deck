// Package command publishes agent focus commands to the collector via NATS.
//
// The panel uses this to request the collector to focus a specific agent
// (e.g., when the user double-clicks an agent card or presses Enter on a
// selected agent).
package command

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// SubjectFocus is the NATS subject for focus command messages.
// Collector subscribes to this subject and forwards the focus request to herdr.
const SubjectFocus = "herdr.v1.command.focus"

// FocusPayload is sent when the user wants to focus a specific agent.
type FocusPayload struct {
	AgentID string `json:"agent_id"` // "machineName|paneID"
	Machine string `json:"machine"`
	PaneID  string `json:"pane_id"`
}

// Publisher sends commands to the collector via NATS.
type Publisher struct {
	nc *nats.Conn
}

// New creates a command publisher connected to the given NATS address.
func New(natsAddr string) (*Publisher, error) {
	nc, err := nats.Connect(natsAddr,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("command publisher connect: %w", err)
	}
	return &Publisher{nc: nc}, nil
}

// PublishFocus sends a focus command for the specified agent.
func (p *Publisher) PublishFocus(agentID, machine, paneID string) error {
	payload := FocusPayload{
		AgentID: agentID,
		Machine: machine,
		PaneID:  paneID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.nc.Publish(SubjectFocus, data)
}

// Close closes the NATS connection.
func (p *Publisher) Close() {
	if p.nc != nil {
		p.nc.Close()
	}
}
