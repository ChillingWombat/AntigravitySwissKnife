package gui

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAdversarial_DesktopLoader_URLInjectionGuard(t *testing.T) {
	script := generateMainLoaderScript()
	start := strings.Index(script, "function isSwissInjectableWindowURL")
	if start < 0 {
		t.Fatal("generated loader URL guard not found")
	}
	endOffset := strings.Index(script[start:], "\n    async function injectIntoWebContents")
	if endOffset < 0 {
		t.Fatal("generated loader URL guard boundary not found")
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for loader URL guard coverage")
	}

	nodeScript := script[start:start+endOffset] + `
const allowed = [
  "",
  "about:blank",
  "file:///tmp/workbench.html",
  "vscode-webview://antigravity/index.html",
  "vscode-file://vscode-app/Applications/Antigravity.app/",
  "antigravity://workbench",
  "http://127.0.0.1:8765/chat",
  "https://127.0.0.1:8765/chat",
  "http://localhost:8765",
  "https://localhost:3000/chat"
];
const blocked = [
  "devtools://devtools/bundled/inspector.html",
  "https://example.com/",
  "http://example.com/",
  "http://localhost.evil.test/",
  "https://127.0.0.1.evil.test/",
  "data:text/html,<h1>external</h1>",
  "javascript:alert(1)",
  "about:srcdoc",
  "not a url"
];
for (const value of allowed) {
  if (!isSwissInjectableWindowURL(value)) {
    console.error("expected injectable URL: " + value);
    process.exit(1);
  }
}
for (const value of blocked) {
  if (isSwissInjectableWindowURL(value)) {
    console.error("expected blocked URL: " + value);
    process.exit(1);
  }
}
`
	if out, err := exec.Command(nodePath, "-e", nodeScript).CombinedOutput(); err != nil {
		t.Fatalf("loader URL guard failed: %v\n%s", err, string(out))
	}
}

func TestAdversarial_DesktopLoader_CSSInjectionDeduplication(t *testing.T) {
	script := generateMainLoaderScript()
	start := strings.Index(script, "async function injectIntoWebContents")
	if start < 0 {
		t.Fatal("generated loader injection function not found")
	}
	endOffset := strings.Index(script[start:], "\n    if (app &&")
	if endOffset < 0 {
		t.Fatal("generated loader injection function boundary not found")
	}

	for _, token := range []string{
		"if (!isSwissInjectableWindowURL(currentUrl)) return;",
		"wc.__swissInjectedCssLength !== cssLength",
		"wc.__swissInjectedCssLength = cssLength",
		"win.webContents.on('did-start-navigation'",
		"win.webContents.__swissInjectedCssLength = 0",
	} {
		if !strings.Contains(script, token) {
			t.Errorf("generated loader missing CSS injection contract %q", token)
		}
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for CSS injection deduplication coverage")
	}

	guardStart := strings.Index(script, "function isSwissInjectableWindowURL")
	if guardStart < 0 {
		t.Fatal("generated loader URL guard not found")
	}
	guardEndOffset := strings.Index(script[guardStart:], "\n    function installDaemonAuthBridge")
	if guardEndOffset < 0 {
		t.Fatal("generated loader URL guard boundary not found")
	}

	nodeScript := `
const cssPath = "persistent_styles.css";
const jsPath = "persistent_script.js";
const injecting = new WeakSet();
const fs = {
  existsSync: value => value === cssPath,
  readFileSync: value => {
    if (value !== cssPath) throw new Error("unexpected read: " + value);
    return "body { color: red; }";
  }
};
function buildDaemonBrokerScript() { return ""; }
` + script[guardStart:guardStart+guardEndOffset] + script[start:start+endOffset] + `
(async () => {
  let inserts = 0;
  const wc = {
    __swissInjectedCssLength: undefined,
    isDestroyed: () => false,
    getURL: () => "http://localhost:8765",
    insertCSS: () => { inserts += 1; return Promise.resolve("css-key"); },
    executeJavaScript: () => { throw new Error("unexpected JavaScript injection"); }
  };
  await injectIntoWebContents(wc);
  await injectIntoWebContents(wc);
  if (inserts !== 1) throw new Error("expected one CSS insertion, got " + inserts);
  wc.__swissInjectedCssLength = 0;
  await injectIntoWebContents(wc);
  if (inserts !== 2) throw new Error("expected CSS insertion after navigation reset, got " + inserts);

  let externalInserts = 0;
  const external = {
    isDestroyed: () => false,
    getURL: () => "https://example.com/",
    insertCSS: () => { externalInserts += 1; return Promise.resolve("css-key"); },
    executeJavaScript: () => { throw new Error("unexpected external JavaScript injection"); }
  };
  await injectIntoWebContents(external);
  if (externalInserts !== 0) throw new Error("external page received CSS");
})().catch(error => {
  console.error(error);
  process.exit(1);
});
`
	if out, err := exec.Command(nodePath, "-e", nodeScript).CombinedOutput(); err != nil {
		t.Fatalf("loader CSS deduplication failed: %v\n%s", err, string(out))
	}
}

