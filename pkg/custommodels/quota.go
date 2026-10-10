package custommodels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QuotaResult represents the auto-derived balance, quota, and pricing details for a model.
type QuotaResult struct {
	QuotaType            QuotaType `json:"quota_type"`     // "na", "balance", "quota"
	BalanceValue         string    `json:"balance_value"`  // e.g. "$12.34" or "¥10.00"
	QuotaValue           string    `json:"quota_value"`    // e.g. "250,000 tokens" or "$10.00"
	Fraction             *float64  `json:"fraction"`       // 0.0 to 1.0; nil if untracked/none
	HasPercentage        bool      `json:"has_percentage"`
	Message              string    `json:"message"`
	InputPricePerM       *float64  `json:"input_price_per_m"`
	CachedInputPricePerM *float64  `json:"cached_input_price_per_m"`
	OutputPricePerM      *float64  `json:"output_price_per_m"`
	PriceSource          string    `json:"price_source,omitempty"`
}

// ProviderQuotaRegistryEntry defines a known provider's balance/quota endpoint spec.
type ProviderQuotaRegistryEntry struct {
	Name         string
	HostPatterns []string // substrings to match in baseURL hostname
	QuotaType    QuotaType
	EndpointFunc func(baseURL string) string
	FetchFunc    func(ctx context.Context, client *http.Client, endpoint string, apiKey string) (*QuotaResult, error)
}

