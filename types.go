package bcr

import "strings"

// Metadata contains information about a module in the registry.
//
// This corresponds to the metadata.json file in a Bazel registry.
type Metadata struct {
	// Versions lists all available versions in registry order.
	// The last element is typically the most recent version.
	Versions []string `json:"versions"`

	// YankedVersions maps version strings to yank reasons.
	// A yanked version should not be used; the reason explains why.
	YankedVersions map[string]string `json:"yanked_versions,omitempty"`

	// Maintainers lists the module maintainers.
	Maintainers []Maintainer `json:"maintainers,omitempty"`

	// Homepage is the module's homepage URL.
	Homepage string `json:"homepage,omitempty"`

	// Repository lists source repository identifiers (e.g., "github:owner/repo").
	Repository []string `json:"repository,omitempty"`
}

// IsYanked reports whether the given version is yanked.
func (m *Metadata) IsYanked(version string) bool {
	if m == nil || m.YankedVersions == nil {
		return false
	}
	_, ok := m.YankedVersions[version]
	return ok
}

// YankReason returns the yank reason for a version, or empty string if not yanked.
func (m *Metadata) YankReason(version string) string {
	if m == nil || m.YankedVersions == nil {
		return ""
	}
	return m.YankedVersions[version]
}

// Latest returns the latest non-yanked version, or empty string if none available.
func (m *Metadata) Latest() string {
	if m == nil || len(m.Versions) == 0 {
		return ""
	}
	// Iterate from end (newest) to find first non-yanked
	for i := len(m.Versions) - 1; i >= 0; i-- {
		v := m.Versions[i]
		if !m.IsYanked(v) {
			return v
		}
	}
	return ""
}

// LatestStable returns the latest non-yanked, non-prerelease version.
// Falls back to the latest non-yanked prerelease if no stable version exists.
// Returns empty string if all versions are yanked.
func (m *Metadata) LatestStable() string {
	if m == nil || len(m.Versions) == 0 {
		return ""
	}

	// First pass: find latest stable (non-prerelease, non-yanked)
	for i := len(m.Versions) - 1; i >= 0; i-- {
		v := m.Versions[i]
		if m.IsYanked(v) {
			continue
		}
		if !IsPrerelease(v) {
			return v
		}
	}

	// Second pass: any non-yanked version (including prerelease)
	for i := len(m.Versions) - 1; i >= 0; i-- {
		v := m.Versions[i]
		if !m.IsYanked(v) {
			return v
		}
	}

	return ""
}

// prereleaseIndicators are common version string patterns indicating prereleases.
var prereleaseIndicators = []string{"-rc", "-alpha", "-beta", "-dev", "-pre"}

// IsPrerelease reports whether a version string indicates a prerelease.
// Checks for common prerelease indicators: -rc, -alpha, -beta, -dev, -pre
func IsPrerelease(version string) bool {
	for _, indicator := range prereleaseIndicators {
		if strings.Contains(version, indicator) {
			return true
		}
	}
	return false
}

// HasVersion reports whether the given version exists.
func (m *Metadata) HasVersion(version string) bool {
	if m == nil {
		return false
	}
	for _, v := range m.Versions {
		if v == version {
			return true
		}
	}
	return false
}

// Source describes how to fetch a module version's source code.
//
// This corresponds to the source.json file in a Bazel registry.
type Source struct {
	// Type is the source type. Common values:
	//   - "archive" (default if empty): fetch via http_archive
	//   - "git_repository": fetch via git_repository
	//   - "local_path": local filesystem path
	Type string `json:"type,omitempty"`

	// URL is the download URL for archive sources.
	URL string `json:"url,omitempty"`

	// Integrity is the Subresource Integrity hash (e.g., "sha256-...").
	// Used to verify the downloaded archive.
	Integrity string `json:"integrity,omitempty"`

	// StripPrefix is the directory prefix to strip from archive contents.
	StripPrefix string `json:"strip_prefix,omitempty"`

	// Patches maps patch file names to their integrity hashes.
	// Patches are applied in sorted order by filename.
	Patches map[string]string `json:"patches,omitempty"`

	// PatchStrip is the number of leading path components to strip
	// when applying patches (equivalent to patch -p).
	PatchStrip int `json:"patch_strip,omitempty"`

	// ArchiveType overrides automatic archive type detection.
	// Examples: "zip", "tar.gz", "tar.bz2".
	ArchiveType string `json:"archive_type,omitempty"`

	// --- Git repository fields ---

	// Remote is the git repository URL (for git_repository type).
	Remote string `json:"remote,omitempty"`

	// Commit is the git commit hash (for git_repository type).
	Commit string `json:"commit,omitempty"`

	// ShallowSince limits git history for faster clones (for git_repository type).
	ShallowSince string `json:"shallow_since,omitempty"`

	// --- Local path fields ---

	// Path is the local filesystem path (for local_path type).
	Path string `json:"path,omitempty"`
}

// SourceType returns the effective source type, defaulting to "archive".
func (s *Source) SourceType() string {
	if s == nil || s.Type == "" {
		return "archive"
	}
	return s.Type
}

// Maintainer represents a module maintainer.
type Maintainer struct {
	// Name is the maintainer's display name.
	Name string `json:"name,omitempty"`

	// Email is the maintainer's email address.
	Email string `json:"email,omitempty"`

	// GitHub is the maintainer's GitHub username.
	GitHub string `json:"github,omitempty"`

	// GitHubID is the maintainer's GitHub user ID (for identity verification).
	GitHubID int64 `json:"github_user_id,omitempty"`
}
