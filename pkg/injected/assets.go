// Package injected owns the browser runtime embedded in the companion daemon.
package injected

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

//go:embed assets/runtime.js
var runtimeBundle []byte

// RuntimeDiagnostics describes the exact embedded runtime artifact.
type RuntimeDiagnostics struct {
	Bytes              int    `json:"bytes"`
	GzipBytes          int    `json:"gzipBytes"`
	SHA256             string `json:"sha256"`
	ModuleCount        int    `json:"moduleCount"`
	ObserverCount      int    `json:"observerCount"`
	DeclaredTimerSites int    `json:"declaredTimerSites"`
}

// RuntimeBundle returns an isolated copy of the embedded runtime.
func RuntimeBundle() []byte {
	return bytes.Clone(runtimeBundle)
}

// Diagnostics reports artifact and runtime-contract metadata.
func Diagnostics() RuntimeDiagnostics {
	sum := sha256.Sum256(runtimeBundle)
	return RuntimeDiagnostics{
		Bytes:              len(runtimeBundle),
		GzipBytes:          gzipSize(runtimeBundle),
		SHA256:             hex.EncodeToString(sum[:]),
		ModuleCount:        1,
		ObserverCount:      1,
		DeclaredTimerSites: 1,
	}
}

func gzipSize(data []byte) int {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		panic(fmt.Sprintf("gzip embedded runtime: %v", err))
	}
	if err := writer.Close(); err != nil {
		panic(fmt.Sprintf("finish embedded runtime gzip: %v", err))
	}
	return compressed.Len()
}
