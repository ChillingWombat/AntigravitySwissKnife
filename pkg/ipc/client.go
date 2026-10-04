package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// Client sends requests over the Unix domain socket to the Swiss Knife daemon.
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient returns a new IPC client.
func NewClient(socketPath string) *Client {
	if socketPath == "" {
		socketPath = core.GetSocketPath()
	}
	return &Client{
		socketPath: socketPath,
		timeout:    core.DefaultSocketTimeout,
	}
}

// Call sends a JSON-RPC 2.0 request and unmarshals the result into out.
func (c *Client) Call(method string, params interface{}, out interface{}) error {
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return fmt.Errorf("%w: %v", core.ErrDaemonUnavailable, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		rawParams = b
	}

	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
		ID:      1,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	reqBytes = append(reqBytes, '\n')

	if _, err := conn.Write(reqBytes); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return err
	}

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return err
	}

	if resp.Error != nil {
		return resp.Error
	}

	if out != nil && resp.Result != nil {
		resultBytes, err := json.Marshal(resp.Result)
		if err != nil {
			return err
		}
		return json.Unmarshal(resultBytes, out)
	}
	return nil
}
