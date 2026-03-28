// ---------------------------------------------------------------------------------------
//
//	main.go
//	-------
//
//	Entry point for the tls-switch Go binary. Runs a JSON Lines protocol loop
//	over stdin/stdout — reads requests, dispatches to handlers, writes responses.
//	Never prints directly to console; all output is structured JSON.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created
//
// ---------------------------------------------------------------------------------------
package main

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"bufio"
	"encoding/json"
	"os"
)

// ---------------------------------------------------------------------------------------
//
//	Types
//
// ---------------------------------------------------------------------------------------

// Request is a JSON Lines request from the Python wrapper.
type Request struct {
	Command string          `json:"command"`
	Args    json.RawMessage `json:"args,omitempty"`
}

// Response is a JSON Lines response to the Python wrapper.
type Response struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------------------
//
//	Command Handlers
//
// ---------------------------------------------------------------------------------------

func handleHello(args json.RawMessage) Response {
	return Response{
		Status: "ok",
		Data: map[string]string{
			"message": "Hello, World!",
		},
	}
}

// ---------------------------------------------------------------------------------------
//
//	Command Dispatch
//
// ---------------------------------------------------------------------------------------

var commands = map[string]func(json.RawMessage) Response{
	"hello": handleHello,
}

// ---------------------------------------------------------------------------------------
//
//	Main
//
// ---------------------------------------------------------------------------------------

const maxLineSize = 1024 * 1024 // 1MB max request size

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, maxLineSize), maxLineSize)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			if err := encoder.Encode(Response{
				Status: "error",
				Error:  "invalid request: " + err.Error(),
			}); err != nil {
				return
			}
			continue
		}

		handler, ok := commands[req.Command]
		if !ok {
			if err := encoder.Encode(Response{
				Status: "error",
				Error:  "unknown command: " + req.Command,
			}); err != nil {
				return
			}
			continue
		}

		resp := handler(req.Args)
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
}
