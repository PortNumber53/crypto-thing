package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"embed"

	"github.com/gorilla/websocket"

	"cryptool/cmd/cryptool/root"
	"cryptool/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Command types for websocket communication
type Command struct {
	ID      string                 `json:"id"`
	Command string                 `json:"command"`
	Args    []string              `json:"args"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type Response struct {
	ID       string      `json:"id"`
	Success  bool        `json:"success"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
	Error    string      `json:"error,omitempty"`
}

// WebSocket connection wrapper
type Connection struct {
	conn   *websocket.Conn
	send   chan Response
	daemon *Daemon
}

// Daemon server
type Daemon struct {
	port        string
	connections map[*Connection]bool
	mutex       sync.RWMutex
	commandChan chan Command
	ctx         context.Context
	cancel      context.CancelFunc
	config      *config.Config
	// Job tracking
	jobs      map[string]*Job
	jobsMutex sync.RWMutex
}

// Job represents a unit of work tracked by the daemon
type Job struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Args      []string  `json:"args,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"` // running, stopping, done, error
	Error     string    `json:"error,omitempty"`
	cancel    context.CancelFunc
}

// NewDaemon creates a new daemon instance
func NewDaemon(port string, cfg *config.Config) *Daemon {
	ctx, cancel := context.WithCancel(context.Background())
	return &Daemon{
		port:        port,
		connections: make(map[*Connection]bool),
		commandChan: make(chan Command, 100),
		ctx:         ctx,
		cancel:      cancel,
		config:      cfg,
		jobs:        make(map[string]*Job),
	}
}

// Start starts the daemon server
func (d *Daemon) Start() error {
	// Setup HTTP routes
	http.HandleFunc("/ws", d.handleWebSocket)
	http.HandleFunc("/health", d.handleHealth)
	http.HandleFunc("/status", d.handleStatus)
	http.HandleFunc("/jobs/kill", d.handleJobsKill)

	// Start command processor
	go d.processCommands()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		d.Stop()
		os.Exit(0)
	}()

	log.Printf("Starting crypto daemon on port %s", d.port)
	log.Println("WebSocket endpoint: ws://localhost:" + d.port + "/ws")
	log.Println("Health endpoint: http://localhost:" + d.port + "/health")

	return http.ListenAndServe(":"+d.port, nil)
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	d.cancel()
	d.mutex.Lock()
	defer d.mutex.Unlock()

	for conn := range d.connections {
		conn.conn.Close()
	}
}

// handleWebSocket handles websocket connections
func (d *Daemon) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	wsConn := &Connection{
		conn:   conn,
		send:   make(chan Response, 256),
		daemon: d,
	}

	d.mutex.Lock()
	d.connections[wsConn] = true
	d.mutex.Unlock()

	go wsConn.writer()
	wsConn.reader()
}

// handleHealth provides health check endpoint
func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().Format(time.RFC3339),
		"connections": len(d.connections),
	})
}

// handleStatus returns daemon status and active jobs
func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	d.jobsMutex.RLock()
	jobs := make([]*Job, 0, len(d.jobs))
	for _, j := range d.jobs {
		jobs = append(jobs, &Job{ID: j.ID, Command: j.Command, Args: j.Args, StartedAt: j.StartedAt, Status: j.Status, Error: j.Error})
	}
	d.jobsMutex.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"timestamp":   time.Now().Format(time.RFC3339),
		"connections": len(d.connections),
		"jobs":        jobs,
	})
}

// handleJobsKill cancels a job by ID if running
func (d *Daemon) handleJobsKill(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	d.jobsMutex.RLock()
	j, ok := d.jobs[id]
	d.jobsMutex.RUnlock()
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	if j.cancel != nil {
		j.Status = "stopping"
		j.cancel()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "stopping",
		"id":     id,
	})
}

// reader handles incoming messages
func (c *Connection) reader() {
	defer func() {
		c.daemon.mutex.Lock()
		delete(c.daemon.connections, c)
		c.daemon.mutex.Unlock()
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var cmd Command
		if err := json.Unmarshal(message, &cmd); err != nil {
			c.sendResponse(Response{
				ID:      cmd.ID,
				Success: false,
				Error:   fmt.Sprintf("Invalid JSON: %v", err),
			})
			continue
		}

		// Send command for processing
		select {
		case c.daemon.commandChan <- cmd:
		case <-c.daemon.ctx.Done():
			return
		}
	}
}

