// Command acp-probe drives an ACP agent over stdio without an editor. It exists so
// the adapter can be exercised from a terminal: initialize, open a session, send one
// prompt, print the streamed answer and how the turn ended.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

// probeClient records what the agent sends and answers permission requests, since
// there is no human in the loop.
type probeClient struct {
	text strings.Builder
}

func (c *probeClient) SessionUpdate(_ context.Context, n acpsdk.SessionNotification) error {
	if n.Update.AgentMessageChunk != nil && n.Update.AgentMessageChunk.Content.Text != nil {
		chunk := n.Update.AgentMessageChunk.Content.Text.Text
		c.text.WriteString(chunk)
		fmt.Print(chunk)
	}
	return nil
}

func (c *probeClient) RequestPermission(_ context.Context, p acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	// Choose the first allow-shaped option; otherwise deny rather than hang.
	for _, option := range p.Options {
		if strings.Contains(strings.ToLower(string(option.Kind)), "allow") {
			return acpsdk.RequestPermissionResponse{
				Outcome: acpsdk.RequestPermissionOutcome{Selected: &acpsdk.RequestPermissionOutcomeSelected{OptionId: option.OptionId}},
			}, nil
		}
	}
	return acpsdk.RequestPermissionResponse{
		Outcome: acpsdk.RequestPermissionOutcome{Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{}},
	}, nil
}

func (c *probeClient) ReadTextFile(context.Context, acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	return acpsdk.ReadTextFileResponse{}, acpsdk.NewMethodNotFound("fs/read_text_file")
}

func (c *probeClient) WriteTextFile(context.Context, acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	return acpsdk.WriteTextFileResponse{}, acpsdk.NewMethodNotFound("fs/write_text_file")
}

func (c *probeClient) CreateTerminal(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/create")
}

func (c *probeClient) KillTerminal(context.Context, acpsdk.KillTerminalRequest) (acpsdk.KillTerminalResponse, error) {
	return acpsdk.KillTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/kill")
}

func (c *probeClient) TerminalOutput(context.Context, acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	return acpsdk.TerminalOutputResponse{}, acpsdk.NewMethodNotFound("terminal/output")
}

func (c *probeClient) ReleaseTerminal(context.Context, acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	return acpsdk.ReleaseTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/release")
}

func (c *probeClient) WaitForTerminalExit(context.Context, acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	return acpsdk.WaitForTerminalExitResponse{}, acpsdk.NewMethodNotFound("terminal/wait_for_exit")
}

func main() {
	server := flag.String("server", "antigravity-acp --surface ls", "agent command to spawn")
	prompt := flag.String("prompt", "Reply with exactly: probe-ok", "prompt to send")
	cwd := flag.String("cwd", ".", "session working directory")
	timeout := flag.Duration("timeout", 180*time.Second, "overall deadline")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fields := strings.Fields(*server)
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdin:", err)
		os.Exit(1)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdout:", err)
		os.Exit(1)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}
	defer func() { _ = cmd.Process.Kill() }()

	client := &probeClient{}
	conn := acpsdk.NewClientSideConnection(client, stdin, stdout)

	init, err := conn.Initialize(ctx, acpsdk.InitializeRequest{ProtocolVersion: acpsdk.ProtocolVersionNumber})
	if err != nil {
		fmt.Fprintln(os.Stderr, "initialize:", err)
		os.Exit(1)
	}
	fmt.Printf("agent: %s %s | protocol %d\n", init.AgentInfo.Name, init.AgentInfo.Version, init.ProtocolVersion)

	session, err := conn.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: *cwd, McpServers: []acpsdk.McpServer{}})
	if err != nil {
		fmt.Fprintln(os.Stderr, "session/new:", err)
		os.Exit(1)
	}
	fmt.Printf("session: %s\n", session.SessionId)

	var blocks []acpsdk.ContentBlock
	blocks = append(blocks, acpsdk.ContentBlock{Text: &acpsdk.ContentBlockText{Type: "text", Text: *prompt}})
	result, err := conn.Prompt(ctx, acpsdk.PromptRequest{SessionId: session.SessionId, Prompt: blocks})
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nprompt:", err)
		os.Exit(1)
	}
	fmt.Printf("\nstop: %s\n", result.StopReason)
}
