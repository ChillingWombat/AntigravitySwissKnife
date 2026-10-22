// Command antigravity-acp serves one Antigravity surface over ACP on stdio.
package main

import (
	"flag"
	"log"
	"os"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/acp"
	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface/cli"
)

func main() {
	agyBinary := flag.String("agy-binary", "agy", "path to the Antigravity CLI binary")
	flag.Parse()

	log.SetOutput(os.Stderr) // stdout belongs to the protocol
	surf := cli.New(*agyBinary)
	agent := acp.New(surf)

	conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.Attach(conn)
	log.Printf("antigravity-acp: serving surface %q", surf.Name())
	<-conn.Done()
}
