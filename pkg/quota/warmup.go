package quota

import (
	"encoding/json"
	"time"
)

// OneTokenPayload creates the lightweight 1-token keep-alive request structure.
type OneTokenPayload struct {
	Contents         []ContentItem    `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}

type ContentItem struct {
	Parts []PartItem `json:"parts"`
}

type PartItem struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

// Build1TokenKeepAliveJSON serializes a minimal ping payload to activate the next horizon.
func Build1TokenKeepAliveJSON() ([]byte, error) {
	p := OneTokenPayload{
		Contents: []ContentItem{
			{
				Parts: []PartItem{
					{Text: "ping"},
				},
			},
		},
		GenerationConfig: GenerationConfig{
			MaxOutputTokens: 1,
		},
	}
	return json.Marshal(p)
}

// WarmupScheduler tracks when to execute keep-alives based on lead time.
type WarmupScheduler struct {
	LeadTime time.Duration
}

// ShouldTrigger returns true if current time is within [resetTime - LeadTime, resetTime].
func (ws *WarmupScheduler) ShouldTrigger(resetTime time.Time, now time.Time) bool {
	if resetTime.IsZero() {
		return false
	}
	triggerWindowStart := resetTime.Add(-ws.LeadTime)
	return now.After(triggerWindowStart) || now.Equal(triggerWindowStart)
}
