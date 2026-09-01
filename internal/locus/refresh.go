package locus

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	maxArchiveBytes   = 64 << 20
	maxArchiveEntries = 10000
)

var gitCommitPattern = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

type RefreshOptions struct {
	Home                    string
	Store                   *Store
	GitExecutable           string
	HTTPClient              *http.Client
	AllowRegression         bool
	ExpectedCandidateDigest string
}

type RefreshedSource struct {
	OwnerScopeID     string   `json:"owner_scope_id"`
	AliasPath        []string `json:"alias_path"`
	ScopeID          string   `json:"scope_id"`
	ContentDigest    string   `json:"content_digest"`
	ResolvedRevision string   `json:"resolved_revision,omitempty"`
}

type RefreshError struct {
	OwnerScopeID string   `json:"owner_scope_id"`
	AliasPath    []string `json:"alias_path"`
	Reason       string   `json:"reason"`
}

type RefreshResult struct {
	Status            string              `json:"status"`
	Activated         []RefreshedSource   `json:"activated"`
	Retained          []RefreshedSource   `json:"retained"`
	RefreshErrors     []RefreshError      `json:"refresh_errors"`
	Completeness      Completeness        `json:"completeness"`
	BlockedImports    []BlockedImport     `json:"blocked_imports"`
	ActiveSnapshot    *DependencySnapshot `json:"active_snapshot,omitempty"`
	CandidateSnapshot *DependencySnapshot `json:"candidate_snapshot,omitempty"`
	Diff              *DependencyDiff     `json:"diff,omitempty"`
}

type refreshWalker struct {
	ctx                context.Context
	options            RefreshOptions
	layout             HomeLayout
	target             []string
	result             RefreshResult
	visited            map[string]bool
	matched            bool
	activations        []sourceActivation
	pending            []RefreshedSource
	sourceOverrides    map[SourceCacheKey]SourceCacheEntry
	authorityOverrides map[string]ScopeAuthority
	authorityConflicts map[string]bool
}

func RefreshRegistry(ctx context.Context, root, aliasPath string, options RefreshOptions) (RefreshResult, error) {
	if options.Store == nil {
		return RefreshResult{}, errors.New("refresh requires a Store")
	}
	if options.Home == "" {
		var err error
		options.Home, err = DefaultHome()
		if err != nil {
			return RefreshResult{}, err
		}
	}
	layout := HomeLayout{
		Root: options.Home, Registry: filepath.Join(options.Home, "registry"),
		Objects: filepath.Join(options.Home, "cache", "objects"), Candidates: filepath.Join(options.Home, "cache", "candidates"),
	}
	if err := os.MkdirAll(layout.Objects, 0o755); err != nil {
		return RefreshResult{}, err
	}
	if err := os.MkdirAll(layout.Candidates, 0o755); err != nil {
		return RefreshResult{}, err
	}
	var target []string
	if aliasPath != "" {
		target = strings.Split(aliasPath, "::")
		for _, alias := range target {
			if !validDeclarationName(alias) {
				return RefreshResult{}, fmt.Errorf("invalid alias path %q", aliasPath)
			}
		}
	}
	rootRegistry, err := LoadScopeRegistry(root, false)
	if err != nil {
		return RefreshResult{}, err
	}
	activeView, err := CollectRegistry(root, CollectorOptions{Home: options.Home, Store: options.Store})
	if err != nil {
		return RefreshResult{}, err
	}
	activeSnapshot := SnapshotDependency(activeView)
	walker := &refreshWalker{
		ctx: ctx, options: options, layout: layout, target: target, visited: map[string]bool{},
		sourceOverrides: map[SourceCacheKey]SourceCacheEntry{}, authorityOverrides: map[string]ScopeAuthority{},
		authorityConflicts: map[string]bool{},
		result: RefreshResult{
			Activated: []RefreshedSource{}, Retained: []RefreshedSource{}, RefreshErrors: []RefreshError{},
			ActiveSnapshot: &activeSnapshot,
		},
	}
	walker.walk(rootRegistry, nil)
	if len(target) != 0 && !walker.matched {
		return RefreshResult{}, fmt.Errorf("refresh target %q is not a remote import", aliasPath)
	}
	candidateView, err := CollectRegistry(root, CollectorOptions{
		Home: options.Home, Store: options.Store,
		SourceOverrides: walker.sourceOverrides, AuthorityOverrides: walker.authorityOverrides,
	})
	if err != nil {
		return RefreshResult{}, err
	}
	candidateSnapshot := SnapshotDependency(candidateView)
	diff := DiffDependencies(activeSnapshot, candidateSnapshot)
	walker.result.CandidateSnapshot = &candidateSnapshot
	walker.result.Diff = &diff
	walker.result.Completeness = candidateView.Completeness
	walker.result.BlockedImports = append([]BlockedImport{}, candidateView.BlockedImports...)
	if options.ExpectedCandidateDigest != "" && options.ExpectedCandidateDigest != candidateSnapshot.SnapshotDigest {
		return RefreshResult{}, errors.New("refresh candidate changed; review the new dependency diff")
	}
	if diff.RequiresConfirmation && options.AllowRegression && options.ExpectedCandidateDigest == "" {
		return RefreshResult{}, errors.New("confirmed refresh requires the reviewed candidate snapshot digest")
	}
	if diff.RequiresConfirmation && !options.AllowRegression {
		walker.result.Status = "confirmation_required"
		walker.sortResult()
		return walker.result, nil
	}
	if err := options.Store.activateSources(ctx, walker.activations); err != nil {
		return RefreshResult{}, err
	}
	walker.result.Activated = append(walker.result.Activated, walker.pending...)
	switch {
	case len(walker.result.RefreshErrors) == 0:
		walker.result.Status = "success"
	case len(walker.result.Activated) != 0 || len(walker.result.Retained) != 0:
		walker.result.Status = "partial"
	default:
		walker.result.Status = "failure"
	}
	walker.sortResult()
	return walker.result, nil
}

