package watcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enekos/marrow/internal/db"
)

// Crawler walks a directory tree and identifies changed/new Markdown files.
type Crawler struct {
	db *db.DB
}

// NewCrawler creates a crawler backed by the database.
func NewCrawler(database *db.DB) *Crawler {
	return &Crawler{db: database}
}

// FileInfo holds the path and hash of a changed markdown file.
type FileInfo struct {
	Path string
	Hash string
}

// ScanIncremental walks root recursively and yields changed .md files since 'since'.
// It also returns a list of tracked paths in DB for this source that no longer exist on disk.
func (c *Crawler) ScanIncremental(ctx context.Context, root string, since time.Time, source string) ([]FileInfo, []string, error) {
	// Prefetch every (path, hash) tracked for this source in a single query.
	// On large repos this avoids 10k+ per-file SELECTs inside WalkDir.
	known, err := loadKnownHashes(ctx, c.db, source)
	if err != nil {
		return nil, nil, fmt.Errorf("load known hashes: %w", err)
	}

	var files []FileInfo
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		storedHash, tracked := known[path]
		// Fast-path skip: if mtime hasn't changed since last sync and we already
		// have a DB hash, trust it — avoids reading + hashing the file.
		if tracked && (info.ModTime().Before(since) || info.ModTime().Equal(since)) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		hash := fmt.Sprintf("%x", HashBytes(data))
		if tracked && storedHash == hash {
			return nil
		}
		files = append(files, FileInfo{Path: path, Hash: hash})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Find deleted files: paths tracked in DB for this source but missing on disk.
	var deleted []string
	for p := range known {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			deleted = append(deleted, p)
		}
	}
	return files, deleted, nil
}

// checkFile hashes path and returns a FileInfo only if the hash differs from
// the stored hash for (path, source). Kept as a convenience for tests.
func (c *Crawler) checkFile(ctx context.Context, path, source string) (*FileInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprintf("%x", HashBytes(data))

	var storedHash string
	err = c.db.QueryRowContext(ctx, `SELECT hash FROM documents WHERE path = ? AND source = ?`, path, source).Scan(&storedHash)
	if err == nil && storedHash == hash {
		return nil, nil
	}
	return &FileInfo{Path: path, Hash: hash}, nil
}

func loadKnownHashes(ctx context.Context, database *db.DB, source string) (map[string]string, error) {
	rows, err := database.QueryContext(ctx, `SELECT path, hash FROM documents WHERE source = ?`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string, 1024)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, err
		}
		out[path] = hash
	}
	return out, rows.Err()
}

// HashBytes returns the SHA-256 hex digest of data.
func HashBytes(data []byte) [32]byte {
	return sha256.Sum256(data)
}
