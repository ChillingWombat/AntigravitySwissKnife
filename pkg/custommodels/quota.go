package custommodels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// QuotaResult represents the auto-derived balance, quota, and pricing details for a model.
type QuotaResult struct {
	QuotaType            QuotaType `json:"quota_type"`    // "na", "balance", "quota"
	BalanceValue         string    `json:"balance_value"` // e.g. "$12.34" or "¥10.00"
	QuotaValue           string    `json:"quota_value"`   // e.g. "250,000 tokens" or "$10.00"
	Fraction             *float64  `json:"fraction"`      // 0.0 to 1.0; nil if untracked/none
	HasPercentage        bool      `json:"has_percentage"`
	Message              string    `json:"message"`
	InputPricePerM       *float64  `json:"input_price_per_m"`
	CachedInputPricePerM *float64  `json:"cached_input_price_per_m"`
	OutputPricePerM      *float64  `json:"output_price_per_m"`
	PriceSource          string    `json:"price_source,omitempty"`
	// Buckets carries every quota window the provider reported (5H / 7D /
	// monthly ...). Providers that expose a single rolling window leave this
	// empty and are rendered as a single gauge.
	Buckets []QuotaBucket `json:"buckets,omitempty"`
}

// ApplyFetchedQuota copies a live provider result onto a model. Windowed
// quota providers are usage-limited rather than prepaid-dollar accounts, so
// stale legacy budget fields must not override the reported quota fractions.
func ApplyFetchedQuota(model *CustomModel, result QuotaResult) {
	if model == nil {
		return
	}
	model.QuotaType = result.QuotaType
	model.BalanceValue = result.BalanceValue
	model.QuotaValue = result.QuotaValue
	model.QuotaFraction = result.Fraction
	model.QuotaBuckets = result.Buckets
	if len(result.Buckets) > 0 {
		model.TotalBudget = 0
		model.PrepaidBalance = 0
	}
}

// QuotaBucket is one quota window reported by a provider, e.g. the 5H window
// together with a 7D/monthly window of the same plan.
type QuotaBucket struct {
	Label    string   `json:"label"`    // display label, e.g. "5H", "7D", "Monthly"
	Fraction *float64 `json:"fraction"` // 0.0 to 1.0 remaining; nil if untracked
	Value    string   `json:"value,omitempty"`
	Message  string   `json:"message,omitempty"`
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
				return clean + "/usage"
			}
			return clean + "/v1/usage"
		},
		FetchFunc: func(ctx context.Context, client *http.Client, endpoint string, apiKey string) (*QuotaResult, error) {
			clean := strings.TrimSuffix(endpoint, "/v1/usage")
			clean = strings.TrimSuffix(clean, "/usage")
			clean = strings.TrimSuffix(clean, "/api/user/usage")

			candidateEndpoints := []string{
				clean + "/v1/usage",
				clean + "/usage",
				clean + "/v1/models",
				clean + "/models",
			}
			if !strings.Contains(clean, "/zen/go") {
				candidateEndpoints = append(candidateEndpoints,
					clean+"/zen/go/v1/usage",
					clean+"/zen/go/v1/models",
				)
			}

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

				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
					var errResp struct {
						Type  string `json:"type"`
						Error struct {
							Type    string `json:"type"`
							Message string `json:"message"`
						} `json:"error"`
						Message string `json:"message"`
					}
					if json.Unmarshal(body, &errResp) == nil {
						msg := errResp.Error.Message
						if msg == "" {
							msg = errResp.Message
						}
						if strings.Contains(strings.ToLower(msg), "subscription") {
							return &QuotaResult{
								QuotaType:  QuotaTypeQuota,
								QuotaValue: "Sub Required",
								Message:    msg,
							}, nil
						}
					}
					continue
				}

				if resp.StatusCode != http.StatusOK {
					continue
				}

				if buckets := parseOpenCodeUsageBuckets(body); len(buckets) > 0 {
					worst := buckets[0]
					for _, bucket := range buckets[1:] {
						if bucket.Fraction != nil && (worst.Fraction == nil || *bucket.Fraction < *worst.Fraction) {
							worst = bucket
						}
					}
					return &QuotaResult{
						QuotaType:     QuotaTypeQuota,
						Fraction:      worst.Fraction,
						HasPercentage: worst.Fraction != nil,
						Message:       "OpenCode Go usage",
						Buckets:       buckets,
					}, nil
				}

				if rl := ParseRateLimitHeaders(resp.Header); rl != nil {
					return rl, nil
				}

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

