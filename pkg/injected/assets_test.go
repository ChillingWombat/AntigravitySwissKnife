package injected

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

func TestRuntimeBundleAndDiagnostics(t *testing.T) {
	bundle := RuntimeBundle()
	if len(bundle) < 1000 {
		t.Fatalf("runtime bundle is implausibly small: %d bytes", len(bundle))
	}
	if !bytes.Contains(bundle, []byte("__SWISS_RUNTIME__")) {
		t.Fatal("runtime bundle is missing the reinjection handle")
	}

	diagnostics := Diagnostics()
	if diagnostics.Bytes != len(bundle) {
		t.Fatalf("Bytes = %d, want %d", diagnostics.Bytes, len(bundle))
	}
	if diagnostics.ModuleCount != 1 || diagnostics.ObserverCount != 1 || diagnostics.DeclaredTimerSites != 1 {
		t.Fatalf("unexpected runtime contract counts: %+v", diagnostics)
	}
	sum := sha256.Sum256(bundle)
	if diagnostics.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("SHA256 = %q, want %q", diagnostics.SHA256, hex.EncodeToString(sum[:]))
	}
	if diagnostics.GzipBytes != gzipSize(bundle) || diagnostics.GzipBytes <= 0 {
		t.Fatalf("GzipBytes = %d, want %d", diagnostics.GzipBytes, gzipSize(bundle))
	}

	reader, err := gzip.NewReader(bytes.NewReader(mustGzip(t, bundle)))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, bundle) {
		t.Fatal("gzip round trip changed the runtime bundle")
	}

	bundle[0] ^= 0xff
	if bytes.Equal(bundle, RuntimeBundle()) {
		t.Fatal("RuntimeBundle must return a copy")
	}
}

func TestRuntimeBundleContainsSingleOwnerContract(t *testing.T) {
	bundle := string(RuntimeBundle())
	for _, required := range []string{"MutationObserver", "setInterval", "swiss-runtime-contract-v1"} {
		if !strings.Contains(bundle, required) {
			t.Fatalf("runtime bundle is missing %q", required)
		}
	}
	if strings.Count(bundle, "MutationObserver") != 1 {
		t.Fatalf("runtime bundle must declare one MutationObserver, found %d", strings.Count(bundle, "MutationObserver"))
	}
	if strings.Count(bundle, "setInterval") != 1 {
		t.Fatalf("runtime bundle must declare one timer loop, found %d", strings.Count(bundle, "setInterval"))
	}
}

func mustGzip(t *testing.T, data []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}
