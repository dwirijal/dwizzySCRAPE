package snapshot

import "time"

const (
	KindHome     = "home"
	KindCatalog  = "catalog"
	KindSearch   = "search"
	KindTitle    = "title"
	KindPlayback = "playback"
)

type BuildOptions struct {
	OutputDir          string
	HotLimit           int
	CatalogPage        int
	MovieGenres        []string
	MovieSearchQueries []string
	GeneratedAt        time.Time
}

type Document struct {
	Domain      string    `json:"domain"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	GeneratedAt time.Time `json:"generated_at"`
	Payload     any       `json:"payload"`
}

type Manifest struct {
	Version     int               `json:"version"`
	GeneratedAt time.Time         `json:"generated_at"`
	Entries     []ManifestEntry   `json:"entries"`
	Domains     []DomainInventory `json:"domains"`
}

type ManifestEntry struct {
	Domain    string `json:"domain"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type DomainInventory struct {
	Domain string   `json:"domain"`
	Kinds  []string `json:"kinds"`
}

type Collector interface {
	Domain() string
	Build(ctx Context, writer *Writer, options BuildOptions) error
	Patch(ctx Context, writer *Writer, slug string, options BuildOptions) error
}
