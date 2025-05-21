package ryskcore

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Client handles WebSocket communication.
type Client struct {
	Connection *websocket.Conn
	Ctx        context.Context    // Context for the client's operations
	Disconnect context.CancelFunc // Call this to stop the client

	in      chan []byte          // Channel for messages to be processed by the handler
	out     chan interface{}     // Channel for messages to be sent to the WebSocket
	handler func(message []byte) // User-defined message handler
}

// NewClient creates and initializes a new WebSocket client.
// It establishes the WebSocket connection and starts internal goroutines for message handling.
// The client will shut down if the parentCtx is cancelled or if a critical error occurs.
func NewClient(parentCtx context.Context, urlStr string, requestHeader http.Header) (*Client, error) {
	conn, resp, err := websocket.DefaultDialer.Dial(urlStr, requestHeader)
	if err != nil {
		errMsg := fmt.Sprintf("failed to connect to WebSocket %s", urlStr)
		if resp != nil {
			errMsg = fmt.Sprintf("%s: status %d", errMsg, resp.StatusCode)
		}
		return nil, fmt.Errorf("%s: %w", errMsg, err)
	}

	clientCtx, clientCancel := context.WithCancel(parentCtx)

	c := &Client{
		Connection: conn,
		Ctx:        clientCtx,
		Disconnect: clientCancel,
		in:         make(chan []byte, 32),    // Buffered channel
		out:        make(chan interface{}, 32), // Buffered channel
		handler: func(msg []byte) { // Default handler
			log.Printf("Default handler: No user-specific handler set for client. Received: %s", string(msg))
		},
	}

	// Setup underlying connection handlers
	conn.SetPingHandler(func(appData string) error {
		log.Println("Ping received, sending Pong")
		err := conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(5*time.Second))
		if err != nil {
			log.Printf("Error sending pong: %v", err)
		}
		return nil
	})

	conn.SetPongHandler(func(appData string) error {
		log.Println("Pong received")
		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		log.Printf("Connection closed by peer: %d %s", code, text)
		c.Disconnect() // Trigger client shutdown
		return nil
	})

	go c.processOutboundMessages()
	go c.processInboundMessages()
	go c.readFromWebSocket() // New goroutine to read from WebSocket

	return c, nil
}

// SetHandler sets the message handler for incoming messages.
// This must be called to process messages received from the WebSocket.
func (c *Client) SetHandler(handler func([]byte)) {
	c.handler = handler
}

// Ingest allows external code to inject a message into the client's
// inbound processing queue, to be handled by the registered handler.
func (c *Client) Ingest(req []byte) {
	select {
	case c.in <- req:
	case <-c.Ctx.Done():
		log.Println("Client context done, cannot ingest message.")
	}
}

// Send queues a message to be sent over the WebSocket connection.
func (c *Client) Send(payload []byte) {
	select {
	case c.out <- payload:
	case <-c.Ctx.Done():
		log.Println("Client context done, cannot send message.")
	}
}

// processOutboundMessages handles sending messages from the 'out' channel to the WebSocket.
func (c *Client) processOutboundMessages() {
	for {
		select {
		case <-c.Ctx.Done():
			log.Println("processOutboundMessages: context done, shutting down.")
			_ = c.Connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		case msg := <-c.out:
			var err error
			switch m := msg.(type) {
			case []byte: // Assumed to be TextMessage
				err = c.Connection.WriteMessage(websocket.TextMessage, m)
			default:
				log.Printf("processOutboundMessages: unknown message type to send: %T", msg)
				continue
			}
			if err != nil {
				log.Printf("Error writing message: %v", err)
				c.Disconnect() // Critical error, shut down client
				return
			}
		}
	}
}

// processInboundMessages handles messages from the 'in' channel, passing them to the user-defined handler.
func (c *Client) processInboundMessages() {
	for {
		select {
		case <-c.Ctx.Done():
			log.Println("processInboundMessages: context done, shutting down.")
			return
		case req := <-c.in:
			if c.handler != nil {
				c.handler(req)
			} else {
				log.Println("processInboundMessages: no handler set, discarding message.")
			}
		}
	}
}

// readFromWebSocket reads messages from the WebSocket connection and passes them to Ingest.
func (c *Client) readFromWebSocket() {
	defer func() {
		log.Println("readFromWebSocket: stopping.")
	}()

	for {
		select {
		case <-c.Ctx.Done(): // Check context before blocking on ReadMessage
			log.Println("readFromWebSocket: context done, exiting read loop.")
			return
		default:
			if err := c.Connection.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				log.Printf("readFromWebSocket: error setting read deadline: %v", err)
				c.Disconnect()
				return
			}

			messageType, payload, err := c.Connection.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					log.Printf("readFromWebSocket: unexpected close error: %v", err)
				} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue // Read deadline exceeded, loop to check context
				} else if c.Ctx.Err() != nil {
					log.Println("readFromWebSocket: context cancelled during read.")
				} else {
					log.Printf("readFromWebSocket: error reading message: %v", err)
				}
				c.Disconnect()
				return
			}

			if err := c.Connection.SetReadDeadline(time.Time{}); err != nil {
				log.Printf("readFromWebSocket: error resetting read deadline: %v", err)
				c.Disconnect()
				return
			}

			if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
				c.Ingest(payload)
			} else if messageType == websocket.CloseMessage {
				log.Println("readFromWebSocket: received close message from peer.")
				return
			}
		}
	}
}

// Close sends a WebSocket close message and stops the client.
func (c *Client) Close() error {
	log.Println("Client.Close called, initiating shutdown.")
	c.Disconnect() // Signal all goroutines to stop

	err := c.Connection.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closing"),
		time.Now().Add(2*time.Second))

	if err != nil && err != websocket.ErrCloseSent {
		log.Printf("Error sending close message during Client.Close: %v", err)
	}

	return c.Connection.Close()
}
