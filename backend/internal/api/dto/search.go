package dto

// SearchResp is the response of GET /search/tasks.
type SearchResp struct {
	Results []SearchResultItem `json:"results"`
}

// SearchResultItem is one row of the task search output.
type SearchResultItem struct {
	Course     string   `json:"course"`
	Track      string   `json:"track"`
	Topic      string   `json:"topic"`
	Unit       string   `json:"unit"`
	Task       string   `json:"task"`
	Title      string   `json:"title"`
	Tags       []string `json:"tags"`
	Difficulty int      `json:"difficulty,omitempty"`
	Rank       string   `json:"rank"` // title-word | tag | title | content
	Snippet    string   `json:"snippet,omitempty"`
}

// SearchCatalogResp is the response of GET /search/catalog.
type SearchCatalogResp struct {
	Tasks []SearchCatalogEntry `json:"tasks"`
}

// SearchCatalogEntry is a lightweight task entry used to build AI search prompts.
type SearchCatalogEntry struct {
	Course     string   `json:"course"`
	Track      string   `json:"track"`
	Topic      string   `json:"topic"`
	Unit       string   `json:"unit"`
	Task       string   `json:"task"`
	Title      string   `json:"title"`
	Tags       []string `json:"tags,omitempty"`
	Difficulty int      `json:"difficulty,omitempty"`
}
