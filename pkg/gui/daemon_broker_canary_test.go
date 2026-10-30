package gui_test

import (
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/plugins"
)

func TestInjectedScriptsDoNotExposeDaemonBearerToken(t *testing.T) {
	scripts := map[string]string{
		"gui":          gui.GenerateScript(nil),
		"custommodels": custommodels.GenerateCustomModelsScript(nil),
		"enhancements": enhancements.GenerateEnhancementsScript(nil),
		"auxiliary":    plugins.GenerateAuxiliaryPluginsScript(),
		"github":       plugins.GenerateGitHubExtensionScript(),
	}
	for name, script := range scripts {
		if strings.Contains(script, "__SWISS_SESSION_TOKEN__") {
			t.Errorf("%s still exposes window.__SWISS_SESSION_TOKEN__", name)
		}
		if strings.Contains(script, "antigravity_swiss_session_token") {
			t.Errorf("%s still writes the daemon token to sessionStorage", name)
		}
		if !strings.Contains(script, "window.swiss") {
			t.Errorf("%s does not contain the Electron broker bridge", name)
		}
	}
}
