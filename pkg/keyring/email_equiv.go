package keyring

import "strings"

// EmailsEquivalent reports whether two email addresses identify the same
// Google account for vault reconciliation purposes:
//   - comparison is case-insensitive;
//   - Gmail addresses ignore dots in the local part (gmail.com and
//     googlemail.com are treated as the same domain);
//   - "+tag" sub-addresses and every other domain are compared verbatim, so
//     unrelated accounts are never merged.
//
// This exists because Google's own tokens can carry either form of a Gmail
// address (e.g. "GersioOlba.p238@gmail.com" in the vault vs
// "gersioolbap238@gmail.com" in an id_token claim), and a plain string match
// used to auto-import the variant as a duplicate account.
func EmailsEquivalent(a, b string) bool {
	aNorm := strings.ToLower(strings.TrimSpace(a))
	bNorm := strings.ToLower(strings.TrimSpace(b))
	if aNorm == "" || bNorm == "" {
		return false
	}
	if aNorm == bNorm {
		return true
	}
	aCanon, aIsGmail := canonicalGmailAddress(aNorm)
	bCanon, bIsGmail := canonicalGmailAddress(bNorm)
	if !aIsGmail || !bIsGmail {
		return false
	}
	return aCanon == bCanon
}

// canonicalGmailAddress strips dots from the local part and normalizes the
// googlemail.com domain. Returns ("", false) for non-Gmail addresses.
func canonicalGmailAddress(addr string) (string, bool) {
	at := strings.LastIndex(addr, "@")
	if at <= 0 || at == len(addr)-1 {
		return "", false
	}
	local := addr[:at]
	domain := addr[at+1:]
	switch domain {
	case "gmail.com", "googlemail.com":
	default:
		return "", false
	}
	local = strings.ReplaceAll(local, ".", "")
	if local == "" {
		return "", false
	}
	return local + "@gmail.com", true
}

// canonicalEmailLocked maps email to the stored canonical form of an
// equivalent existing account (Gmail dot/domain variants), or returns the
// input unchanged when no equivalent account exists.
// Caller must hold s.mu.
func (s *Store) canonicalEmailLocked(email string) string {
	if _, ok := s.accounts[email]; ok {
		return email
	}
	for existing := range s.accounts {
		if EmailsEquivalent(existing, email) {
			return existing
		}
	}
	return email
}
