package storeedgemanager

import "time"

type WorkerStatus struct {
	State         string     `json:"state"`
	PID           int        `json:"pid,omitempty"`
	Version       string     `json:"version,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	LastExitAt    *time.Time `json:"last_exit_at,omitempty"`
	LastExitError string     `json:"last_exit_error,omitempty"`
	Healthy       bool       `json:"healthy"`
}

type LifecycleStatus struct {
	ManagerVersion  string       `json:"manager_version"`
	OS              string       `json:"os"`
	Arch            string       `json:"arch"`
	ServiceMode     string       `json:"service_mode"`
	Worker          WorkerStatus `json:"worker"`
	UpdateEnabled   bool         `json:"update_enabled"`
	UpdateState     string       `json:"update_state"`
	LatestVersion   string       `json:"latest_version,omitempty"`
	UpdateAvailable bool         `json:"update_available"`
	LastUpdateError string       `json:"last_update_error,omitempty"`
	ManifestURL     string       `json:"manifest_url,omitempty"`
}

type UpdateAsset struct {
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}

type UpdateManifest struct {
	Version      string      `json:"version"`
	Platform     string      `json:"platform"`
	PublishedAt  string      `json:"published_at,omitempty"`
	ReleaseNotes string      `json:"release_notes,omitempty"`
	Worker       UpdateAsset `json:"worker"`
	Manager      UpdateAsset `json:"manager,omitempty"`
}

type UpdateCheck struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	PublishedAt     string `json:"published_at,omitempty"`
}
