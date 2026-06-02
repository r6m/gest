package cli

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

type pollWatcher struct {
	root     string
	watch    []string
	ignore   []string
	debounce time.Duration
	seen     map[string]time.Time
}

func newPollWatcher(root string, watch []string, ignore []string, debounce time.Duration) *pollWatcher {
	return &pollWatcher{
		root:     root,
		watch:    append([]string(nil), watch...),
		ignore:   append([]string(nil), ignore...),
		debounce: debounce,
		seen:     make(map[string]time.Time),
	}
}

func (w *pollWatcher) Run(ctx context.Context, onChange func([]string)) error {
	w.snapshot()
	ticker := time.NewTicker(w.debounce)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			changed := w.Changed()
			if len(changed) > 0 {
				onChange(changed)
			}
		}
	}
}

func (w *pollWatcher) Changed() []string {
	current := w.scan()
	changed := []string{}
	for path, modTime := range current {
		if previous, ok := w.seen[path]; !ok || !previous.Equal(modTime) {
			changed = append(changed, path)
		}
	}
	for path := range w.seen {
		if _, ok := current[path]; !ok {
			changed = append(changed, path)
		}
	}
	w.seen = current
	return changed
}

func (w *pollWatcher) snapshot() {
	w.seen = w.scan()
}

func (w *pollWatcher) scan() map[string]time.Time {
	files := make(map[string]time.Time)
	for _, watchPath := range w.watch {
		fullPath := watchPath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(w.root, fullPath)
		}
		info, err := filepath.EvalSymlinks(fullPath)
		if err == nil {
			fullPath = info
		}
		_ = filepath.WalkDir(fullPath, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, relErr := filepath.Rel(w.root, path)
			if relErr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if w.ignored(rel, entry.IsDir()) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			info, statErr := entry.Info()
			if statErr != nil {
				return nil
			}
			files[rel] = info.ModTime()
			return nil
		})
	}
	return files
}

func (w *pollWatcher) ignored(path string, isDir bool) bool {
	base := filepath.Base(path)
	for _, pattern := range w.ignore {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if path == pattern || strings.HasPrefix(path, pattern+"/") {
			return true
		}
		if isDir && base == pattern {
			return true
		}
	}
	return false
}