func (w *refreshWalker) sortResult() {
	sort.Slice(w.result.Activated, func(i, j int) bool {
		return strings.Join(w.result.Activated[i].AliasPath, "::") < strings.Join(w.result.Activated[j].AliasPath, "::")
	})
	sort.Slice(w.result.Retained, func(i, j int) bool {
		return strings.Join(w.result.Retained[i].AliasPath, "::") < strings.Join(w.result.Retained[j].AliasPath, "::")
	})
	sort.Slice(w.result.RefreshErrors, func(i, j int) bool {
		return strings.Join(w.result.RefreshErrors[i].AliasPath, "::") < strings.Join(w.result.RefreshErrors[j].AliasPath, "::")
	})
}

func (w *refreshWalker) walk(scope *ScopeRegistry, prefix []string) {
	key := scope.Manifest.ScopeID + "@" + scope.Digest
	if w.visited[key] {
		return
	}
	w.visited[key] = true
	for _, alias := range sortedMapKeys(scope.Manifest.Imports) {
		imported := scope.Manifest.Imports[alias]
		aliasPath := append(append([]string(nil), prefix...), alias)
		if len(w.target) != 0 && !pathPrefix(aliasPath, w.target) && !pathPrefix(w.target, aliasPath) {
			continue
		}
		shouldRefresh := imported.Source.Kind != "directory" && (len(w.target) == 0 || equalPath(aliasPath, w.target))
		if shouldRefresh {
			w.matched = true
			activation, refreshed, retained, reason := w.refreshEdge(scope, imported, aliasPath)
			if reason != "" {
				w.result.RefreshErrors = append(w.result.RefreshErrors, RefreshError{OwnerScopeID: scope.Manifest.ScopeID, AliasPath: aliasPath, Reason: reason})
				_ = w.options.Store.recordRefreshFailure(w.ctx, scope.Manifest.ScopeID, imported, reason)
				if retained != nil {
					w.result.Retained = append(w.result.Retained, *retained)
				}
			} else if activation != nil {
				w.stage(*activation)
				if refreshed != nil {
					w.pending = append(w.pending, *refreshed)
				}
				if retained != nil {
					w.result.Retained = append(w.result.Retained, *retained)
				}
			}
		}
		child, _ := w.loadCurrentChild(scope, imported)
		if child != nil && (len(w.target) == 0 || pathPrefix(aliasPath, w.target)) {
			w.walk(child, aliasPath)
		}
	}
}

func (w *refreshWalker) stage(activation sourceActivation) {
	entry := activation.Entry
	activation.UpdateAuthority = true
	if authority, ok := w.authorityOverrides[entry.ActualScopeID]; ok && authority.ActiveContentDigest != entry.ActiveContentDigest {
		w.authorityConflicts[entry.ActualScopeID] = true
		delete(w.authorityOverrides, entry.ActualScopeID)
		for index := range w.activations {
			if w.activations[index].Entry.ActualScopeID == entry.ActualScopeID {
				w.activations[index].UpdateAuthority = false
			}
		}
	}
	if w.authorityConflicts[entry.ActualScopeID] {
		activation.UpdateAuthority = false
	} else {
		w.authorityOverrides[entry.ActualScopeID] = ScopeAuthority{
			ScopeID: entry.ActualScopeID, ActiveContentDigest: entry.ActiveContentDigest,
			ObjectPath: entry.ObjectPath, Provenance: sanitizeSource(activation.Source),
		}
	}
	w.activations = append(w.activations, activation)
	w.sourceOverrides[SourceCacheKey{OwnerScopeID: entry.OwnerScopeID, ImportAlias: entry.ImportAlias}] = entry
}