// parseOpenCodeUsageBuckets parses the OpenCode Go account usage contract:
// {"usage":{"rolling":{"percent":24},"weekly":{"percent":37},"monthly":{"percent":22}}}.
// OpenCode reports consumed percentages, while quota buckets store remaining
// fractions for the progress bars.
func parseOpenCodeUsageBuckets(body []byte) []QuotaBucket {
	type usageWindow struct {
		Status   string   `json:"status"`
		Percent  *float64 `json:"percent"`
		ResetsAt string   `json:"resetsAt"`
	}
	var doc struct {
		Usage struct {
			Rolling usageWindow `json:"rolling"`
			Weekly  usageWindow `json:"weekly"`
			Monthly usageWindow `json:"monthly"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}

	windows := []struct {
		label string
		value usageWindow
	}{
		{label: "5H", value: doc.Usage.Rolling},
		{label: "7D", value: doc.Usage.Weekly},
		{label: "30D", value: doc.Usage.Monthly},
	}

	buckets := make([]QuotaBucket, 0, len(windows))
	for _, window := range windows {
		if window.value.Percent == nil {
			continue
		}
		used := math.Max(0, math.Min(100, *window.value.Percent))
		remaining := 1 - used/100
		fraction := remaining
		message := fmt.Sprintf("%s quota: %.0f%% used", window.label, used)
		if window.value.ResetsAt != "" {
			message += ", resets " + window.value.ResetsAt
		}
		if window.value.Status == "rate-limited" {
			message += ", rate limited"
		}
		buckets = append(buckets, QuotaBucket{
			Label:    window.label,
			Fraction: &fraction,
			Value:    fmt.Sprintf("%.0f%%", remaining*100),
			Message:  message,
		})
	}
	return buckets
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

	jsonBuckets := ExtractQuotaBucketsFromJSON(body)
	attach := func(res *QuotaResult) *QuotaResult {
		if res != nil && len(res.Buckets) == 0 {
			res.Buckets = jsonBuckets
		}
		return res
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
		return attach(&QuotaResult{
			QuotaType:     QuotaTypeBalance,
			BalanceValue:  fmt.Sprintf("$%.2f", avail),
			Fraction:      &f,
			HasPercentage: true,
			Message:       fmt.Sprintf("Proxy Billing Balance: $%.2f", avail),
		})
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
		return attach(&QuotaResult{
			QuotaType:     QuotaTypeQuota,
			QuotaValue:    fmt.Sprintf("%s tokens", formatTokens(uData.Data.Quota)),
			Fraction:      frac,
			HasPercentage: hasPct,
			Message:       fmt.Sprintf("User Quota: %s tokens", formatTokens(uData.Data.Quota)),
		})
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
			return attach(&QuotaResult{
				QuotaType:     QuotaTypeBalance,
				BalanceValue:  fmt.Sprintf("$%.2f", avail),
				Fraction:      &f,
				HasPercentage: true,
				Message:       fmt.Sprintf("Quota Balance: $%.2f", avail),
			})
		}

		bal := parseRawFloat(genData.Balance)
		if bal == 0 {
			bal = parseRawFloat(genData.Credits)
		}
		if bal == 0 {
			bal = parseRawFloat(genData.Data.Balance)
		}
		if bal > 0 {
			return attach(&QuotaResult{
				QuotaType:    QuotaTypeBalance,
				BalanceValue: fmt.Sprintf("$%.2f", bal),
				Message:      fmt.Sprintf("Balance: $%.2f", bal),
			})
		}

		q := genData.Quota
		if q == 0 {
			q = genData.Data.Quota
		}
		if q == 0 {
			q = genData.TotalTokens
		}
		if q > 0 {
			return attach(&QuotaResult{
				QuotaType:  QuotaTypeQuota,
				QuotaValue: fmt.Sprintf("%s tokens", formatTokens(q)),
				Message:    fmt.Sprintf("Quota: %s tokens", formatTokens(q)),
			})
		}
	}

	// D. Multi-window-only payload: surface the reported windows directly.
	if len(jsonBuckets) > 0 {
		var frac *float64
		worst := jsonBuckets[0]
		for _, b := range jsonBuckets {
			if b.Fraction != nil && (worst.Fraction == nil || *b.Fraction < *worst.Fraction) {
				worst = b
			}
		}
		frac = worst.Fraction
		return &QuotaResult{
			QuotaType:     QuotaTypeQuota,
			Fraction:      frac,
			HasPercentage: frac != nil,
			Message:       "Multi-window quota tracking",
			Buckets:       jsonBuckets,
		}
	}

	return nil
}

// ExtractQuotaBucketsFromJSON parses multi-window quota payloads returned by
// custom quota endpoints. Supported shapes:
//
//	{"buckets":[{"label":"5h","fraction":0.42,"value":"42%"}]}
//	{"data":{"buckets":[...same fields...]}}
//	{"limits":[{"window":"5h","used":42,"limit":100}]}
//	{"data":{"limits":[{"type":"7d","used":42,"limit":100}]}}
//
// Providers that only report a single scalar quota yield no buckets.
func ExtractQuotaBucketsFromJSON(body []byte) []QuotaBucket {
	type rawBucket struct {
		Label     string   `json:"label"`
		Window    string   `json:"window"`
		Name      string   `json:"name"`
		Type      string   `json:"type"`
		Fraction  *float64 `json:"fraction"`
		Remaining *float64 `json:"remaining"`
		Used      *float64 `json:"used"`
		Limit     *float64 `json:"limit"`
		Value     string   `json:"value"`
	}
	type rawDoc struct {
		Buckets []rawBucket `json:"buckets"`
		Limits  []rawBucket `json:"limits"`
		Data    struct {
			Buckets []rawBucket `json:"buckets"`
			Limits  []rawBucket `json:"limits"`
		} `json:"data"`
	}

	var doc rawDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}

	raw := make([]rawBucket, 0, len(doc.Buckets)+len(doc.Limits)+len(doc.Data.Buckets)+len(doc.Data.Limits))
	raw = append(raw, doc.Buckets...)
	raw = append(raw, doc.Limits...)
	raw = append(raw, doc.Data.Buckets...)
	raw = append(raw, doc.Data.Limits...)
	if len(raw) == 0 {
		return nil
	}

	seen := map[string]bool{}
	buckets := make([]QuotaBucket, 0, len(raw))
	for _, rb := range raw {
		window := rb.Label
		if window == "" {
			window = rb.Window
		}
		if window == "" {
			window = rb.Name
		}
		if window == "" {
			window = rb.Type
		}
		label := FormatQuotaWindowLabel(window)
		if seen[label] {
			continue
		}

		var frac *float64
		switch {
		case rb.Fraction != nil:
			f := math.Max(0, math.Min(1, *rb.Fraction))
			frac = &f
		case rb.Remaining != nil && rb.Limit != nil && *rb.Limit > 0:
			f := math.Max(0, math.Min(1, *rb.Remaining / *rb.Limit))
			frac = &f
		case rb.Used != nil && rb.Limit != nil && *rb.Limit > 0:
			f := math.Max(0, math.Min(1, 1-(*rb.Used / *rb.Limit)))
			frac = &f
		}
		if frac == nil && rb.Value == "" {
			// Nothing renderable for this entry; skip it rather than show a 0%.
			continue
		}
		seen[label] = true
		bucket := QuotaBucket{Label: label, Fraction: frac, Value: rb.Value}
		if bucket.Value == "" && frac != nil {
			bucket.Value = fmt.Sprintf("%d%%", int(math.Round(*frac*100)))
		}
		buckets = append(buckets, bucket)
	}
	if len(buckets) == 0 {
		return nil
	}
	return buckets
}

// DetectAndFetchQuota inspects the model configuration and attempts to auto-derive
// balance or quota limits by querying registered providers or inspecting proxy endpoints.
func DetectAndFetchQuota(model CustomModel) QuotaResult {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 0. ProviderGCP models use GCP project-level billing & quota
	if model.ProviderType == ProviderGCP {
		balanceVal := model.BalanceValue
		billingDesc := "Google Cloud Platform Vertex AI Managed"

		saKeyStr := strings.TrimSpace(model.GCPServiceAccountKey)
		if saKeyStr != "" && saKeyStr != "[REDACTED]" {
			if saKey, err := ParseServiceAccountKey([]byte(saKeyStr)); err == nil {
				projID := strings.TrimSpace(model.GCPProjectID)
				if projID == "" {
					projID = saKey.ProjectID
				}
				if bInfo, bErr := FetchGCPBillingInfo(ctx, saKey, projID); bErr == nil && bInfo != nil {
					if bInfo.BillingEnabled {
						billingDesc = fmt.Sprintf("Billing Account: %s", bInfo.BillingAccountName)
						balanceVal = bInfo.StatusMessage
					} else {
						billingDesc = bInfo.StatusMessage
						balanceVal = "Disabled / No Account"
					}
				}
			}
		}

		if balanceVal != "" {
			res := QuotaResult{
				QuotaType:    QuotaTypeBalance,
				QuotaValue:   balanceVal,
				BalanceValue: balanceVal,
				Message:      billingDesc,
			}
			if model.QuotaFraction != nil {
				res.Fraction = model.QuotaFraction
				res.HasPercentage = true
			} else if model.TotalBudget > 0 {
				f := model.PrepaidBalance / model.TotalBudget
				if f < 0 {
					f = 0
				}
				if f > 1 {
					f = 1
				}
				res.Fraction = &f
				res.HasPercentage = true
			} else {
				clean := strings.TrimLeft(strings.TrimSpace(balanceVal), "$¥€£")
				if val, err := strconv.ParseFloat(clean, 64); err == nil && val > 0 {
					one := 1.0
					res.Fraction = &one
					res.HasPercentage = true
				}
			}
			return res
		}

		if model.QuotaType == QuotaTypeBalance {
			return QuotaResult{
				QuotaType:    QuotaTypeBalance,
				QuotaValue:   "Active (GCP Billing)",
				BalanceValue: "Active (GCP Billing)",
				Message:      billingDesc,
			}
		}

		return QuotaResult{
			QuotaType:  QuotaTypeQuota,
			QuotaValue: "GCP Managed",
			Message:    billingDesc,
		}
	}

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
	cleanBase = strings.TrimSuffix(cleanBase, "/v1")

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
	// Multi-window providers (e.g. Claude's unified rate limit headers) expose a
	// 5H window and a 7D window side by side. They must all be surfaced, not just
	// the first one that happens to match.
	buckets := parseWindowBucketsFromHeaders(headers)

	var primary *QuotaResult

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

		primary = &QuotaResult{
			QuotaType:     QuotaTypeQuota,
			QuotaValue:    quotaVal,
			Fraction:      frac,
			HasPercentage: frac != nil,
			Message:       "Rate limit token tracking",
		}
	}

	// 2. Requests remaining & limit
	if primary == nil {
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
				primary = &QuotaResult{
					QuotaType:     QuotaTypeQuota,
					QuotaValue:    "", // percentage only -> renders 'Quota'
					Fraction:      &f,
					HasPercentage: true,
					Message:       "Rate limit request percentage",
				}
			}
		}
	}

	// 3. Windowed headers only: derive the headline value from the most
	// constrained window (lowest remaining fraction) for backwards compatibility.
	if primary == nil && len(buckets) > 0 {
		worst := buckets[0]
		for _, b := range buckets {
			if b.Fraction != nil && (worst.Fraction == nil || *b.Fraction < *worst.Fraction) {
				worst = b
			}
		}
		primary = &QuotaResult{
			QuotaType:     QuotaTypeQuota,
			QuotaValue:    "",
			Fraction:      worst.Fraction,
			HasPercentage: worst.Fraction != nil,
			Message:       fmt.Sprintf("Rate limit window %s", worst.Label),
		}
	}

	if primary == nil {
		return nil
	}
	primary.Buckets = buckets
	return primary
}

// windowBucketHeaderRe matches rate-limit headers that describe a specific
// rolling window, e.g. anthropic-ratelimit-unified-5h-utilization or
// x-ratelimit-7d-remaining / x-ratelimit-7d-limit.
var windowBucketHeaderRe = regexp.MustCompile(`^(?:anthropic-ratelimit-unified|anthropic-ratelimit|x-ratelimit)-([0-9]+[a-z]|[a-z]+)-(utilization|remaining|limit)$`)

// FormatQuotaWindowLabel renders a quota window name as a compact display label:
// "5h" -> "5H", "7d"/"weekly" -> "7D" (matching the dashboard convention),
// "monthly" -> "Monthly".
func FormatQuotaWindowLabel(window string) string {
	w := strings.ToLower(strings.TrimSpace(window))
	switch w {
	case "", "all", "overall":
		return "Quota"
	case "5h", "5hr", "5hour", "5hours":
		return "5H"
	case "7d", "7day", "7days", "weekly", "week":
		return "7D"
	case "30d", "30day", "30days", "monthly", "month":
		return "30D"
	case "24h", "daily", "day":
		return "24H"
	}
	if len(w) <= 3 {
		return strings.ToUpper(w)
	}
	return strings.ToUpper(w[:1]) + w[1:]
}

// parseWindowBucketsFromHeaders extracts every rolling-window quota advertised
// via rate-limit headers. Utilization-style headers report a *consumed*
// fraction, so they are inverted into remaining fractions.
func parseWindowBucketsFromHeaders(headers http.Header) []QuotaBucket {
	type windowState struct {
		window string
		util   *float64
		remain *float64
		limit  *float64
	}
	states := map[string]*windowState{}

	getWindow := func(window string) *windowState {
		key := strings.ToLower(window)
		st, ok := states[key]
		if !ok {
			st = &windowState{window: key}
			states[key] = st
		}
		return st
	}

	for key, values := range headers {
		m := windowBucketHeaderRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(key)))
		if m == nil || len(values) == 0 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(values[0]), 64)
		if err != nil {
			continue
		}
		st := getWindow(m[1])
		switch m[2] {
		case "utilization":
			val := v
			st.util = &val
		case "remaining":
			val := v
			st.remain = &val
		case "limit":
			val := v
			st.limit = &val
		}
	}

	if len(states) == 0 {
		return nil
	}

	// Deterministic order: shorter windows first (5H before 7D before Monthly).
	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buckets := make([]QuotaBucket, 0, len(states))
	for _, k := range keys {
		st := states[k]
		var frac *float64
		switch {
		case st.util != nil:
			f := 1.0 - *st.util
			f = math.Max(0, math.Min(1, f))
			frac = &f
		case st.remain != nil && st.limit != nil && *st.limit > 0:
			f := *st.remain / *st.limit
			f = math.Max(0, math.Min(1, f))
			frac = &f
		}
		bucket := QuotaBucket{
			Label:    FormatQuotaWindowLabel(st.window),
			Fraction: frac,
		}
		if frac != nil {
			bucket.Value = fmt.Sprintf("%d%%", int(math.Round(*frac*100)))
		}
		buckets = append(buckets, bucket)
	}
	return buckets
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