func TestAdversarial_CopyFile_Permissions_RestrictiveSource(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src_restrictive.bin")
	dst := filepath.Join(tempDir, "dst_restrictive.bin")

	// 1. Test with 0400 (read-only owner)
	content := []byte("read-only source content")
	if err := os.WriteFile(src, content, 0400); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed with 0400 src: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0644 {
		t.Fatalf("expected 0644 permissions, got %#o", info.Mode().Perm())
	}

	readDst, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(readDst) != string(content) {
		t.Fatalf("content mismatch: expected %q, got %q", string(content), string(readDst))
	}
}

func TestAdversarial_CopyFile_OverwriteReadOnlyDestination(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.bin")
	dst := filepath.Join(tempDir, "dst_readonly.bin")

	srcContent := []byte("new replacement content")
	if err := os.WriteFile(src, srcContent, 0644); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}

	// Pre-create dst with 0444 (read-only for all)
	oldContent := []byte("old read-only content")
	if err := os.WriteFile(dst, oldContent, 0444); err != nil {
		t.Fatalf("failed to pre-create dst: %v", err)
	}

	// copyFile should successfully atomically replace read-only destination via rename
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed to overwrite read-only dst: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0644 {
		t.Fatalf("expected dst to have 0644 permissions after copy, got %#o", info.Mode().Perm())
	}

	readDst, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(readDst) != string(srcContent) {
		t.Fatalf("expected dst to contain new content, got %q", string(readDst))
	}
}

func TestAdversarial_EnsureDesktopFileReplaceable_ReadOnlyFileInWritableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows requires the destination file itself to be probe-opened")
	}

	tempDir := t.TempDir()
	asarPath := filepath.Join(tempDir, "app.asar")
	if err := os.WriteFile(asarPath, []byte("factory-asar"), 0444); err != nil {
		t.Fatalf("failed to create read-only asar: %v", err)
	}

	if err := ensureDesktopFileReplaceable(asarPath); err != nil {
		t.Fatalf("read-only app.asar in writable resources directory should be replaceable: %v", err)
	}
}

func TestAdversarial_CopyFile_LargePayloadAndIntegrity(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "large_src.bin")
	dst := filepath.Join(tempDir, "large_dst.bin")

	// 2MB random payload simulating large asar archive
	payload := make([]byte, 2*1024*1024)
	r := rand.New(rand.NewSource(42))
	r.Read(payload)

	hasher := sha256.New()
	hasher.Write(payload)
	srcHash := hex.EncodeToString(hasher.Sum(nil))

	if err := os.WriteFile(src, payload, 0644); err != nil {
		t.Fatalf("failed to write large src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed on large payload: %v", err)
	}

	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}
	if dstInfo.Size() != int64(len(payload)) {
		t.Fatalf("size mismatch: expected %d, got %d", len(payload), dstInfo.Size())
	}
	if runtime.GOOS != "windows" && dstInfo.Mode().Perm() != 0644 {
		t.Fatalf("expected 0644 permissions, got %#o", dstInfo.Mode().Perm())
	}

	dstBytes, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	dstHasher := sha256.New()
	dstHasher.Write(dstBytes)
	dstHash := hex.EncodeToString(dstHasher.Sum(nil))

	if srcHash != dstHash {
		t.Fatalf("sha256 mismatch between src and dst: src=%s, dst=%s", srcHash, dstHash)
	}
}

