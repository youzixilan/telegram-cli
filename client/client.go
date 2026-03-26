package client

/*
#cgo CFLAGS: -I/Users/aqin/dev/tdlib-build/install/include
#cgo LDFLAGS: -L/Users/aqin/dev/tdlib-build/install/lib -ltdjson
#include <stdlib.h>
#include <td/telegram/td_json_client.h>
#include <td/telegram/td_log.h>
*/
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// Client wraps a TDLib JSON client instance.
type Client struct {
	id       unsafe.Pointer
	mu       sync.Mutex
	closed   bool
	timeout  float64
	handlers map[string]chan json.RawMessage
	hmu      sync.Mutex
	extraSeq int64
	updates  chan json.RawMessage
	done     chan struct{}
}

// SetLogVerbosity sets TDLib log verbosity level (0=fatal, 1=error, 2=warning, 3=info, 4=debug, 5=verbose).
func SetLogVerbosity(level int) {
	req := fmt.Sprintf(`{"@type":"setLogVerbosityLevel","new_verbosity_level":%d}`, level)
	cs := C.CString(req)
	defer C.free(unsafe.Pointer(cs))
	C.td_json_client_execute(nil, cs)
}

// New creates a new TDLib client.
func New() *Client {
	c := &Client{
		id:       C.td_json_client_create(),
		timeout:  10.0,
		handlers: make(map[string]chan json.RawMessage),
		updates:  make(chan json.RawMessage, 1024),
		done:     make(chan struct{}),
	}
	go c.receiver()
	return c
}

// Send sends a request to TDLib without waiting for a response.
func (c *Client) Send(req interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	cs := C.CString(string(data))
	defer C.free(unsafe.Pointer(cs))
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("client is closed")
	}
	C.td_json_client_send(c.id, cs)
	return nil
}

// Execute sends a synchronous request to TDLib.
func (c *Client) Execute(req interface{}) (json.RawMessage, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	cs := C.CString(string(data))
	defer C.free(unsafe.Pointer(cs))
	result := C.td_json_client_execute(c.id, cs)
	if result == nil {
		return nil, errors.New("execute returned nil")
	}
	return json.RawMessage(C.GoString(result)), nil
}

// SendAndWait sends a request and waits for the response.
func (c *Client) SendAndWait(req map[string]interface{}, timeout time.Duration) (json.RawMessage, error) {
	c.hmu.Lock()
	c.extraSeq++
	extra := fmt.Sprintf("req_%d", c.extraSeq)
	c.hmu.Unlock()

	req["@extra"] = extra

	ch := make(chan json.RawMessage, 1)
	c.hmu.Lock()
	c.handlers[extra] = ch
	c.hmu.Unlock()

	defer func() {
		c.hmu.Lock()
		delete(c.handlers, extra)
		c.hmu.Unlock()
	}()

	if err := c.Send(req); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("request timed out after %v", timeout)
	}
}

// Updates returns the channel for receiving TDLib updates.
func (c *Client) Updates() <-chan json.RawMessage {
	return c.updates
}

// Close destroys the TDLib client.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	C.td_json_client_destroy(c.id)
	close(c.done)
}

func (c *Client) receiver() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		result := C.td_json_client_receive(c.id, C.double(c.timeout))
		if result == nil {
			continue
		}

		data := json.RawMessage(C.GoString(result))

		// check if this is a response to a request (has @extra)
		var meta struct {
			Extra string `json:"@extra"`
		}
		if err := json.Unmarshal(data, &meta); err == nil && meta.Extra != "" {
			c.hmu.Lock()
			ch, ok := c.handlers[meta.Extra]
			c.hmu.Unlock()
			if ok {
				ch <- data
				continue
			}
		}

		// otherwise it's an update
		select {
		case c.updates <- data:
		default:
			// drop if buffer full
		}
	}
}
