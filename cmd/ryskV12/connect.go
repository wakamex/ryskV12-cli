package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time" // Added to resolve undefined: time errors

	"github.com/goccy/go-json"
	"github.com/urfave/cli/v2"

	"github.com/wakamex/rysk-v12-cli/ryskcore" // Adjust if your fork's module path is different
)

// JsonRPCRequest defines the structure for JSON-RPC messages used in IPC.
type JsonRPCRequest struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

var connectAction = &cli.Command{
	Name:  "connect",
	Usage: "Instantiate a websocket connection and listen for local commands via Unix socket.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "channel_id",
			Required: true,
			Usage:    "A unique id for the Unix domain socket (e.g., /tmp/channel_id.sock)",
		},
		&cli.StringFlag{
			Name:     "url",
			Required: true,
			Usage:    "WebSocket URL to connect to (e.g., wss://api.rysk.finance/ws)",
		},
		// --role flag and X-Rysk-Role header functionality removed as per user request.
	},
	Action: func(c *cli.Context) error {
		return connectCmdFunc(c) // Renamed to avoid conflict
	},
}

func connectCmdFunc(c *cli.Context) error {
	channelID := c.String("channel_id")
	socketPath := fmt.Sprintf("/tmp/%s.sock", channelID)
	cmdChan := make(chan []byte)

	// Setup Unix domain socket listener for IPC
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		log.Printf("Error listening on Unix socket %s: %v", socketPath, err)
		return err
	}
	log.Printf("Listening for commands on %s", socketPath)
	defer func() {
		ln.Close()
		os.Remove(socketPath)
		log.Printf("Closed and removed Unix socket %s", socketPath)
	}()

	// Use the command's context for the ryskcore client's parent context
	// The ryskcore.Client will manage its own internal context derived from this.
	// X-Rysk-Role header functionality removed. Passing nil for headers.
	ryskClient, err := ryskcore.NewClient(c.Context, c.String("url"), nil)
	if err != nil {
		log.Printf("Failed to connect to WebSocket %s: %v", c.String("url"), err)
		return err
	}
	log.Printf("Successfully connected to WebSocket: %s", c.String("url"))

	// Set a handler for messages received from the WebSocket via ryskcore.Client
	ryskClient.SetHandler(func(msg []byte) {
		// Process or display messages from the WebSocket
		// For example, log them or forward to connected IPC clients if needed.
		fmt.Printf("Received from WebSocket: %s\n", string(msg))
	})

	// Start goroutine to accept commands from the Unix domain socket
	// Use c.Context for this goroutine as well, so it stops when the command context is done.
	go pipeCommands(c.Context, ln, cmdChan)

	log.Println("Connect command running. Waiting for IPC commands or context cancellation.")

	// Main loop for the connect command
	for {
		select {
		case <-c.Context.Done(): // Triggered by Ctrl+C or if ryskClient's parent context is cancelled
			log.Println("Connect command context done, initiating shutdown.")
			if err := ryskClient.Close(); err != nil {
				log.Printf("Error closing Rysk client: %v", err)
			}
			log.Println("Rysk client closed.")
			return nil
		case <-ryskClient.Ctx.Done(): // Triggered if the ryskClient itself shuts down (e.g. WebSocket error)
			log.Println("Rysk client context done, connect command shutting down.")
			// The main command context (c.Context) should also be cancelled or this will hang
			// if c.Context is not already being cancelled. This indicates an external shutdown of the client.
			return fmt.Errorf("rysk client shut down unexpectedly")

		case cmd, ok := <-cmdChan:
			if !ok {
				log.Println("Command channel closed, shutting down.")
				if err := ryskClient.Close(); err != nil {
					log.Printf("Error closing Rysk client: %v", err)
				}
				return nil
			}
			if strings.Contains(strings.ToLower(string(cmd)), "disconnect") {
				log.Println("Received 'disconnect' IPC command.")
				if err := ryskClient.Close(); err != nil {
					log.Printf("Error closing Rysk client on disconnect command: %v", err)
				}
				// Assuming the main context cancellation (Ctrl+C) is the primary way to stop.
				// Or, explicitly cancel c.Context if this command should terminate the connect process.
				return nil // Or call a cancel func if connectCmdFunc managed its own cancellable context
			} else {
				log.Printf("Relaying IPC command to WebSocket: %s", string(cmd))
				ryskClient.Send(cmd)
			}
		}
	}
}

// writeToSocket is used by other CLI commands (quote, transfer) to send data to the connect command's Unix socket.
func writeToSocket(channelID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("invalid payload for IPC: %w", err)
	}

	socketPath := fmt.Sprintf("/tmp/%s.sock", channelID)
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return fmt.Errorf("failed to connect to IPC socket %s: %w", socketPath, err)
	}
	defer conn.Close()

	_, err = conn.Write(append(data, '\n')) // Append newline for line-based reading by the scanner in pipeCommands.
	if err != nil {
		return fmt.Errorf("failed to write to IPC socket %s: %w", socketPath, err)
	}
	log.Printf("Successfully sent command to IPC socket: %s", channelID)
	return nil
}

// pipeCommands accepts connections on the Unix domain socket and forwards commands.
func pipeCommands(ctx context.Context, ln *net.UnixListener, cmdChan chan<- []byte) {
	defer close(cmdChan) // Close cmdChan when pipeCommands exits
	for {
		var unixConn *net.UnixConn
		var err error
		// Set a deadline for Accept so it doesn't block indefinitely and can check ctx.Done()
		if err := ln.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			log.Printf("pipeCommands: failed to set listener deadline: %v", err)
			return // or handle error more gracefully
		}

		unixConn, err = ln.AcceptUnix()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					log.Println("pipeCommands: context done, exiting accept loop.")
					return
				default:
					continue // Timeout, continue to check context and accept again
				}
			}
			log.Printf("pipeCommands: accept error: %v. This might happen during shutdown.", err)
			return // Exit if a non-timeout error occurs or if context is not done yet (unexpected)
		}

		// Handle each connection in a new goroutine to allow multiple simultaneous IPC clients (optional)
		// For simplicity here, handling one at a time. For concurrent, wrap the below in go func() { ... }()
		log.Printf("IPC connection accepted from: %s", unixConn.RemoteAddr())
		scanner := bufio.NewScanner(unixConn)
		for scanner.Scan() {
			cmdBytes := scanner.Bytes()
			// It's important to copy the bytes if they are to be used beyond this iteration
			// as scanner.Bytes() may reuse the buffer.
			cmdCopy := make([]byte, len(cmdBytes))
			copy(cmdCopy, cmdBytes)

			select {
			case cmdChan <- cmdCopy:
			case <-ctx.Done():
				log.Println("pipeCommands: context done while sending to cmdChan.")
				unixConn.Close()
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("pipeCommands: error reading from IPC socket: %v", err)
		}
		unixConn.Close()
		log.Println("IPC connection closed.")
		// Check context again before looping to accept a new connection
		select {
		case <-ctx.Done():
			log.Println("pipeCommands: context done after handling a connection.")
			return
		default:
			// continue to next accept
		}
	}
}
