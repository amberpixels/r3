package enginefile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// storage is an internal interface for reading/writing entity collections.
type storage interface {
	// load reads all entities from storage. target must be a pointer to a slice.
	load(codec Codec, target any) error
	// save writes all entities to storage atomically. source must be a slice.
	save(codec Codec, source any) error
}

// singleFileStorage holds an entire collection in one file (e.g. cities.json).
type singleFileStorage struct {
	filePath string
}

func newSingleFileStorage(baseDir, resourceName string, codec Codec) *singleFileStorage {
	return &singleFileStorage{
		filePath: filepath.Join(baseDir, resourceName+codec.FileExtension()),
	}
}

func (s *singleFileStorage) load(codec Codec, target any) error {
	f, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No file yet: an empty collection.
			return nil
		}
		return fmt.Errorf("open file %s: %w", s.filePath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file %s: %w", s.filePath, err)
	}
	if info.Size() == 0 {
		return nil
	}

	dec := codec.NewDecoder(f)
	if err := dec.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("decode file %s: %w", s.filePath, err)
	}
	return nil
}

func (s *singleFileStorage) save(codec Codec, source any) error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	// Atomic write: encode to a temp file, then rename over the target.
	tmpPath := s.filePath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file %s: %w", tmpPath, err)
	}

	enc := codec.NewEncoder(f)
	if err := enc.Encode(source); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("encode to file %s: %w", tmpPath, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, s.filePath, err)
	}

	return nil
}

// directoryStorage holds one file per entity (e.g. cities/1.json, cities/2.json).
type directoryStorage struct {
	dirPath string
	meta    *StructMeta
}

func newDirectoryStorage(baseDir, resourceName string, meta *StructMeta) *directoryStorage {
	return &directoryStorage{
		dirPath: filepath.Join(baseDir, resourceName),
		meta:    meta,
	}
}

func (s *directoryStorage) load(codec Codec, target any) error {
	// target must be *[]T
	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Pointer || targetVal.Elem().Kind() != reflect.Slice {
		return errors.New("target must be a pointer to a slice")
	}
	sliceVal := targetVal.Elem()
	elemType := sliceVal.Type().Elem()

	entries, err := os.ReadDir(s.dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read directory %s: %w", s.dirPath, err)
	}

	// Deterministic ordering across runs.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	ext := codec.FileExtension()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ext) {
			continue
		}

		filePath := filepath.Join(s.dirPath, entry.Name())
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open file %s: %w", filePath, err)
		}

		entityPtr := reflect.New(elemType)
		dec := codec.NewDecoder(f)
		if err := dec.Decode(entityPtr.Interface()); err != nil {
			_ = f.Close()
			return fmt.Errorf("decode file %s: %w", filePath, err)
		}
		_ = f.Close()

		sliceVal = reflect.Append(sliceVal, entityPtr.Elem())
	}

	targetVal.Elem().Set(sliceVal)
	return nil
}

// tmpFilePrefix marks per-entity temp files written during a save. It has no
// codec extension, so load() skips it.
const tmpFilePrefix = ".tmp-"

func (s *directoryStorage) save(codec Codec, source any) error {
	if err := os.MkdirAll(s.dirPath, 0o750); err != nil {
		return fmt.Errorf("create directory %s: %w", s.dirPath, err)
	}

	ext := codec.FileExtension()

	// Resolve every filename up front: an unsafe PK or a duplicate aborts the
	// save before any write, so a crafted id can never escape the storage dir and
	// a key collision can never silently overwrite another entity.
	sourceVal := reflect.ValueOf(source)
	type pendingWrite struct {
		filename string
		entity   any
	}
	items := make([]pendingWrite, 0, sourceVal.Len())
	desired := make(map[string]struct{}, sourceVal.Len())
	for i := range sourceVal.Len() {
		entity := sourceVal.Index(i).Interface()
		filename, err := safeEntityFilename(s.meta.PKValue(entity), ext)
		if err != nil {
			return err
		}
		if _, dup := desired[filename]; dup {
			return fmt.Errorf("duplicate entity id maps to file %q", filename)
		}
		desired[filename] = struct{}{}
		items = append(items, pendingWrite{filename: filename, entity: entity})
	}

	// Write each entity (temp file + rename) without removing anything first: a
	// failed write leaves the existing collection intact, so a partial save
	// never destroys data.
	for _, it := range items {
		if err := s.writeEntityFile(codec, it.filename, it.entity); err != nil {
			return err
		}
	}

	// All writes succeeded: only now remove files for entities that no longer
	// exist (deletions), plus leftover temp files from a crashed save.
	entries, err := os.ReadDir(s.dirPath)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", s.dirPath, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch {
		case strings.HasPrefix(name, tmpFilePrefix):
			_ = os.Remove(filepath.Join(s.dirPath, name))
		case strings.HasSuffix(name, ext):
			if _, keep := desired[name]; !keep {
				_ = os.Remove(filepath.Join(s.dirPath, name))
			}
		}
	}

	return nil
}

// writeEntityFile writes one entity atomically: encode into a temp file in the
// same directory, then rename it over the target path.
func (s *directoryStorage) writeEntityFile(codec Codec, filename string, entity any) error {
	tmp, err := os.CreateTemp(s.dirPath, tmpFilePrefix+"*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", s.dirPath, err)
	}
	tmpPath := tmp.Name()

	enc := codec.NewEncoder(tmp)
	if err := enc.Encode(entity); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("encode entity to %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}

	finalPath := filepath.Join(s.dirPath, filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, finalPath, err)
	}
	return nil
}

// safeEntityFilename builds a filesystem-safe filename from an entity's primary
// key. The key is used verbatim as one path segment, so it must not be empty,
// "." / "..", or contain a separator - otherwise a crafted id (e.g.
// "../../etc/passwd") could escape the storage dir. Unsafe keys are rejected,
// not rewritten, since rewriting could map two keys onto the same file.
func safeEntityFilename(pkVal any, ext string) (string, error) {
	key := fmt.Sprintf("%v", pkVal)
	if key == "" || key == "." || key == ".." || strings.ContainsAny(key, `/\`) {
		return "", fmt.Errorf(
			"entity id %q is not a valid storage filename: "+
				"directory mode requires ids that are a single path segment "+
				"with no separators or path traversal",
			key,
		)
	}
	return key + ext, nil
}
