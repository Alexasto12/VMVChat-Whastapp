package main

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/websocket"
)

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func newHub() *Hub {
	return &Hub{clients: map[*websocket.Conn]bool{}}
}

func (h *Hub) handle(c *websocket.Conn) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		c.Close()
	}()
	// El envío va por POST /send; aquí solo mantenemos viva la conexión.
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}
}

// broadcast a todos los clientes. // ponytail: sin filtro por chat; añádelo si tienes muchos usuarios.
func (h *Hub) broadcast(m Message) {
	env := struct {
		Type string `json:"type"`
		Message
	}{"message", m}
	data, _ := json.Marshal(env)
	h.send(data)
}

// broadcastStatus avisa de un cambio de estado de un mensaje saliente (delivered/read/failed).
func (h *Hub) broadcastStatus(id int64, status string) {
	data, _ := json.Marshal(map[string]any{"type": "status", "id": id, "status": status})
	h.send(data)
}

func (h *Hub) send(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			c.Close()
			delete(h.clients, c)
		}
	}
}
