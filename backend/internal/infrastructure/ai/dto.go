package ai

type openAIRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string    `json:"model"`
	Messages  []anthMsg `json:"messages"`
	System    string    `json:"system,omitempty"`
	Stream    bool      `json:"stream,omitempty"`
	MaxTokens int       `json:"max_tokens"`
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
