package bridge

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// postJSON posts a JSON body and treats a non-2xx answer as an error carrying the
// response snippet, so a rejected send is never mistaken for a delivered message.
func postJSON(url, bearer string, body []byte) error {
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}
