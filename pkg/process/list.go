package process

// ProcEntry is a minimal cross-platform process snapshot used by callers that
// only need to match on executable name and command line (e.g. the ACP agent
// registry). Unlike ProcessInfo it carries no shield/protection semantics.
type ProcEntry struct {
	PID     int
	Comm    string
	Cmdline string
}
