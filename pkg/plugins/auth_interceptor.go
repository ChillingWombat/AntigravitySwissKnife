package plugins

import "github.com/ChillingWombat/antigravity-swiss-knife/pkg/rendererbroker"

// SwissAuthInterceptorScript routes daemon-origin fetches through the Electron
// main-process broker. The bearer never enters page script.
const SwissAuthInterceptorScript = rendererbroker.FetchScript