// ProviderQuotaRegistry is our dictionary/table of known providers and their balance/quota endpoints.
var ProviderQuotaRegistry = []ProviderQuotaRegistryEntry{
	// 1. OpenRouter (Prepaid credits or key limit)
	{
		Name:         "OpenRouter",
		HostPatterns: []string{"openrouter.ai"},
		QuotaType:    QuotaTypeBalance,
		EndpointFunc: func(baseURL string) string {
			return "https://openrouter.ai/api/v1/credits"
		},
		FetchFunc: func(ctx context.Context, client *http.Client, endpoint string, apiKey string) (*QuotaResult, error) {
			if apiKey == "" {
				return nil, fmt.Errorf("no api key provided")
			}
			req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				var cResp struct {
					Data struct {
						TotalCredits float64 `json:"total_credits"`
						TotalUsage   float64 `json:"total_usage"`
					} `json:"data"`
				}
				if json.Unmarshal(body, &cResp) == nil {
					total := cResp.Data.TotalCredits
					used := cResp.Data.TotalUsage
					avail := total - used
					if avail < 0 {
						avail = 0
					}
					var frac *float64
					if total > 0 {
						f := avail / total
						f = math.Max(0, math.Min(1, f))
						frac = &f
					}
					return &QuotaResult{
						QuotaType:     QuotaTypeBalance,
						BalanceValue:  fmt.Sprintf("$%.2f", avail),
						Fraction:      frac,
						HasPercentage: frac != nil,
						Message:       fmt.Sprintf("OpenRouter Balance: $%.2f", avail),
					}, nil
				}
			}

			// Fallback: check /api/v1/auth/key
			keyEndpoint := "https://openrouter.ai/api/v1/auth/key"
			kReq, err := http.NewRequestWithContext(ctx, "GET", keyEndpoint, nil)
			if err == nil {
				kReq.Header.Set("Authorization", "Bearer "+apiKey)
				kResp, err := client.Do(kReq)
				if err == nil {
					defer kResp.Body.Close()
					if kResp.StatusCode == http.StatusOK {
						kBody, _ := io.ReadAll(kResp.Body)
						var kData struct {
							Data struct {
								Usage float64  `json:"usage"`
								Limit *float64 `json:"limit"`
							} `json:"data"`
						}
						if json.Unmarshal(kBody, &kData) == nil && kData.Data.Limit != nil && *kData.Data.Limit > 0 {
							limit := *kData.Data.Limit
							avail := limit - kData.Data.Usage
							if avail < 0 {
								avail = 0
							}
							f := avail / limit
							f = math.Max(0, math.Min(1, f))
							return &QuotaResult{
								QuotaType:     QuotaTypeBalance,
								BalanceValue:  fmt.Sprintf("$%.2f", avail),
								Fraction:      &f,
								HasPercentage: true,
								Message:       fmt.Sprintf("OpenRouter Key Limit Balance: $%.2f", avail),
							}, nil
						}
					}
				}
			}

			return nil, fmt.Errorf("openrouter credits query returned HTTP %d", resp.StatusCode)
		},
	},

	// 2. DeepSeek (Account Balance)
	{
		Name:         "DeepSeek",
		HostPatterns: []string{"deepseek.com"},
		QuotaType:    QuotaTypeBalance,
		EndpointFunc: func(baseURL string) string {
			return "https://api.deepseek.com/user/balance"
		},
		FetchFunc: func(ctx context.Context, client *http.Client, endpoint string, apiKey string) (*QuotaResult, error) {
			if apiKey == "" {
				return nil, fmt.Errorf("no api key provided")
			}
			req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Accept", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("deepseek balance returned HTTP %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			var dsResp struct {
				IsAvailable  bool `json:"is_available"`
				BalanceInfos []struct {
					Currency        string          `json:"currency"`
					TotalBalance    json.RawMessage `json:"total_balance"`
					GrantedBalance  json.RawMessage `json:"granted_balance"`
					ToppedUpBalance json.RawMessage `json:"topped_up_balance"`
				} `json:"balance_infos"`
			}
			if err := json.Unmarshal(body, &dsResp); err != nil {
				return nil, err
			}

			var bestCurrency string
			var bestAmount float64
			hasAny := false

			for _, b := range dsResp.BalanceInfos {
				amt := parseRawFloat(b.TotalBalance)
				hasAny = true
				if amt > 0 || bestCurrency == "" {
					bestCurrency = strings.ToUpper(b.Currency)
					bestAmount = amt
				}
			}

			if !hasAny {
				return nil, fmt.Errorf("no balance_infos in deepseek response")
			}

			symbol := "¥"
			if bestCurrency == "USD" {
				symbol = "$"
			}
			balanceStr := fmt.Sprintf("%s%.2f", symbol, bestAmount)

			return &QuotaResult{
				QuotaType:    QuotaTypeBalance,
				BalanceValue: balanceStr,
				Message:      fmt.Sprintf("DeepSeek balance: %s", balanceStr),
			}, nil
		},
	},

	// 3. SiliconFlow (User Info Balance)
	{
		Name:         "SiliconFlow",
		HostPatterns: []string{"siliconflow.cn", "siliconflow.com"},
		QuotaType:    QuotaTypeBalance,
		EndpointFunc: func(baseURL string) string {
			return "https://api.siliconflow.cn/v1/user/info"
		},
		FetchFunc: func(ctx context.Context, client *http.Client, endpoint string, apiKey string) (*QuotaResult, error) {
			if apiKey == "" {
				return nil, fmt.Errorf("no api key provided")
			}
			req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Accept", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("siliconflow returned HTTP %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			var sfResp struct {
				Data struct {
					TotalBalance json.RawMessage `json:"totalBalance"`
					Balance      json.RawMessage `json:"balance"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &sfResp); err != nil {
				return nil, err
			}

			amount := parseRawFloat(sfResp.Data.TotalBalance)
			if amount == 0 {
				amount = parseRawFloat(sfResp.Data.Balance)
			}
			balanceStr := fmt.Sprintf("¥%.2f", amount)

			return &QuotaResult{
				QuotaType:    QuotaTypeBalance,
				BalanceValue: balanceStr,
				Message:      fmt.Sprintf("SiliconFlow balance: %s", balanceStr),
			}, nil
		},
	},

	// 4. Moonshot AI / Kimi (User Balance)
	{
		Name:         "Moonshot",
		HostPatterns: []string{"moonshot.cn", "kimi.com"},
		QuotaType:    QuotaTypeBalance,
		EndpointFunc: func(baseURL string) string {
			return "https://api.moonshot.cn/v1/users/me/balance"
		},
		FetchFunc: func(ctx context.Context, client *http.Client, endpoint string, apiKey string) (*QuotaResult, error) {
			if apiKey == "" {
				return nil, fmt.Errorf("no api key provided")
			}
			req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Accept", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("moonshot returned HTTP %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			var msResp struct {
				Data struct {
					AvailableBalance float64 `json:"available_balance"`
					CashBalance      float64 `json:"cash_balance"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &msResp); err != nil {
				return nil, err
			}

			val := msResp.Data.AvailableBalance
			if val == 0 && msResp.Data.CashBalance > 0 {
				val = msResp.Data.CashBalance
			}
			balanceStr := fmt.Sprintf("¥%.2f", val)

			return &QuotaResult{
				QuotaType:    QuotaTypeBalance,
				BalanceValue: balanceStr,
				Message:      fmt.Sprintf("Moonshot balance: %s", balanceStr),
			}, nil
		},
	},

	// 5. Together AI (Account info)
	{
		Name:         "TogetherAI",
		HostPatterns: []string{"together.xyz", "together.ai"},
		QuotaType:    QuotaTypeBalance,
		EndpointFunc: func(baseURL string) string {
			return "https://api.together.xyz/v1/users/me"
		},
		FetchFunc: func(ctx context.Context, client *http.Client, endpoint string, apiKey string) (*QuotaResult, error) {
			if apiKey == "" {
				return nil, fmt.Errorf("no api key provided")
			}
			req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("together returned HTTP %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			var tgResp struct {
				Credits float64 `json:"credits"`
				Balance float64 `json:"balance"`
			}
			if err := json.Unmarshal(body, &tgResp); err == nil {
				val := tgResp.Credits
				if val == 0 {
					val = tgResp.Balance
				}
				if val > 0 {
					return &QuotaResult{
						QuotaType:    QuotaTypeBalance,
						BalanceValue: fmt.Sprintf("$%.2f", val),
					}, nil
				}
			}
			return nil, fmt.Errorf("no credits or balance found in together response")
		},
	},

	// 6. OpenCode (opencode.ai)
	{
		Name:         "OpenCode",
		HostPatterns: []string{"opencode.ai"},
		QuotaType:    QuotaTypeBalance,
		EndpointFunc: func(baseURL string) string {
			clean := strings.TrimRight(baseURL, "/")
			if strings.HasSuffix(clean, "/v1") {
				clean = strings.TrimSuffix(clean, "/v1")
			}
			return clean + "/api/user/usage"
		},
		FetchFunc: func(ctx context.Context, client *http.Client, endpoint string, apiKey string) (*QuotaResult, error) {
			candidateEndpoints := []string{endpoint}
			clean := strings.TrimSuffix(endpoint, "/api/user/usage")
			candidateEndpoints = append(candidateEndpoints, clean+"/v1/usage", clean+"/usage", "https://opencode.ai/api/user/usage")

			for _, ep := range candidateEndpoints {
				req, err := http.NewRequestWithContext(ctx, "GET", ep, nil)
				if err != nil {
					continue
				}
				if apiKey != "" {
					req.Header.Set("Authorization", "Bearer "+apiKey)
					req.Header.Set("x-api-key", apiKey)
				}
				req.Header.Set("x-opencode-session", fmt.Sprintf("swiss-quota-%d", time.Now().Unix()))

				resp, err := client.Do(req)
				if err != nil {
					continue
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					continue
				}

				if rl := ParseRateLimitHeaders(resp.Header); rl != nil {
					return rl, nil
				}

				body, _ := io.ReadAll(resp.Body)
				var ocData struct {
					Data struct {
						Balance      json.RawMessage `json:"balance"`
						TotalCredits json.RawMessage `json:"total_credits"`
						TotalUsage   json.RawMessage `json:"total_usage"`
						Quota        int64           `json:"quota"`
					} `json:"data"`
					Balance      json.RawMessage `json:"balance"`
					Credits      json.RawMessage `json:"credits"`
					TotalCredits json.RawMessage `json:"total_credits"`
					TotalUsage   json.RawMessage `json:"total_usage"`
					Quota        int64           `json:"quota"`
				}
				if err := json.Unmarshal(body, &ocData); err == nil {
					bal := parseRawFloat(ocData.Balance)
					if bal == 0 {
						bal = parseRawFloat(ocData.Credits)
					}
					if bal == 0 {
						bal = parseRawFloat(ocData.Data.Balance)
					}
					totCred := parseRawFloat(ocData.TotalCredits)
					if totCred == 0 {
						totCred = parseRawFloat(ocData.Data.TotalCredits)
					}
					totUse := parseRawFloat(ocData.TotalUsage)
					if totUse == 0 {
						totUse = parseRawFloat(ocData.Data.TotalUsage)
					}
					if totCred > 0 {
						avail := totCred - totUse
						if avail < 0 {
							avail = 0
						}
						var frac *float64
						f := avail / totCred
						f = math.Max(0, math.Min(1, f))
						frac = &f
						return &QuotaResult{
							QuotaType:     QuotaTypeBalance,
							BalanceValue:  fmt.Sprintf("$%.2f", avail),
							Fraction:      frac,
							HasPercentage: true,
							Message:       fmt.Sprintf("OpenCode balance: $%.2f", avail),
						}, nil
					}
					if bal > 0 {
						return &QuotaResult{
							QuotaType:    QuotaTypeBalance,
							BalanceValue: fmt.Sprintf("$%.2f", bal),
							Message:      fmt.Sprintf("OpenCode balance: $%.2f", bal),
						}, nil
					}
					q := ocData.Quota
					if q == 0 {
						q = ocData.Data.Quota
					}
					if q > 0 {
						return &QuotaResult{
							QuotaType:  QuotaTypeQuota,
							QuotaValue: fmt.Sprintf("%s tokens", formatTokens(q)),
							Message:    fmt.Sprintf("OpenCode quota: %s tokens", formatTokens(q)),
						}, nil
					}
				}
			}
			return nil, fmt.Errorf("no balance or quota found in opencode response")
		},
	},
}