func (w *refreshWalker) refreshEdge(owner *ScopeRegistry, imported Import, aliasPath []string) (*sourceActivation, *RefreshedSource, *RefreshedSource, string) {
	previous, _ := w.options.Store.SourceCacheEntry(owner.Manifest.ScopeID, imported.Alias)
	candidate, err := os.MkdirTemp(w.layout.Candidates, "refresh-")
	if err != nil {
		return nil, nil, retainedSource(previous, aliasPath), "source_unavailable"
	}
	defer os.RemoveAll(candidate)
	registryRoot := ""
	resolvedRevision := ""
	etag, lastModified := "", ""
	switch imported.Source.Kind {
	case "git":
		registryRoot, resolvedRevision, err = w.fetchGit(imported.Source, candidate)
	case "url":
		var notModified bool
		registryRoot, etag, lastModified, notModified, err = w.fetchURL(imported.Source, candidate, previous)
		if notModified && previous != nil {
			entry := *previous
			entry.ConfiguredSourceDigest = sourceDigest(imported.Source)
			return &sourceActivation{Entry: entry, Source: imported.Source}, nil, retainedSource(previous, aliasPath), ""
		}
	default:
		return nil, nil, retainedSource(previous, aliasPath), "source_unavailable"
	}
	if err != nil {
		return nil, nil, retainedSource(previous, aliasPath), "source_unavailable"
	}
	registry, err := LoadScopeRegistry(registryRoot, true)
	if err != nil {
		return nil, nil, retainedSource(previous, aliasPath), "invalid_registry"
	}
	if imported.ExpectedScopeID != "" && imported.ExpectedScopeID != registry.Manifest.ScopeID {
		return nil, nil, retainedSource(previous, aliasPath), "scope_id_mismatch"
	}
	objectPath := filepath.Join(w.layout.Objects, strings.TrimPrefix(registry.Digest, "sha256:"))
	if _, statErr := os.Stat(objectPath); os.IsNotExist(statErr) {
		if err := os.Rename(registryRoot, objectPath); err != nil {
			return nil, nil, retainedSource(previous, aliasPath), "source_unavailable"
		}
	}
	entry := SourceCacheEntry{
		OwnerScopeID: owner.Manifest.ScopeID, ImportAlias: imported.Alias, ConfiguredSourceDigest: sourceDigest(imported.Source),
		ExpectedScopeID: imported.ExpectedScopeID, ActualScopeID: registry.Manifest.ScopeID,
		ActiveContentDigest: registry.Digest, ResolvedRevision: resolvedRevision, ObjectPath: objectPath,
		ETag: etag, LastModified: lastModified,
	}
	refreshed := &RefreshedSource{
		OwnerScopeID: owner.Manifest.ScopeID, AliasPath: aliasPath, ScopeID: registry.Manifest.ScopeID,
		ContentDigest: registry.Digest, ResolvedRevision: resolvedRevision,
	}
	return &sourceActivation{Entry: entry, Source: imported.Source}, refreshed, nil, ""
}

func (w *refreshWalker) loadCurrentChild(owner *ScopeRegistry, imported Import) (*ScopeRegistry, error) {
	if imported.Source.Kind == "directory" {
		path, err := resolveDirectorySource(imported.Source, owner.Root, w.options.Home)
		if err != nil {
			return nil, err
		}
		return LoadScopeRegistry(path, false)
	}
	key := SourceCacheKey{OwnerScopeID: owner.Manifest.ScopeID, ImportAlias: imported.Alias}
	if entry, ok := w.sourceOverrides[key]; ok {
		return LoadScopeRegistry(entry.ObjectPath, true)
	}
	entry, err := w.options.Store.SourceCacheEntry(owner.Manifest.ScopeID, imported.Alias)
	if err != nil || entry == nil || entry.ObjectPath == "" {
		return nil, err
	}
	return LoadScopeRegistry(entry.ObjectPath, true)
}

