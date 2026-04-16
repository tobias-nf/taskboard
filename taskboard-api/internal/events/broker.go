package events

import (
	"encoding/json"
	"sync"
	"time"
)

// Event is a single SSE event pushed to agents.
type Event struct {
	Type      string `json:"event"`
	TaskID    string `json:"task_id"`
	Actor     string `json:"actor"`
	Data      any    `json:"data,omitempty"`
	Timestamp string `json:"timestamp"`
}

// Broker manages SSE connections for all agents.
type Broker struct {
	mu      sync.RWMutex
	clients map[string]map[chan string]struct{} // agentID → set of channels
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]map[chan string]struct{}),
	}
}

// Subscribe registers a channel for the given agent. Returns the channel and an unsubscribe func.
func (b *Broker) Subscribe(agentID string) (chan string, func()) {
	ch := make(chan string, 64)
	b.mu.Lock()
	if b.clients[agentID] == nil {
		b.clients[agentID] = make(map[chan string]struct{})
	}
	b.clients[agentID][ch] = struct{}{}
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		delete(b.clients[agentID], ch)
		if len(b.clients[agentID]) == 0 {
			delete(b.clients, agentID)
		}
		b.mu.Unlock()
		close(ch)
	}
	return ch, unsub
}

// Publish sends an event to all specified agent IDs.
func (b *Broker) Publish(agentIDs []string, evt Event) {
	if evt.Timestamp == "" {
		evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	msg := "event: " + evt.Type + "\ndata: " + string(data) + "\n\n"

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, id := range agentIDs {
		for ch := range b.clients[id] {
			select {
			case ch <- msg:
			default:
				// Client too slow, drop event
			}
		}
	}
}