// probeQuotaEndpoint makes a GET request to the given endpoint and parses rate-limit
// headers or JSON usage/balance/quota payloads.
func probeQuotaEndpoint(ctx context.Context, client *http.Client, endpoint string, apiKey string) *QuotaResult {
	if strings.TrimSpace(endpoint) == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("x-api-key", apiKey)
	}
	req.Header.Set("x-opencode-session", fmt.Sprintf("swiss-probe-%d", time.Now().Unix()))

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// 1. Check rate limit headers
	if rl := ParseRateLimitHeaders(resp.Header); rl != nil {
		return rl
	}

	// 2. Parse JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return nil
	}

	// A. OneAPI billing subscription: {"hard_limit_usd": 50.0, "total_usage": 1500}
	var subData struct {
		HardLimitUSD float64 `json:"hard_limit_usd"`
		TotalUsage   float64 `json:"total_usage"`
	}
	if json.Unmarshal(body, &subData) == nil && subData.HardLimitUSD > 0 {
		avail := subData.HardLimitUSD - (subData.TotalUsage / 100.0)
		if avail < 0 {
			avail = 0
		}
		f := avail / subData.HardLimitUSD
		f = math.Max(0, math.Min(1, f))
		return &QuotaResult{
			QuotaType:     QuotaTypeBalance,
			BalanceValue:  fmt.Sprintf("$%.2f", avail),
			Fraction:      &f,
			HasPercentage: true,
			Message:       fmt.Sprintf("Proxy Billing Balance: $%.2f", avail),
		}
	}

	// B. OneAPI user token quota: {"success": true, "data": {"quota": 1000000, "used_quota": ...}}
	var uData struct {
		Success bool `json:"success"`
		Data    struct {
			Quota     int64 `json:"quota"`
			UsedQuota int64 `json:"used_quota"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &uData) == nil && uData.Success && uData.Data.Quota > 0 {
		var frac *float64
		hasPct := false
		if uData.Data.UsedQuota > 0 && uData.Data.Quota >= uData.Data.UsedQuota {
			f := float64(uData.Data.Quota-uData.Data.UsedQuota) / float64(uData.Data.Quota)
			f = math.Max(0, math.Min(1, f))
			frac = &f
			hasPct = true
		}
		return &QuotaResult{
			QuotaType:     QuotaTypeQuota,
			QuotaValue:    fmt.Sprintf("%s tokens", formatTokens(uData.Data.Quota)),
			Fraction:      frac,
			HasPercentage: hasPct,
			Message:       fmt.Sprintf("User Quota: %s tokens", formatTokens(uData.Data.Quota)),
		}
	}

	// C. General Usage / Balance / Credits / Quota JSON
	var genData struct {
		Data struct {
			Balance      json.RawMessage `json:"balance"`
			TotalCredits json.RawMessage `json:"total_credits"`
			TotalUsage   json.RawMessage `json:"total_usage"`
			Quota        int64           `json:"quota"`
			UsedQuota    int64           `json:"used_quota"`
		} `json:"data"`
		Balance      json.RawMessage `json:"balance"`
		Credits      json.RawMessage `json:"credits"`
		TotalCredits json.RawMessage `json:"total_credits"`
		TotalUsage   json.RawMessage `json:"total_usage"`
		TotalTokens  int64           `json:"total_tokens"`
		Quota        int64           `json:"quota"`
	}
	if json.Unmarshal(body, &genData) == nil {
		totCred := parseRawFloat(genData.TotalCredits)
		if totCred == 0 {
			totCred = parseRawFloat(genData.Data.TotalCredits)
		}
		totUse := parseRawFloat(genData.TotalUsage)
		if totUse == 0 {
			totUse = parseRawFloat(genData.Data.TotalUsage)
		}
		if totCred > 0 {
			avail := totCred - totUse
			if avail < 0 {
				avail = 0
			}
			f := avail / totCred
			f = math.Max(0, math.Min(1, f))
			return &QuotaResult{
				QuotaType:     QuotaTypeBalance,
				BalanceValue:  fmt.Sprintf("$%.2f", avail),
				Fraction:      &f,
				HasPercentage: true,
				Message:       fmt.Sprintf("Quota Balance: $%.2f", avail),
			}
		}

		bal := parseRawFloat(genData.Balance)
		if bal == 0 {
			bal = parseRawFloat(genData.Credits)
		}
		if bal == 0 {
			bal = parseRawFloat(genData.Data.Balance)
		}
		if bal > 0 {
			return &QuotaResult{
				QuotaType:    QuotaTypeBalance,
				BalanceValue: fmt.Sprintf("$%.2f", bal),
				Message:      fmt.Sprintf("Balance: $%.2f", bal),
			}
		}

		q := genData.Quota
		if q == 0 {
			q = genData.Data.Quota
		}
		if q == 0 {
			q = genData.TotalTokens
		}
		if q > 0 {
			return &QuotaResult{
				QuotaType:  QuotaTypeQuota,
				QuotaValue: fmt.Sprintf("%s tokens", formatTokens(q)),
				Message:    fmt.Sprintf("Quota: %s tokens", formatTokens(q)),
			}
		}
	}

	return nil
}

// DetectAndFetchQuota inspects the model configuration and attempts to auto-derive
// balance or quota limits by querying registered providers or inspecting proxy endpoints.
func DetectAndFetchQuota(model CustomModel) QuotaResult {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 0. Check custom quota endpoint if explicitly specified
	if strings.TrimSpace(model.CustomQuotaEndpoint) != "" {
		if res := probeQuotaEndpoint(ctx, client, strings.TrimSpace(model.CustomQuotaEndpoint), model.APIKey); res != nil {
			return *res
		}
	}

	rawBase := strings.TrimSpace(model.BaseURL)
	if rawBase == "" {
		return UntrackedQuotaResult("Base URL is empty")
	}

	parsed, err := url.Parse(rawBase)
	if err != nil {
		return UntrackedQuotaResult(fmt.Sprintf("Invalid URL: %v", err))
	}

	host := strings.ToLower(parsed.Host)
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	// 1. Check known provider dictionary entries
	for _, entry := range ProviderQuotaRegistry {
		for _, pattern := range entry.HostPatterns {
			if strings.Contains(host, pattern) {
				ep := entry.EndpointFunc(rawBase)
				res, err := entry.FetchFunc(ctx, client, ep, model.APIKey)
				if err == nil && res != nil {
					return *res
				}
				// If query failed or returned no value, return N/A
				return UntrackedQuotaResult(fmt.Sprintf("%s query failed: %v", entry.Name, err))
			}
		}
	}

	// 2. Probe common quota and billing proxy endpoints
	cleanBase := strings.TrimRight(rawBase, "/")
	if strings.HasSuffix(cleanBase, "/v1") {
		cleanBase = strings.TrimSuffix(cleanBase, "/v1")
	}

	suffixes := []string{
		"/v1/usage",
		"/usage",
		"/api/user/usage",
		"/dashboard/billing/subscription",
		"/api/user/self",
	}

	for _, suffix := range suffixes {
		if res := probeQuotaEndpoint(ctx, client, cleanBase+suffix, model.APIKey); res != nil {
			return *res
		}
	}

	// 3. Not in storage dictionary or API does not return a value -> N/A
	return UntrackedQuotaResult("Base URL not in recognized quota providers, or no balance/quota value returned.")
}

// ParseRateLimitHeaders inspects HTTP headers from a test or completion response
// to extract standard rate-limit quota metrics (e.g. Groq, Anthropic, OpenAI).
func ParseRateLimitHeaders(headers http.Header) *QuotaResult {
	// 1. Tokens remaining & limit
	remTokensStr := headers.Get("x-ratelimit-remaining-tokens")
	if remTokensStr == "" {
		remTokensStr = headers.Get("anthropic-ratelimit-tokens-remaining")
	}
	limitTokensStr := headers.Get("x-ratelimit-limit-tokens")
	if limitTokensStr == "" {
		limitTokensStr = headers.Get("anthropic-ratelimit-tokens-limit")
	}

	if remTokensStr != "" {
		remTokens, _ := strconv.ParseInt(remTokensStr, 10, 64)
		limitTokens, _ := strconv.ParseInt(limitTokensStr, 10, 64)

		var frac *float64
		if limitTokens > 0 {
			f := float64(remTokens) / float64(limitTokens)
			f = math.Max(0, math.Min(1, f))
			frac = &f
		}

		quotaVal := ""
		if remTokens > 0 {
			quotaVal = fmt.Sprintf("%s tokens", formatTokens(remTokens))
		}

		return &QuotaResult{
			QuotaType:     QuotaTypeQuota,
			QuotaValue:    quotaVal,
			Fraction:      frac,
			HasPercentage: frac != nil,
			Message:       "Rate limit token tracking",
		}
	}

	// 2. Requests remaining & limit
	remReqStr := headers.Get("x-ratelimit-remaining-requests")
	if remReqStr == "" {
		remReqStr = headers.Get("anthropic-ratelimit-requests-remaining")
	}
	limitReqStr := headers.Get("x-ratelimit-limit-requests")
	if limitReqStr == "" {
		limitReqStr = headers.Get("anthropic-ratelimit-requests-limit")
	}

	if remReqStr != "" && limitReqStr != "" {
		remReq, _ := strconv.ParseInt(remReqStr, 10, 64)
		limitReq, _ := strconv.ParseInt(limitReqStr, 10, 64)
		if limitReq > 0 {
			f := float64(remReq) / float64(limitReq)
			f = math.Max(0, math.Min(1, f))
			return &QuotaResult{
				QuotaType:     QuotaTypeQuota,
				QuotaValue:    "", // percentage only -> renders 'Quota'
				Fraction:      &f,
				HasPercentage: true,
				Message:       "Rate limit request percentage",
			}
		}
	}

	return nil
}

// UntrackedQuotaResult returns a default N/A quota result.
func UntrackedQuotaResult(msg string) QuotaResult {
	return QuotaResult{
		QuotaType:     QuotaTypeNA,
		BalanceValue:  "",
		QuotaValue:    "",
		Fraction:      nil,
		HasPercentage: false,
		Message:       msg,
	}
}

// parseRawFloat parses a JSON raw message into a float64 (handles string or float).
func parseRawFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	str := strings.Trim(strings.TrimSpace(string(raw)), "\"")
	if str == "null" || str == "" {
		return 0
	}
	v, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0
	}
	return v
}

// formatTokens formats integer token counts cleanly (e.g. 250,000 or 1.5M).
func formatTokens(tokens int64) string {
	if tokens >= 1_000_000 && tokens%100_000 == 0 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000.0)
	}
	if tokens >= 10_000 && tokens%1_000 == 0 {
		return fmt.Sprintf("%dk", tokens/1_000)
	}
	in := strconv.FormatInt(tokens, 10)
	var out strings.Builder
	n := len(in)
	for i, c := range in {
		if i > 0 && (n-i)%3 == 0 {
			out.WriteRune(',')
		}
		out.WriteRune(c)
	}
	return out.String()
}
