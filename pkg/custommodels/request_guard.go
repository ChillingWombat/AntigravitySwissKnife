package custommodels

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Upstream429Kind distinguishes legitimate quota exhaustion from synthetic
// anti-proxy 429 honeypot/firewall responses.
type Upstream429Kind string

const (
	RateLimit429GenuineQuota  Upstream429Kind = "genuine_quota_429"
	RateLimit429AnomalousTrap Upstream429Kind = "anomalous_trap_429"
)

// Upstream429Classification captures signals extracted from an upstream HTTP 429
// response to decide whether failover to standby accounts is safe.
type Upstream429Classification struct {
	Kind            Upstream429Kind
	HasRetryAfter   bool
	RetryAfter      time.Duration
	HasQuotaDetails bool
	Reason          string
}

// RequestGuardResult describes whether an incoming Cloud Code request must be
// blocked, had bait markers sanitized, or must stay pinned to a single account.
type RequestGuardResult struct {
	Block            bool
	BlockStatus      int
	BlockReason      string
	Sanitized        bool
	SanitizedMarkers []string
	PinSingleAccount bool
	Body             []byte
}

var (
	thirdPartyUARegex    = regexp.MustCompile(`(?i)(cline/|roo-code/|kilo-code/|opencode/|claude-code/|cursor/|aider/|continue/|cpa-proxy|one-api|new-api)`)
	reInterpretedRFC2119 = regexp.MustCompile(`(?i)interpreted as described in RFC\s*-?\s*2119`)
	reRFC2119Colon       = regexp.MustCompile(`(?i)\bRFC\s*-?\s*2119\s*:`)
	reRFC2119Word        = regexp.MustCompile(`(?i)\bRFC\s*-?\s*2119\b`)
	reRFC2119Message     = regexp.MustCompile(`(?i)rfc\s*-?\s*2119`)
)

