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

// GitImportBatchReq is a list of repository URLs to import in one call.
// Branch is intentionally not per-URL: the default branch is cloned and the
// user switches branches in the course UI afterwards.
type GitImportBatchReq struct {
	URLs []string `json:"urls"`
}

// GitImportBatchItem is the per-URL outcome of a batch import.
type GitImportBatchItem struct {
	URL    string `json:"url"`
	OK     bool   `json:"ok"`
	Slug   string `json:"slug,omitempty"`
	Branch string `json:"branch,omitempty"`
	Commit string `json:"commit,omitempty"`
	Error  string `json:"error,omitempty"`
}

// GitImportBatchResp reports every URL's outcome; one bad URL never stops
// the rest of the batch.
type GitImportBatchResp struct {
	Results []GitImportBatchItem `json:"results"`
}

// GitAttachReq binds an existing local course to a remote repository.
// The repo is cloned, its .git moves into the course directory — course
// files are never touched.
type GitAttachReq struct {
	URL  string `json:"url"`
	Slug string `json:"slug"`
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

// GitCheckoutReq switches an imported course to another branch; for pull it
// also carries the update mode ("" = refuse on dirty, "merge" = stash+ff+pop,
// force = discard local edits).
type GitCheckoutReq struct {
	Branch string `json:"branch"`
	Force  bool   `json:"force"`
	Mode   string `json:"mode,omitempty"`
}