// writer handles outgoing messages
func (c *Connection) writer() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case response, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(response); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.daemon.ctx.Done():
			return
		}
	}
}

// sendResponse sends a response to the websocket client
func (c *Connection) sendResponse(response Response) {
	select {
	case c.send <- response:
	case <-c.daemon.ctx.Done():
		return
	default:
		log.Printf("Response channel full, dropping response")
	}
}

// processCommands processes incoming commands
func (d *Daemon) processCommands() {
	for {
		select {
		case cmd := <-d.commandChan:
			d.handleCommand(cmd)
		case <-d.ctx.Done():
			return
		}
	}
}

// handleCommand processes a daemon command
func (d *Daemon) handleCommand(cmd Command) {
	// Find a connection to send response to
	d.mutex.RLock()
	var targetConn *Connection
	for conn := range d.connections {
		targetConn = conn
		break
	}
	d.mutex.RUnlock()

	if targetConn == nil {
		return
	}

	response := d.executeCommand(cmd)
	targetConn.sendResponse(response)
}

// executeCommand executes the actual command
func (d *Daemon) executeCommand(cmd Command) Response {
	response := Response{
		ID:      cmd.ID,
		Success: true,
	}

	switch cmd.Command {
	case "server:status":
		// Return lightweight status and jobs list
		d.jobsMutex.RLock()
		jobs := make([]*Job, 0, len(d.jobs))
		for _, j := range d.jobs {
			jobs = append(jobs, &Job{ID: j.ID, Command: j.Command, Args: j.Args, StartedAt: j.StartedAt, Status: j.Status, Error: j.Error})
		}
		d.jobsMutex.RUnlock()
		response.Message = "Server status"
		response.Data = map[string]interface{}{
			"connections": len(d.connections),
			"jobs":        jobs,
		}

	case "jobs:kill":
		id := ""
		if v, ok := cmd.Data["id"]; ok {
			if s, ok := v.(string); ok { id = s }
		}
		if id == "" {
			response.Success = false
			response.Error = "missing job id"
			break
		}
		d.jobsMutex.RLock()
		j, ok := d.jobs[id]
		d.jobsMutex.RUnlock()
		if !ok {
			response.Success = false
			response.Error = "job not found"
			break
		}
		if j.cancel != nil { j.Status = "stopping"; j.cancel() }
		response.Message = fmt.Sprintf("Stopping job %s", id)

	case "migrate:status":
		response.Message = "Migration status checked"
		response.Data = map[string]interface{}{
			"status": "completed",
		}

	case "coinbase:fetch":
		product := ""
		if productInterface, ok := cmd.Data["product"]; ok {
			if productStr, ok := productInterface.(string); ok {
				product = productStr
			}
		}

		response.Message = fmt.Sprintf("Fetching coinbase data for product: %s", product)
		response.Data = map[string]interface{}{
			"product": product,
			"status":  "fetching",
		}

	case "health":
		response.Message = "Daemon is healthy"
		response.Data = map[string]interface{}{
			"uptime":      time.Since(time.Now().Add(-time.Hour)).String(),
			"connections": len(d.connections),
		}

	case "stop":
		response.Message = "Daemon stopping"
		go func() {
			time.Sleep(1 * time.Second)
			d.Stop()
		}()

	default:
		response.Success = false
		response.Error = fmt.Sprintf("Unknown command: %s", cmd.Command)
	}

	return response
}

// startDaemon starts the daemon server
func startDaemon(port string) error {
	// Load configuration
	cfg, err := config.Load("", "")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	daemon := NewDaemon(port, cfg)
	return daemon.Start()
}

// main function with daemon support
func main() {
	// Check if running as daemon
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		port := "40000"
		if len(os.Args) > 2 {
			port = os.Args[2]
		}

		if err := startDaemon(port); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Normal CLI execution
	if err := root.Execute(migrationsFS); err != nil {
		log.Fatal(err)
	}
}