func TestAdversarial_CopyFile_NoDanglingTempFiles(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.txt")
	dst := filepath.Join(tempDir, "dst.txt")
	_ = os.WriteFile(src, []byte("data"), 0644)

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, entry := range entries {
		matched, _ := filepath.Match("swiss_copy_*", entry.Name())
		if matched {
			t.Fatalf("found dangling temporary file %s in directory", entry.Name())
		}
	}

	// Trigger failure: nonexistent source
	nonexistent := filepath.Join(tempDir, "nonexistent.bin")
	_ = copyFile(nonexistent, filepath.Join(tempDir, "failed.bin"))

	entriesAfterFail, _ := os.ReadDir(tempDir)
	for _, entry := range entriesAfterFail {
		matched, _ := filepath.Match("swiss_copy_*", entry.Name())
		if matched {
			t.Fatalf("found dangling temporary file %s after copy failure", entry.Name())
		}
	}
}

func TestAdversarial_DesktopLoader_MultiStageLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	asarPath := filepath.Join(tempDir, "app.asar")
	backupPath := filepath.Join(tempDir, "app.asar.factory_backup")

	v1Content := "upstream-antigravity-version-1.0.0"
	if err := os.WriteFile(asarPath, []byte(v1Content), 0644); err != nil {
		t.Fatalf("failed to write asar: %v", err)
	}

	dm := NewDesktopManager(tempDir, filepath.Join(tempDir, "gui.json"), nil)

	// Stage 1: No backup exists, asar is unpatched
	st := dm.GetStatus()
	if st.Installed || st.BackupExists {
		t.Fatalf("expected Installed=false, BackupExists=false, got %+v", st)
	}

	// Calling InstallDesktopLoader should create the backup from v1.0.0 before unpack failure
	_, _ = dm.InstallDesktopLoader()

	b1, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backupPath: %v", err)
	}
	if string(b1) != v1Content {
		t.Fatalf("expected backup to match v1 content, got %q", string(b1))
	}
	b1Info, _ := os.Stat(backupPath)
	if runtime.GOOS != "windows" && b1Info.Mode().Perm() != 0644 {
		t.Fatalf("expected backup to have 0644 permissions, got %#o", b1Info.Mode().Perm())
	}

	// Stage 2: Upstream Antigravity update to v2.0.0
	// Upstream update overwrites app.asar with clean unpatched v2.0.0
	v2Content := "upstream-antigravity-version-2.0.0"
	if err := os.WriteFile(asarPath, []byte(v2Content), 0644); err != nil {
		t.Fatalf("failed to overwrite asar with v2: %v", err)
	}

	st2 := dm.GetStatus()
	if st2.Installed {
		t.Fatalf("expected Installed=false for new clean v2 asar")
	}
	if !st2.BackupExists {
		t.Fatalf("expected BackupExists=true from previous backup")
	}

	// InstallDesktopLoader MUST refresh backupPath from clean v2 asar!
	_, _ = dm.InstallDesktopLoader()

	b2, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backupPath after update: %v", err)
	}
	if string(b2) != v2Content {
		t.Fatalf("expected backupPath to be refreshed to v2Content, got %q", string(b2))
	}

	// Stage 3: Repeated call when already patched (Installed == true)
	patchedContent := "patched-asar-containing-" + SwissLoaderMarker
	if err := os.WriteFile(asarPath, []byte(patchedContent), 0644); err != nil {
		t.Fatalf("failed to write patched asar: %v", err)
	}

	st3 := dm.GetStatus()
	if !st3.Installed {
		t.Fatalf("expected Installed=true for patched asar")
	}

	// Calling InstallDesktopLoader MUST NOT overwrite clean backup with patched content!
	_, _ = dm.InstallDesktopLoader()

	b3, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backupPath: %v", err)
	}
	if string(b3) != v2Content {
		t.Fatalf("expected backupPath to remain clean v2Content, got %q", string(b3))
	}

	// Stage 4: UninstallDesktopLoader restores the clean v2 backup
	res, err := dm.UninstallDesktopLoader()
	if err != nil {
		t.Fatalf("UninstallDesktopLoader failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("UninstallDesktopLoader returned success=false: %s", res.Message)
	}

	restoredAsar, err := os.ReadFile(asarPath)
	if err != nil {
		t.Fatalf("failed to read restored asarPath: %v", err)
	}
	if string(restoredAsar) != v2Content {
		t.Fatalf("expected restored asar to match v2Content, got %q", string(restoredAsar))
	}
	asarInfo, _ := os.Stat(asarPath)
	if runtime.GOOS != "windows" && asarInfo.Mode().Perm() != 0644 {
		t.Fatalf("expected restored asar to have 0644 permissions, got %#o", asarInfo.Mode().Perm())
	}
}
