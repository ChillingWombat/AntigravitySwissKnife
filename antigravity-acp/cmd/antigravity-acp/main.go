// Command antigravity-acp serves one Antigravity surface over ACP on stdio.
package main

import (
	"flag"
	"log"
	"os"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/acp"
	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface/cli"
	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface/ls"
)

func main() {
	agyBinary := flag.String("agy-binary", "agy", "path to the Antigravity CLI binary")
	surfaceName := flag.String("surface", "auto", "surface to serve: auto, ls, or cli")
	logPath := flag.String("ls-log", ls.DefaultLogPath(), "language server log used to find the port")
	flag.Parse()

	log.SetOutput(os.Stderr) // stdout belongs to the protocol

	surf := pickSurface(*surfaceName, *agyBinary, *logPath)
	agent := acp.New(surf)

	conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.Attach(conn)
	log.Printf("antigravity-acp: serving surface %q", surf.Name())
	<-conn.Done()
}

// pickSurface prefers the language server because it is the same runtime the
// desktop app and the VS Code extension use; the CLI is the fallback when no
// server is running.
func pickSurface(name, agyBinary, logPath string) surface.Surface {
	switch name {
	case "cli":
		return cli.New(agyBinary)
	case "ls":
		return mustLS(logPath)
	}
	if baseURL, csrf, err := ls.Discover(logPath); err == nil {
		log.Printf("antigravity-acp: language server at %s", baseURL)
		return ls.New(baseURL, csrf)
	} else {
		log.Printf("antigravity-acp: no language server (%v), falling back to the CLI", err)
	}
	return cli.New(agyBinary)
}

func mustLS(logPath string) surface.Surface {
	baseURL, csrf, err := ls.Discover(logPath)
	if err != nil {
		log.Fatalf("antigravity-acp: language server discovery failed: %v", err)
	}
	return ls.New(baseURL, csrf)
}
