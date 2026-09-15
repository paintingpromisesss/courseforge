package ai

type openAIRequest struct {
	Model              string                 `json:"model"`
	Messages           []message              `json:"messages"`
	Stream             bool                   `json:"stream,omitempty"`
	EnableThinking     *bool                  `json:"enable_thinking,omitempty"`
	ReasoningEffort    string                 `json:"reasoning_effort,omitempty"`
	ChatTemplateKwargs map[string]interface{} `json:"chat_template_kwargs,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type anthropicRequest struct {
	Model        string             `json:"model"`
	Messages     []anthMsg          `json:"messages"`
	System       string             `json:"system,omitempty"`
	Stream       bool               `json:"stream,omitempty"`
	MaxTokens    int                `json:"max_tokens"`
	Thinking     *anthropicThinking `json:"thinking,omitempty"`
}

type anthMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIModelInfo struct {
	ID string `json:"id"`
}

type openAIModelsResponse struct {
	Data []openAIModelInfo `json:"data"`
}
