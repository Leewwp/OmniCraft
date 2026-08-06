package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// File is a single forward-only migration found in the migrations directory.
type File struct {
	Version  int
	Filename string
	Path     string
	Checksum string
}

var filenamePattern = regexp.MustCompile(`^(\d{3})_([a-z0-9_]+)\.sql$`)

// ParseFilename validates a forward-only migration filename of the form
// NNN_name.sql and returns its numeric version.
func ParseFilename(name string) (int, error) {
	match := filenamePattern.FindStringSubmatch(name)
	if match == nil {
		return 0, fmt.Errorf("invalid migration filename %q: want NNN_lowercase_name.sql", name)
	}
	version, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("invalid migration version in %q: %w", name, err)
	}
	return version, nil
}

// ScanFiles lists every migration file in dir, computes its SHA-256 checksum,
// rejects duplicate versions, and returns the files sorted by version then
// filename.
func ScanFiles(dir string) ([]File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory %s: %w", dir, err)
	}

	seen := make(map[int]string)
	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "metadata.json" || entry.Name() == "metadata.schema.json" {
			continue
		}
		version, err := ParseFilename(entry.Name())
		if err != nil {
			return nil, err
		}
		if previous, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %03d: %s and %s", version, previous, entry.Name())
		}
		seen[version] = entry.Name()

		path := filepath.Join(dir, entry.Name())
		checksum, err := ChecksumFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, File{
			Version:  version,
			Filename: entry.Name(),
			Path:     path,
			Checksum: checksum,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].Version != files[j].Version {
			return files[i].Version < files[j].Version
		}
		return files[i].Filename < files[j].Filename
	})
	return files, nil
}

// ChecksumFile returns the lowercase hex SHA-256 of a migration file.
func ChecksumFile(path string) (string, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("open migration %s: %w", path, err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("checksum migration %s: %w", path, err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
