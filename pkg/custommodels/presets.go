package custommodels

// ProviderPreset provides default templates for popular LLM providers.
type ProviderPreset struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	ProviderType   ProviderType `json:"provider_type"`
	DefaultBaseURL string       `json:"default_base_url"`
	CallingFormat  string       `json:"calling_format"`
	Description    string       `json:"description"`
	AuthHeader     string       `json:"auth_header"`
	PopularModels  []string     `json:"popular_models"`
}

// GetPresets returns the list of pre-configured provider presets.
func GetPresets() []ProviderPreset {
	return []ProviderPreset{
		{
			ID:             "custom",
			Name:           "Custom / Manual Endpoint",
			ProviderType:   ProviderOpenAI,
			DefaultBaseURL: "",
			CallingFormat:  "OpenAI / Anthropic / Gemini Compatible",
			Description:    "Enter your own custom endpoint URL, model identifier, and credentials.",
			AuthHeader:     "Custom API Key / Bearer token",
			PopularModels:  []string{},
		},
		{
			ID:             "openai",
			Name:           "OpenAI (Official)",
			ProviderType:   ProviderOpenAI,
			DefaultBaseURL: "https://api.openai.com/v1",
			CallingFormat:  "/v1/chat/completions",
			Description:    "Standard OpenAI Chat Completions API format (ChatGPT, GPT-4o, o1, o3-mini)",
			AuthHeader:     "Authorization: Bearer <API_KEY>",
			PopularModels:  []string{"gpt-4o", "gpt-4o-mini", "o1", "o3-mini", "gpt-4-turbo"},
		},
		{
			ID:             "deepseek",
			Name:           "DeepSeek Cloud",
			ProviderType:   ProviderOpenAI,
			DefaultBaseURL: "https://api.deepseek.com",
			CallingFormat:  "/v1/chat/completions",
			Description:    "DeepSeek V3 and R1 reasoning models (OpenAI-compatible protocol)",
			AuthHeader:     "Authorization: Bearer <API_KEY>",
			PopularModels:  []string{"deepseek-chat", "deepseek-reasoner"},
		},
		{
			ID:             "groq",
			Name:           "Groq Cloud",
			ProviderType:   ProviderOpenAI,
			DefaultBaseURL: "https://api.groq.com/openai/v1",
			CallingFormat:  "/v1/chat/completions",
			Description:    "Ultra-low latency LPU cloud inference (OpenAI-compatible protocol)",
			AuthHeader:     "Authorization: Bearer <API_KEY>",
			PopularModels:  []string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768"},
		},
		{
			ID:             "openrouter",
			Name:           "OpenRouter",
			ProviderType:   ProviderOpenAI,
			DefaultBaseURL: "https://openrouter.ai/api/v1",
			CallingFormat:  "/v1/chat/completions",
			Description:    "Unified routing gateway supporting 100+ models with prepaid deposit credits",
			AuthHeader:     "Authorization: Bearer <API_KEY>",
			PopularModels:  []string{"anthropic/claude-3.7-sonnet", "deepseek/deepseek-r1", "meta-llama/llama-3.3-70b-instruct"},
		},
		{
			ID:             "anthropic",
			Name:           "Anthropic Claude",
			ProviderType:   ProviderAnthropic,
			DefaultBaseURL: "https://api.anthropic.com/v1",
			CallingFormat:  "/v1/messages",
			Description:    "Anthropic Messages API format (Claude 3.7 Sonnet, Claude 3.5 Haiku)",
			AuthHeader:     "x-api-key: <API_KEY>",
			PopularModels:  []string{"claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"},
		},
		{
			ID:             "gemini",
			Name:           "Google Gemini API",
			ProviderType:   ProviderGemini,
			DefaultBaseURL: "https://generativelanguage.googleapis.com",
			CallingFormat:  "/v1beta/models/{model}:generateContent",
			Description:    "Google AI Studio / Gemini generateContent endpoint with your own API key",
			AuthHeader:     "x-goog-api-key: <API_KEY>",
			PopularModels:  []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-pro"},
		},
		{
			ID:             "ollama",
			Name:           "Ollama (Local LLM)",
			ProviderType:   ProviderOpenAI,
			DefaultBaseURL: "http://localhost:11434/v1",
			CallingFormat:  "/v1/chat/completions",
			Description:    "Local Ollama server running Llama 3.3, DeepSeek R1, Qwen 2.5 (no API key needed)",
			AuthHeader:     "None required",
			PopularModels:  []string{"llama3.3:70b", "deepseek-r1:8b", "deepseek-r1:14b", "qwen2.5-coder:32b", "mistral:latest"},
		},
		{
			ID:             "vllm",
			Name:           "vLLM / LM Studio (Local)",
			ProviderType:   ProviderOpenAI,
			DefaultBaseURL: "http://localhost:1234/v1",
			CallingFormat:  "/v1/chat/completions",
			Description:    "Local self-hosted OpenAI-compatible inference engine or LM Studio (localhost:1234/v1)",
			AuthHeader:     "None or Bearer token",
			PopularModels:  []string{"local-model"},
		},
	}
}
