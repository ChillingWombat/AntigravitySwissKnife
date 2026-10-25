package custommodels

import "testing"

func TestNormalizeCanonicalNativeModel_ClaudeOpus55(t *testing.T) {
	tests := []struct {
		modelID     string
		displayName string
		wantID      string
		wantName    string
	}{
		{"claude-opus-5-5", "", "claude-opus-5-5", "Claude Opus 5.5"},
		{"claude-opus-5.5", "", "claude-opus-5-5", "Claude Opus 5.5"},
		{"claude-opus-5-5-thinking", "", "claude-opus-5-5", "Claude Opus 5.5"},
		{"claude-opus-5.5-thinking", "Claude Opus 5.5 (Thinking)", "claude-opus-5-5", "Claude Opus 5.5"},
	}
	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			modelID, displayName := NormalizeCanonicalNativeModel(tt.modelID, tt.displayName)
			if modelID != tt.wantID {
				t.Errorf("expected modelID %q, got %q", tt.wantID, modelID)
			}
			if displayName != tt.wantName {
				t.Errorf("expected displayName %q, got %q", tt.wantName, displayName)
			}
		})
	}
}

func TestOfficialPricing_ClaudeOpus55(t *testing.T) {
	for _, modelID := range []string{"claude-opus-5-5", "claude-opus-5-5-thinking"} {
		t.Run(modelID, func(t *testing.T) {
			input, cached, output, ok := lookupOfficialProviderPricing(modelID, "anthropic")
			if !ok || input == nil || cached == nil || output == nil {
				t.Fatalf("expected bundled pricing for %s, got in=%v cached=%v out=%v ok=%v", modelID, input, cached, output, ok)
			}
			if !almostEqual(*input, 15.00) || !almostEqual(*cached, 1.50) || !almostEqual(*output, 75.00) {
				t.Errorf("expected 15.00/1.50/75.00, got %.2f/%.2f/%.2f", *input, *cached, *output)
			}
		})
	}
}
