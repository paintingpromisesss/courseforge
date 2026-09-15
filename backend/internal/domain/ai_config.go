package domain

import "encoding/json"

type AIConfig struct {
	Provider     Provider          `json:"provider"`
	BaseURL      string            `json:"base_url"`
	APIKey       string            `json:"api_key"`
	Model        string            `json:"model"`
	SystemPrompt string            `json:"system_prompt"`
	Enabled      bool              `json:"enabled"`
	ProviderKeys map[string]string `json:"provider_keys,omitempty"`
}

type Provider int

const (
	ProviderOpenAI Provider = iota
	ProviderAnthropic
)

func (p Provider) String() string {
	switch p {
	case ProviderOpenAI:
		return "openai"
	case ProviderAnthropic:
		return "anthropic"
	default:
		return "openai"
	}
}

func (p *Provider) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "anthropic":
		*p = ProviderAnthropic
	default:
		*p = ProviderOpenAI
	}
	return nil
}

func (p Provider) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}
