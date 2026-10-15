//go:build !windows

package keyring

// readNativeKeyringToken: on Linux and macOS account detection relies on the
// existing secret-service / token-file paths; there is no additional native
// keyring source to read here.
func readNativeKeyringToken() (*secretTokenData, error) {
	return nil, nil
}

// WriteWindowsKeyringToken is a no-op outside Windows (the Windows Credential
// Manager is the desktop authstore's session store only on Windows).
func WriteWindowsKeyringToken(acc *Account) error {
	return nil
}
