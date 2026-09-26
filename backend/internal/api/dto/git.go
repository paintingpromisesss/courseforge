package dto

// GitAuthStatus reports GitHub auth state without exposing the raw token.
type GitAuthStatus struct {
	Configured  bool   `json:"configured"`
	Username    string `json:"username"`
	TokenMasked string `json:"token_masked"`
}

// PatchGitAuthReq sets (or with an empty token, clears) the GitHub PAT.
type PatchGitAuthReq struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

// GitAuthTestResp is the result of validating the token against the GitHub API.
type GitAuthTestResp struct {
	OK    bool   `json:"ok"`
	Login string `json:"login,omitempty"`
	Error string `json:"error,omitempty"`
}

// GitImportReq imports a course or catalog from a GitHub repository URL.
type GitImportReq struct {
	URL    string `json:"url"`
	Branch string `json:"branch"` // optional
}

// GitImportResp is returned by import, checkout and pull.
type GitImportResp struct {
	Slug   string `json:"slug"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

// CourseSourceDTO describes where an imported course came from.
type CourseSourceDTO struct {
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit"`
	ImportedAt string `json:"imported_at"`
}

// GitBranchesResp lists remote branches of an imported course.
type GitBranchesResp struct {
	Branches []string         `json:"branches"`
	Current  string           `json:"current"`
	Source   *CourseSourceDTO `json:"source"`
}

// GitStatusResp is the git state of an imported course directory.
type GitStatusResp struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

// GitCheckoutReq switches an imported course to another branch.
type GitCheckoutReq struct {
	Branch string `json:"branch"`
	Force  bool   `json:"force"`
}
