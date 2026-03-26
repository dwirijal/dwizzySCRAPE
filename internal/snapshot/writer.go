package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Context = context.Context

type Writer struct {
	root        string
	generatedAt time.Time
}

func NewWriter(root string, generatedAt time.Time) *Writer {
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	return &Writer{
		root:        strings.TrimSpace(root),
		generatedAt: generatedAt.UTC(),
	}
}

func (w *Writer) Root() string {
	return w.root
}

func (w *Writer) Write(domain, kind, name string, payload any) (string, error) {
	domain = sanitizePathPart(domain)
	kind = sanitizePathPart(kind)
	name = sanitizePathPart(name)
	if domain == "" || kind == "" || name == "" {
		return "", fmt.Errorf("domain, kind, and name are required")
	}
	if strings.TrimSpace(w.root) == "" {
		return "", fmt.Errorf("snapshot output root is required")
	}

	doc := Document{
		Domain:      domain,
		Kind:        kind,
		Name:        name,
		GeneratedAt: w.generatedAt,
		Payload:     payload,
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal snapshot document: %w", err)
	}
	body = append(body, '\n')

	target := filepath.Join(w.root, domain, kind, name+".json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("mkdir snapshot target: %w", err)
	}
	if err := os.WriteFile(target, body, 0o644); err != nil {
		return "", fmt.Errorf("write snapshot document: %w", err)
	}
	return target, nil
}

func WriteManifest(root string, generatedAt time.Time) (Manifest, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Manifest{}, fmt.Errorf("snapshot output root is required")
	}
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	entries := make([]ManifestEntry, 0)
	kindsByDomain := make(map[string]map[string]struct{})
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "manifest.json" || filepath.Ext(path) != ".json" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative snapshot path: %w", err)
		}
		parts := splitRelPath(rel)
		if len(parts) != 3 {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read snapshot file %s: %w", rel, err)
		}
		sum := sha256.Sum256(body)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat snapshot file %s: %w", rel, err)
		}

		entry := ManifestEntry{
			Domain:    parts[0],
			Kind:      parts[1],
			Name:      strings.TrimSuffix(parts[2], filepath.Ext(parts[2])),
			Path:      filepath.ToSlash(rel),
			SizeBytes: info.Size(),
			SHA256:    hex.EncodeToString(sum[:]),
		}
		entries = append(entries, entry)
		if _, ok := kindsByDomain[entry.Domain]; !ok {
			kindsByDomain[entry.Domain] = make(map[string]struct{})
		}
		kindsByDomain[entry.Domain][entry.Kind] = struct{}{}
		return nil
	}); err != nil {
		return Manifest{}, fmt.Errorf("walk snapshot output: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	domains := make([]DomainInventory, 0, len(kindsByDomain))
	for domain, kinds := range kindsByDomain {
		values := make([]string, 0, len(kinds))
		for kind := range kinds {
			values = append(values, kind)
		}
		sort.Strings(values)
		domains = append(domains, DomainInventory{
			Domain: domain,
			Kinds:  values,
		})
	}
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Domain < domains[j].Domain
	})

	manifest := Manifest{
		Version:     1,
		GeneratedAt: generatedAt.UTC(),
		Entries:     entries,
		Domains:     domains,
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("marshal manifest: %w", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), body, 0o644); err != nil {
		return Manifest{}, fmt.Errorf("write manifest: %w", err)
	}
	return manifest, nil
}

func sanitizePathPart(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		" ", "-",
		":", "-",
		"?", "",
		"&", "-",
		"=", "-",
		"%", "-",
		"#", "-",
	)
	input = replacer.Replace(input)
	input = strings.Trim(input, "-.")
	if input == "" {
		return ""
	}
	return input
}

func splitRelPath(rel string) []string {
	normalized := filepath.ToSlash(strings.TrimSpace(rel))
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "/")
}
