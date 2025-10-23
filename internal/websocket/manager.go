package websocket

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// DAGStateEvent represents a DAG state change event
type DAGStateEvent struct {
	Type          string                 `json:"type"`
	ApplicationID string                 `json:"applicationId"`
	NodeID        string                 `json:"nodeId,omitempty"`
	NodeState     string                 `json:"nodeState,omitempty"`
	Timestamp     int64                  `json:"timestamp"`
	Data          map[string]interface{} `json:"data,omitempty"`
}

// Client represents a WebSocket client connection
type Client struct {
	ID           string
	Conn         *websocket.Conn
	Send         chan DAGStateEvent
	ApplicationID string
}

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from the clients
	broadcast chan DAGStateEvent

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe operations
	mutex sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan DAGStateEvent),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("WebSocket client registered: %s for app %s", client.ID, client.ApplicationID)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mutex.Unlock()
			log.Printf("WebSocket client unregistered: %s", client.ID)

		case event := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				// Only send to clients subscribed to this application
				if client.ApplicationID == event.ApplicationID {
					select {
					case client.Send <- event:
					default:
						delete(h.clients, client)
						close(client.Send)
					}
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// BroadcastDAGStateChange broadcasts a DAG state change to all relevant clients
func (h *Hub) BroadcastDAGStateChange(event DAGStateEvent) {
	select {
	case h.broadcast <- event:
	default:
		log.Printf("Failed to broadcast DAG state change: channel full")
	}
}

// GetClientCount returns the number of connected clients for a specific application
func (h *Hub) GetClientCount(applicationID string) int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	
	count := 0
	for client := range h.clients {
		if client.ApplicationID == applicationID {
			count++
		}
	}
	return count
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin in development
		// In production, you should implement proper origin checking
		return true
	},
}

// HandleWebSocket handles WebSocket connections
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request, applicationID, clientID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		ID:            clientID,
		Conn:          conn,
		Send:          make(chan DAGStateEvent, 256),
		ApplicationID: applicationID,
	}

	h.register <- client

	// Start goroutines for reading and writing
	go h.writePump(client)
	go h.readPump(client)
}

// writePump pumps messages from the hub to the websocket connection
func (h *Hub) writePump(client *Client) {
	defer func() {
		client.Conn.Close()
	}()

	for {
		select {
		case event, ok := <-client.Send:
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			client.Conn.WriteJSON(event)
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (h *Hub) readPump(client *Client) {
	defer func() {
		h.unregister <- client
		client.Conn.Close()
	}()

	for {
		var message map[string]interface{}
		err := client.Conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming messages if needed
		log.Printf("Received message from client %s: %v", client.ID, message)
	}
}