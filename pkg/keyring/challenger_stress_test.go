package keyring

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStress_EmailsEquivalent_MalformedAndAdversarial(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		// Malformed syntax
		{"empty a", "", "user@gmail.com", false},
		{"empty b", "user@gmail.com", "", false},
		{"both empty", "", "", false},
		{"single @", "@", "@", true},
		{"at with user", "user@", "user@", true},
		{"at with domain", "@gmail.com", "@gmail.com", true},
		{"at with gmail vs at with gmail", "@gmail.com", "@googlemail.com", false},
		{"double at", "user@@gmail.com", "user@gmail.com", false},
		{"double at dot variant", "u.ser@@gmail.com", "user@@gmail.com", true},
		{"subdomain gmail", "user@sub.gmail.com", "user@gmail.com", false},
		{"subdomain gmail dot variant", "u.ser@sub.gmail.com", "user@sub.gmail.com", false},
		{"gmail prefix domain", "user@gmail.com.attacker.com", "user@gmail.com", false},
		{"just dots", "...", "...", true},
		{"dots gmail", "...@gmail.com", "user@gmail.com", false},
		{"dots only local gmail", "...@gmail.com", "...@gmail.com", true},

		// Standard Gmail dot equivalence
		{"simple dots", "first.middle.last@gmail.com", "firstmiddlelast@gmail.com", true},
		{"mixed case and dots", "FiRsT.mIdDlE.lAsT@gMaIl.CoM", "firstmiddlelast@gmail.com", true},
		{"googlemail to gmail", "user.name@googlemail.com", "username@gmail.com", true},
		{"leading dot in gmail", ".username@gmail.com", "username@gmail.com", true},
		{"trailing dot in gmail", "username.@gmail.com", "username@gmail.com", true},
		{"consecutive dots in gmail", "u...s...e...r@gmail.com", "user@gmail.com", true},

		// Non-Gmail domains (must NOT strip dots)
		{"outlook with dots", "first.last@outlook.com", "firstlast@outlook.com", false},
		{"custom domain with dots", "admin.test@mycompany.org", "admintest@mycompany.org", false},
		{"non-gmail identical case diff", "User@Example.Com", "user@example.com", true},

		// Sub-addressing (+tag)
		{"tag vs no tag", "user+tag@gmail.com", "user@gmail.com", false},
		{"same tag same domain", "user+tag@gmail.com", "user+tag@gmail.com", true},
		{"same tag dot variant", "u.ser+tag@gmail.com", "user+tag@gmail.com", true},
		{"same tag cross domain", "u.ser+tag@googlemail.com", "user+tag@gmail.com", true},
		{"different tags", "user+tag1@gmail.com", "user+tag2@gmail.com", false},
		{"subaddress non-gmail dot variant", "user.name+tag@yahoo.com", "username+tag@yahoo.com", false},

		// Unicode & Homoglyph attacks
		{"unicode accented domain", "user@gm\u00e1il.com", "user@gmail.com", false},
		{"cyrillic small a in domain", "user@gm\u0430il.com", "user@gmail.com", false},
		{"turkish dotless i in domain", "user@gma\u0131l.com", "user@gmail.com", false},
		{"punycode domain", "user@xn--gmil-6na.com", "user@gmail.com", false},
		{"unicode local part gmail dot strip", "\u00fc.ser@gmail.com", "\u00fcser@googlemail.com", true},
		{"fullwidth dot in local part", "user\uff0ename@gmail.com", "username@gmail.com", false},

		// Whitespace handling
		{"whitespace trim", "  user@gmail.com  ", "user@gmail.com", true},
		{"tab and newline trim", "\tuser.name@gmail.com\r\n", "username@googlemail.com", true},
		{"internal whitespace", "user name@gmail.com", "username@gmail.com", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EmailsEquivalent(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("EmailsEquivalent(%q, %q) = %v; want %v", tc.a, tc.b, got, tc.want)
			}
			// Symmetric property: EmailsEquivalent(a, b) == EmailsEquivalent(b, a)
			gotRev := EmailsEquivalent(tc.b, tc.a)
			if gotRev != got {
				t.Errorf("EmailsEquivalent symmetry violation: (%q, %q) = %v, but (%q, %q) = %v",
					tc.a, tc.b, got, tc.b, tc.a, gotRev)
			}
		})
	}
}

