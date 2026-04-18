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

	"marrow/internal/db"
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
	var files []FileInfo
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		// Fast-path skip: if mtime hasn't changed since last sync and we have a DB hash,
		// still hash-check it, but we can skip reading if mtime <= since.
		if info.ModTime().Before(since) || info.ModTime().Equal(since) {
			var storedHash string
			if err := c.db.QueryRowContext(ctx, `SELECT hash FROM documents WHERE path = ? AND source = ?`, path, source).Scan(&storedHash); err == nil {
				return nil // unchanged since last sync and already indexed
			}
		}

		fi, err := c.checkFile(ctx, path, source)
		if err != nil {
			return fmt.Errorf("check %s: %w", path, err)
		}
		if fi != nil {
			files = append(files, *fi)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Find deleted files: paths tracked in DB for this source but missing on disk
	tracked, err := db.NewDocumentRepo(c.db).GetDocumentPathsBySource(ctx, source)
	if err != nil {
		return nil, nil, fmt.Errorf("list tracked: %w", err)
	}
	var deleted []string
	for _, p := range tracked {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			deleted = append(deleted, p)
		}
	}
	return files, deleted, nil
}

func (c *Crawler) checkFile(ctx context.Context, path, source string) (*FileInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprintf("%x", HashBytes(data))

	var storedHash string
	err = c.db.QueryRowContext(ctx, `SELECT hash FROM documents WHERE path = ? AND source = ?`, path, source).Scan(&storedHash)
	if err == nil && storedHash == hash {
		return nil, nil // unchanged
	}
	return &FileInfo{Path: path, Hash: hash}, nil
}

// HashBytes returns the SHA-256 hex digest of data.
func HashBytes(data []byte) [32]byte {
	return sha256.Sum256(data)
}
