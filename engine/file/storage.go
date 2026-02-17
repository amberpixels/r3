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

// --------------------------------------------------------------------------
// Single-file storage: one file per collection (e.g. cities.json)
// --------------------------------------------------------------------------

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
			// File doesn't exist yet — treat as empty collection.
			return nil
		}
		return fmt.Errorf("open file %s: %w", s.filePath, err)
	}
	defer f.Close()

	// Check if file is empty
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

	// Atomic write: write to temp file, then rename
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

// --------------------------------------------------------------------------
// Directory storage: one file per entity (e.g. cities/1.json, cities/2.json)
// --------------------------------------------------------------------------

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

	// Sort entries for deterministic ordering
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

func (s *directoryStorage) save(codec Codec, source any) error {
	if err := os.MkdirAll(s.dirPath, 0o750); err != nil {
		return fmt.Errorf("create directory %s: %w", s.dirPath, err)
	}

	// Remove existing files first (to handle deletions)
	ext := codec.FileExtension()
	entries, err := os.ReadDir(s.dirPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read directory %s: %w", s.dirPath, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ext) {
			_ = os.Remove(filepath.Join(s.dirPath, entry.Name()))
		}
	}

	// Write each entity as a separate file
	sourceVal := reflect.ValueOf(source)
	for i := range sourceVal.Len() {
		entity := sourceVal.Index(i).Interface()
		pkVal := s.meta.PKValue(entity)
		filename := fmt.Sprintf("%v%s", pkVal, ext)
		filePath := filepath.Join(s.dirPath, filename)

		f, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("create file %s: %w", filePath, err)
		}

		enc := codec.NewEncoder(f)
		if err := enc.Encode(entity); err != nil {
			_ = f.Close()
			return fmt.Errorf("encode to file %s: %w", filePath, err)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("close file %s: %w", filePath, err)
		}
	}

	return nil
}