// InspectAndSanitizeCloudCodeRequest checks incoming Cloud Code proxy calls for
// third-party proxy client signatures and neutralizes synthetic bait markers
// (such as RFC 2119 instructions) across system instructions and message parts.
func InspectAndSanitizeCloudCodeRequest(r *http.Request, body []byte, scope string) RequestGuardResult {
	_ = scope
	res := RequestGuardResult{
		Body: body,
	}

	if r != nil {
		for name := range r.Header {
			low := strings.ToLower(strings.TrimSpace(name))
			if strings.HasPrefix(low, "x-cpa-") ||
				strings.HasPrefix(low, "x-oneapi-") ||
				strings.HasPrefix(low, "x-new-api-") {
				res.Block = true
				res.BlockStatus = http.StatusForbidden
				res.BlockReason = "third_party_client_blocked"
				return res
			}
		}

		if r.URL != nil && strings.HasPrefix(r.URL.Path, "/v1internal") {
			ua := strings.TrimSpace(r.Header.Get("User-Agent"))
			if ua != "" && thirdPartyUARegex.MatchString(ua) {
				res.Block = true
				res.BlockStatus = http.StatusForbidden
				res.BlockReason = "third_party_client_blocked"
				return res
			}
		}
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return res
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil || root == nil {
		return res
	}

	sanitizedAny := false
	if changed, err := sanitizeCloudCodeContainer(root); err == nil && changed {
		sanitizedAny = true
	}
	if reqRaw, ok := root["request"]; ok && len(bytes.TrimSpace(reqRaw)) > 0 {
		var reqMap map[string]json.RawMessage
		if json.Unmarshal(reqRaw, &reqMap) == nil && reqMap != nil {
			if changed, err := sanitizeCloudCodeContainer(reqMap); err == nil && changed {
				if updatedReq, err := marshalJSONNoEscape(reqMap); err == nil {
					root["request"] = updatedReq
					sanitizedAny = true
				}
			}
		}
	}

	if sanitizedAny {
		if updatedRoot, err := marshalJSONNoEscape(root); err == nil {
			res.Sanitized = true
			res.SanitizedMarkers = append(res.SanitizedMarkers, "rfc_2119")
			res.PinSingleAccount = true
			res.Body = updatedRoot
		}
	}

	return res
}

func sanitizeCloudCodeContainer(obj map[string]json.RawMessage) (bool, error) {
	changedAny := false

	for _, key := range []string{"systemInstruction", "system_instruction"} {
		raw, ok := obj[key]
		if !ok || len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var asString string
		if json.Unmarshal(raw, &asString) == nil && asString != "" {
			if next, changed := sanitizeRFC2119Text(asString); changed {
				if encoded, err := marshalJSONNoEscape(next); err == nil {
					obj[key] = encoded
					changedAny = true
				}
			}
			continue
		}
		var sysMap map[string]json.RawMessage
		if json.Unmarshal(raw, &sysMap) == nil && sysMap != nil {
			sysChanged := false
			if textRaw, hasText := sysMap["text"]; hasText && len(bytes.TrimSpace(textRaw)) > 0 {
				var text string
				if json.Unmarshal(textRaw, &text) == nil {
					if next, changed := sanitizeRFC2119Text(text); changed {
						if encoded, err := marshalJSONNoEscape(next); err == nil {
							sysMap["text"] = encoded
							sysChanged = true
						}
					}
				}
			}
			if partsRaw, hasParts := sysMap["parts"]; hasParts && len(bytes.TrimSpace(partsRaw)) > 0 {
				if updatedParts, changed := sanitizePartsSlice(partsRaw); changed {
					sysMap["parts"] = updatedParts
					sysChanged = true
				}
			}
			if sysChanged {
				if encoded, err := marshalJSONNoEscape(sysMap); err == nil {
					obj[key] = encoded
					changedAny = true
				}
			}
		}
	}

	if contentsRaw, ok := obj["contents"]; ok && len(bytes.TrimSpace(contentsRaw)) > 0 {
		var contents []json.RawMessage
		if json.Unmarshal(contentsRaw, &contents) == nil && contents != nil {
			contentsChanged := false
			for i, itemRaw := range contents {
				var itemMap map[string]json.RawMessage
				if json.Unmarshal(itemRaw, &itemMap) != nil || itemMap == nil {
					continue
				}
				partsRaw, hasParts := itemMap["parts"]
				if !hasParts || len(bytes.TrimSpace(partsRaw)) == 0 {
					continue
				}
				if updatedParts, changed := sanitizePartsSlice(partsRaw); changed {
					itemMap["parts"] = updatedParts
					if encoded, err := marshalJSONNoEscape(itemMap); err == nil {
						contents[i] = encoded
						contentsChanged = true
					}
				}
			}
			if contentsChanged {
				if encoded, err := marshalJSONNoEscape(contents); err == nil {
					obj["contents"] = encoded
					changedAny = true
				}
			}
		}
	}

	return changedAny, nil
}

func sanitizePartsSlice(partsRaw json.RawMessage) (json.RawMessage, bool) {
	var parts []json.RawMessage
	if json.Unmarshal(partsRaw, &parts) != nil || parts == nil {
		return partsRaw, false
	}
	partsChanged := false
	for i, partRaw := range parts {
		var partMap map[string]json.RawMessage
		if json.Unmarshal(partRaw, &partMap) != nil || partMap == nil {
			continue
		}
		textRaw, ok := partMap["text"]
		if !ok || len(bytes.TrimSpace(textRaw)) == 0 {
			continue
		}
		var text string
		if json.Unmarshal(textRaw, &text) != nil {
			continue
		}
		if next, changed := sanitizeRFC2119Text(text); changed {
			if encoded, err := marshalJSONNoEscape(next); err == nil {
				partMap["text"] = encoded
				if encodedPart, err := marshalJSONNoEscape(partMap); err == nil {
					parts[i] = encodedPart
					partsChanged = true
				}
			}
		}
	}
	if !partsChanged {
		return partsRaw, false
	}
	encoded, err := marshalJSONNoEscape(parts)
	if err != nil {
		return partsRaw, false
	}
	return encoded, true
}

func sanitizeRFC2119Text(s string) (string, bool) {
	if s == "" {
		return s, false
	}
	out := reInterpretedRFC2119.ReplaceAllString(s, "interpreted strictly as specified")
	out = reRFC2119Colon.ReplaceAllString(out, "Specification:")
	out = reRFC2119Word.ReplaceAllString(out, "strict specification")
	return out, out != s
}

func marshalJSONNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

type upstream429ErrorDetail struct {
	Type       string            `json:"@type"`
	Reason     string            `json:"reason"`
	RetryDelay string            `json:"retryDelay"`
	Violations []json.RawMessage `json:"violations"`
}

type upstream429ErrorBody struct {
	Message string                   `json:"message"`
	Status  string                   `json:"status"`
	Details []upstream429ErrorDetail `json:"details"`
}

var genuineQuotaReasons = map[string]bool{
	"RATE_LIMIT_EXCEEDED":      true,
	"QUOTA_EXCEEDED":           true,
	"RESOURCE_EXHAUSTED":       true,
	"MODEL_CAPACITY_EXHAUSTED": true,
	"USER_PROJECT_DENIED":      true,
}

// ClassifyUpstream429 evaluates an upstream HTTP 429 response and determines
// whether it reflects genuine quota exhaustion or a synthetic anti-proxy trap.
func ClassifyUpstream429(resp *http.Response, body []byte) Upstream429Classification {
	var retryAfter time.Duration
	if resp != nil {
		retryAfter = parseRetryAfterDuration(resp.Header.Get("Retry-After"))
	}
	hasRetryAfter := retryAfter > 0

	errBody, parsedJSON := parseUpstream429Body(body)
	errMessage := errBody.Message
	if !parsedJSON && len(bytes.TrimSpace(body)) > 0 {
		errMessage = string(body)
	}

	hasStructuredQuotaDetails := false
	matchedDetailReason := ""
	for _, detail := range errBody.Details {
		lowType := strings.ToLower(strings.TrimSpace(detail.Type))
		upperReason := strings.ToUpper(strings.TrimSpace(detail.Reason))
		if strings.Contains(lowType, "quotafailure") && len(detail.Violations) > 0 {
			hasStructuredQuotaDetails = true
			if matchedDetailReason == "" {
				matchedDetailReason = "QUOTA_FAILURE"
			}
		}
		if strings.Contains(lowType, "retryinfo") && strings.TrimSpace(detail.RetryDelay) != "" {
			hasStructuredQuotaDetails = true
			if matchedDetailReason == "" {
				matchedDetailReason = "RETRY_INFO"
			}
		}
		if genuineQuotaReasons[upperReason] {
			hasStructuredQuotaDetails = true
			if matchedDetailReason == "" {
				matchedDetailReason = upperReason
			}
		}
	}

	hasPolicySignal := hasAnomalous429PolicyMessage(errMessage) ||
		hasAnomalous429PolicyMessage(errBody.Status)

	hasQuotaDetails := hasStructuredQuotaDetails || (hasRetryAfter && !hasPolicySignal)

	kind := RateLimit429GenuineQuota
	reason := matchedDetailReason
	if hasPolicySignal {
		kind = RateLimit429AnomalousTrap
		reason = "anomalous_429_policy_signal"
	} else if !hasRetryAfter && !hasQuotaDetails {
		kind = RateLimit429AnomalousTrap
		reason = "headerless_429_no_quota_details"
	} else if reason == "" {
		if hasRetryAfter {
			reason = "retry_after_header"
		} else if errBody.Status != "" {
			reason = errBody.Status
		} else {
			reason = "quota_details"
		}
	}

	return Upstream429Classification{
		Kind:            kind,
		HasRetryAfter:   hasRetryAfter,
		RetryAfter:      retryAfter,
		HasQuotaDetails: hasQuotaDetails,
		Reason:          reason,
	}
}

func parseUpstream429Body(body []byte) (upstream429ErrorBody, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return upstream429ErrorBody{}, false
	}

	if trimmed[0] == '[' {
		var arr []json.RawMessage
		if json.Unmarshal(trimmed, &arr) == nil && len(arr) > 0 {
			trimmed = bytes.TrimSpace(arr[0])
		}
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &envelope); err != nil || envelope == nil {
		return upstream429ErrorBody{}, false
	}

	if errRaw, hasErr := envelope["error"]; hasErr && len(bytes.TrimSpace(errRaw)) > 0 {
		var nested upstream429ErrorBody
		if json.Unmarshal(errRaw, &nested) == nil && (nested.Message != "" || nested.Status != "" || len(nested.Details) > 0) {
			return nested, true
		}
		var errStr string
		if json.Unmarshal(errRaw, &errStr) == nil {
			return upstream429ErrorBody{Message: errStr}, true
		}
	}

	var top upstream429ErrorBody
	if json.Unmarshal(trimmed, &top) == nil {
		return top, true
	}
	return upstream429ErrorBody{}, false
}

func hasAnomalous429PolicyMessage(msg string) bool {
	if strings.TrimSpace(msg) == "" {
		return false
	}
	if reRFC2119Message.MatchString(msg) {
		return true
	}
	low := strings.ToLower(msg)
	return strings.Contains(low, "invalid client") ||
		strings.Contains(low, "permission denied") ||
		strings.Contains(low, "permission_denied") ||
		strings.Contains(low, "access denied") ||
		strings.Contains(low, "violat")
}
