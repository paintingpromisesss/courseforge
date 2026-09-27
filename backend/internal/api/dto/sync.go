package dto

// SyncTriggers mirrors repo.SyncTriggers in the API contract.
type SyncTriggers struct {
	OnProgress    bool `json:"on_progress"`
	IntervalMin   int  `json:"interval_min"`
	OnStartupPull bool `json:"on_startup_pull"`
}

// SyncConfigResp is the cloud-sync configuration as served to the frontend.
type SyncConfigResp struct {
	RemoteURL string       `json:"remote_url"`
	Branch    string       `json:"branch"`
	Triggers  SyncTriggers `json:"triggers"`
	LastSync  string       `json:"last_sync,omitempty"`
	Enabled   bool         `json:"enabled"`
	Exclude   []string     `json:"exclude,omitempty"`
}

// PatchSyncConfigReq updates the sync config. Nil fields are left unchanged.
type PatchSyncConfigReq struct {
	RemoteURL *string       `json:"remote_url"`
	Branch    *string       `json:"branch"`
	Triggers  *SyncTriggers `json:"triggers"`
	Enabled   *bool         `json:"enabled"`
	Exclude   *[]string     `json:"exclude"`
}

// SyncStatusResp reports whether sync is configured and its current state.
type SyncStatusResp struct {
	Configured     bool     `json:"configured"`
	Enabled        bool     `json:"enabled"`
	Syncing        bool     `json:"syncing"`
	LastSync       string   `json:"last_sync"`
	Branch         string   `json:"branch"`
	Commit         string   `json:"commit"`
	PendingImports []string `json:"pending_imports"`
}

// SyncHistoryItem is one commit in the sync repo history.
type SyncHistoryItem struct {
	Commit  string `json:"commit"`
	Author  string `json:"author"`
	Subject string `json:"subject"`
	Time    string `json:"time"`
}

// SyncCommitFilesResp lists the files that changed in a sync commit.
type SyncCommitFilesResp struct {
	Files []string `json:"files"`
}

// RollbackReq checks out a previous sync commit.
type RollbackReq struct {
	Commit string `json:"commit"`
}

// SyncRestoreImportsResp lists course dirs re-cloned from their own remotes.
type SyncRestoreImportsResp struct {
	Restored []string `json:"restored"`
}