func (w *refreshWalker) fetchGit(source Source, candidate string) (string, string, error) {
	executable := w.options.GitExecutable
	if executable == "" {
		executable = os.Getenv("LOCUS_GIT_EXECUTABLE")
	}
	if executable == "" {
		executable = "git"
	}
	repository := filepath.Join(candidate, "repo")
	if err := runDiscarded(w.ctx, executable, "clone", "--no-checkout", "--", source.URI, repository); err != nil {
		return "", "", err
	}
	revision := source.Revision
	if revision == "" {
		revision = "HEAD"
	}
	command := exec.CommandContext(w.ctx, executable, "-C", repository, "rev-parse", "--verify", revision+"^{commit}")
	command.Stderr = io.Discard
	output, err := command.Output()
	if err != nil {
		return "", "", errors.New("git revision resolution failed")
	}
	commit := strings.TrimSpace(string(output))
	if !gitCommitPattern.MatchString(commit) {
		return "", "", errors.New("git returned an invalid commit")
	}
	if err := runDiscarded(w.ctx, executable, "-C", repository, "checkout", "--detach", "--force", commit); err != nil {
		return "", "", err
	}
	if err := os.RemoveAll(filepath.Join(repository, ".git")); err != nil {
		return "", "", errors.New("Git metadata cleanup failed")
	}
	root, err := containedSourcePath(repository, source.Path)
	return root, commit, err
}

func runDiscarded(ctx context.Context, executable string, args ...string) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return errors.New("external source command failed")
	}
	return nil
}

func (w *refreshWalker) fetchURL(source Source, candidate string, previous *SourceCacheEntry) (string, string, string, bool, error) {
	client := w.options.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(w.ctx, http.MethodGet, source.URI, nil)
	if err != nil {
		return "", "", "", false, err
	}
	if previous != nil && previous.ConfiguredSourceDigest == sourceDigest(source) {
		if previous.ETag != "" {
			request.Header.Set("If-None-Match", previous.ETag)
		}
		if previous.LastModified != "" {
			request.Header.Set("If-Modified-Since", previous.LastModified)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", "", "", false, errors.New("URL fetch failed")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		if previous == nil || previous.ConfiguredSourceDigest != sourceDigest(source) {
			return "", "", "", false, errors.New("URL returned not modified without matching cache metadata")
		}
		return "", previous.ETag, previous.LastModified, true, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "", "", false, errors.New("URL fetch returned a non-success status")
	}
	limited := io.LimitReader(response.Body, maxArchiveBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil || len(payload) > maxArchiveBytes {
		return "", "", "", false, errors.New("URL archive exceeds limit")
	}
	root := filepath.Join(candidate, "archive")
	if err := extractRegistryZIP(payload, root); err != nil {
		return "", "", "", false, err
	}
	root, err = containedSourcePath(root, source.Path)
	return root, response.Header.Get("ETag"), response.Header.Get("Last-Modified"), false, err
}

func extractRegistryZIP(payload []byte, root string) error {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return errors.New("URL payload is not a valid ZIP Registry")
	}
	if len(reader.File) > maxArchiveEntries {
		return errors.New("ZIP Registry has too many entries")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	var expanded int64
	for _, entry := range reader.File {
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		clean := filepath.Clean(filepath.FromSlash(name))
		if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("ZIP Registry contains an unsafe path")
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return errors.New("ZIP Registry symlink is not allowed")
		}
		target := filepath.Join(root, clean)
		if !pathContainedBy(root, target) {
			return errors.New("ZIP Registry path escapes extraction root")
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		remaining := int64(maxArchiveBytes) - expanded
		if entry.UncompressedSize64 > uint64(remaining) {
			return errors.New("ZIP Registry expanded size exceeds limit")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return errors.New("ZIP Registry entry cannot be read")
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			input.Close()
			return err
		}
		copied, copyErr := io.Copy(output, io.LimitReader(input, remaining+1))
		closeErr := output.Close()
		input.Close()
		expanded += copied
		if copyErr != nil || closeErr != nil || copied > remaining {
			return errors.New("ZIP Registry entry extraction failed")
		}
	}
	return nil
}

func containedSourcePath(root, subpath string) (string, error) {
	if subpath == "" {
		return root, nil
	}
	candidate := filepath.Join(root, filepath.FromSlash(subpath))
	if !pathContainedBy(root, candidate) {
		return "", errors.New("source.path escapes source root")
	}
	return candidate, nil
}

func retainedSource(entry *SourceCacheEntry, aliasPath []string) *RefreshedSource {
	if entry == nil || entry.ActiveContentDigest == "" {
		return nil
	}
	return &RefreshedSource{
		OwnerScopeID: entry.OwnerScopeID, AliasPath: append([]string(nil), aliasPath...), ScopeID: entry.ActualScopeID,
		ContentDigest: entry.ActiveContentDigest, ResolvedRevision: entry.ResolvedRevision,
	}
}

func pathPrefix(prefix, value []string) bool {
	if len(prefix) > len(value) {
		return false
	}
	for index := range prefix {
		if prefix[index] != value[index] {
			return false
		}
	}
	return true
}

func equalPath(left, right []string) bool {
	return len(left) == len(right) && pathPrefix(left, right)
}
