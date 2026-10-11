//go:build !windows

package process

import (
	"os/exec"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func setDetachedProcess(cmd *exec.Cmd) {
	core.SetDetachedProcess(cmd)
}