func TestStress_EmailsEquivalent_ExtremeLengths(t *testing.T) {
	// 10,000 dots in local part
	longDots := "u" + strings.Repeat(".", 10000) + "ser@gmail.com"
	if !EmailsEquivalent(longDots, "user@gmail.com") {
		t.Errorf("failed to equate email with 10000 dots")
	}

	// 100,000 character local part
	longLocalA := strings.Repeat("a", 50000) + "." + strings.Repeat("b", 50000) + "@gmail.com"
	longLocalB := strings.Repeat("a", 50000) + strings.Repeat("b", 50000) + "@googlemail.com"
	if !EmailsEquivalent(longLocalA, longLocalB) {
		t.Errorf("failed to equate 100,000 char email addresses")
	}

	// Extremely long non-matching strings
	hugeA := strings.Repeat("x", 100000) + "@gmail.com"
	hugeB := strings.Repeat("y", 100000) + "@gmail.com"
	if EmailsEquivalent(hugeA, hugeB) {
		t.Errorf("falsely equated different 100k char emails")
	}
}

func TestStress_Store_DuplicateImport_ConcurrentAndSubAddressing(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// 1. Initial canonical imports
	baseAccounts := []string{
		"Alpha.User@gmail.com",
		"Beta.User+work@gmail.com",
		"Gamma.User@customdomain.io",
		"Delta.User@googlemail.com",
	}

	for _, em := range baseAccounts {
		_, err := store.ImportAccount(em, "1//initial_"+em, "ya29."+em, "Label "+em, "")
		if err != nil {
			t.Fatalf("ImportAccount %q failed: %v", em, err)
		}
	}

	if got := len(store.ListAccounts()); got != 4 {
		t.Fatalf("expected 4 initial accounts, got %d", got)
	}

	// 2. High concurrency stress test: 50 workers importing dot variants,
	// different casing, and googlemail/gmail domain variants.
	var wg sync.WaitGroup
	workers := 50
	errChan := make(chan error, workers*10)

	variants := []string{
		"alphauser@gmail.com",
		"ALPHA.USER@GOOGLEMAIL.COM",
		"a.l.p.h.a.u.s.e.r@gmail.com",
		"betauser+work@googlemail.com",
		"BETA.USER+WORK@GMAIL.COM",
		"deltauser@gmail.com",
		"DELTA.USER@GOOGLEMAIL.COM",
		"gamma.user@customdomain.io", // same non-gmail with lower case
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j, v := range variants {
				token := fmt.Sprintf("1//token_w%d_v%d", workerID, j)
				acc, err := store.ImportAccount(v, token, "ya29.new", "Updated Label", "")
				if err != nil {
					errChan <- fmt.Errorf("worker %d variant %q error: %w", workerID, v, err)
					return
				}
				if acc == nil {
					errChan <- fmt.Errorf("worker %d variant %q returned nil account", workerID, v)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Fatalf("concurrent import error: %v", err)
	}

	// Must STILL have exactly 4 accounts! No duplicate accounts created.
	accounts := store.ListAccounts()
	if len(accounts) != 4 {
		var names []string
		for _, a := range accounts {
			names = append(names, a.Email)
		}
		t.Fatalf("expected exactly 4 accounts after concurrent variant imports, got %d: %v", len(accounts), names)
	}

	// Now import a genuinely different account and sub-address
	_, _ = store.ImportAccount("Beta.User+personal@gmail.com", "1//personal", "", "", "")
	_, _ = store.ImportAccount("GammaUser@customdomain.io", "1//gamma2", "", "", "") // non-gmail dot diff = new account

	if got := len(store.ListAccounts()); got != 6 {
		t.Fatalf("expected 6 accounts after distinct imports, got %d", got)
	}
}

func TestStress_WindowsKeyring_SchemaAndJSONInjection(t *testing.T) {
	// Test mintMinimalIDToken with injection attempts
	injections := []string{
		`attacker@gmail.com","admin":true,"hack":"`,
		"attacker\n\r\t@gmail.com",
		`attacker\"@gmail.com`,
		`"><script>alert(1)</script>@gmail.com`,
		"unicode_user_\u2603@gmail.com",
		"",
	}

	for _, inj := range injections {
		jwt := mintMinimalIDToken(inj)
		if inj == "" {
			if jwt != "" {
				t.Errorf("mintMinimalIDToken(\"\") expected empty, got %q", jwt)
			}
			continue
		}

		parts := strings.Split(jwt, ".")
		if len(parts) != 3 {
			t.Fatalf("invalid JWT parts for injection %q: %d", inj, len(parts))
		}

		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatalf("JWT payload not valid base64url for %q: %v", inj, err)
		}

		var claims map[string]interface{}
		if err := json.Unmarshal(payloadBytes, &claims); err != nil {
			t.Fatalf("JWT claims corrupted by injection %q: %v, raw: %s", inj, err, string(payloadBytes))
		}

		if emailVal, ok := claims["email"].(string); !ok || emailVal != strings.TrimSpace(inj) {
			t.Errorf("claim email value mismatch for %q: got %v", inj, claims["email"])
		}
	}
}

func TestStress_WindowsKeyring_PayloadRoundTrip(t *testing.T) {
	// Test schema compatibility with both Windows format and nested format
	acc := &Account{
		Email:        "adversarial\"test@gmail.com",
		AccessToken:  "ya29.secret\"token\nwith\rnewlines",
		RefreshToken: "1//refresh\"token\twith\tspecial",
		IDToken:      "", // triggers mintMinimalIDToken
		TokenExpiry:  time.Now().Add(2 * time.Hour),
	}

	// Simulate buildWindowsKeyringPayload serialization
	type testPayload struct {
		Token struct {
			Expiry       string `json:"expiry"`
			AccessToken  string `json:"access_token"`
			TokenType    string `json:"token_type"`
			RefreshToken string `json:"refresh_token"`
		} `json:"token"`
		AuthMethod string `json:"auth_method"`
		IDToken    string `json:"id_token"`
	}

	var p testPayload
	p.AuthMethod = "consumer"
	p.Token.AccessToken = acc.AccessToken
	p.Token.TokenType = "Bearer"
	p.Token.RefreshToken = acc.RefreshToken
	p.Token.Expiry = acc.TokenExpiry.Format(time.RFC3339Nano)
	p.IDToken = mintMinimalIDToken(acc.Email)

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Verify unmarshal succeeds cleanly
	var parsed testPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if parsed.Token.AccessToken != acc.AccessToken {
		t.Errorf("AccessToken roundtrip mismatch: got %q, want %q", parsed.Token.AccessToken, acc.AccessToken)
	}
	if parsed.Token.RefreshToken != acc.RefreshToken {
		t.Errorf("RefreshToken roundtrip mismatch: got %q, want %q", parsed.Token.RefreshToken, acc.RefreshToken)
	}
	if parsed.IDToken == "" {
		t.Errorf("expected minted IDToken, got empty")
	}

	// Test unmarshal with malformed / adversarial JSON payloads
	malformed := []string{
		`{not valid json`,
		`{"token": "not an object"}`,
		`{"token": null}`,
		`{"token": {"access_token": 12345}}`,
		`[]`,
		`""`,
		``,
	}

	for _, m := range malformed {
		var sink testPayload
		_ = json.Unmarshal([]byte(m), &sink) // must not panic
	}
}
