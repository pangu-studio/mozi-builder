// Package manifest manages the .mozi/manifest.json file that tracks
// the last generated database version for each model.
package manifest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ManifestFile is the name of the manifest file relative to models/ dir.
const ManifestFile = ".mozi/manifest.json"

// Manifest tracks code generation state for each model.
type Manifest struct {
	Version         int                     `json:"version"`
	Models          map[string]ModelGenInfo `json:"models"`
	LastEntGenerate string                  `json:"last_ent_generate,omitempty"`
	mu              sync.RWMutex            `json:"-"`
	path            string                  `json:"-"`
}

// ModelGenInfo records the last generation for a single model.
type ModelGenInfo struct {
	LastGenVersion   string                       `json:"last_gen_version"`
	LastGenAt        string                       `json:"last_gen_at"`
	GeneratedFiles   []string                     `json:"generated_files"`
	GeneratorVersion string                       `json:"generator_version,omitempty"`
	TemplateVersion  string                       `json:"template_version,omitempty"`
	Files            map[string]GeneratedFileInfo `json:"files,omitempty"`
}

type GeneratedFileInfo struct {
	Hash      string `json:"hash"`
	Ownership string `json:"ownership"` // generated | marker | advisory
}

// Load reads the manifest from the project's models directory.
func Load(projectRoot string) (*Manifest, error) {
	m := &Manifest{
		Version: 2,
		Models:  make(map[string]ModelGenInfo),
		path:    filepath.Join(projectRoot, "models", ManifestFile),
	}

	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return m, nil // fresh manifest
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	if m.Models == nil {
		m.Models = make(map[string]ModelGenInfo)
	}
	if m.Version < 2 {
		m.Version = 2
	}
	return m, nil
}

// RecordGenWithMetadata records provenance and ownership for generated output.
func (m *Manifest) RecordGenWithMetadata(modelRef, version, generatorVersion, templateVersion string, files map[string][]byte, ownership string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ownership == "" {
		ownership = "generated"
	}
	info := ModelGenInfo{LastGenVersion: version, LastGenAt: time.Now().Format(time.RFC3339), GeneratorVersion: generatorVersion, TemplateVersion: templateVersion, Files: map[string]GeneratedFileInfo{}}
	for path, content := range files {
		hash := sha256.Sum256(content)
		info.GeneratedFiles = append(info.GeneratedFiles, path)
		info.Files[path] = GeneratedFileInfo{Hash: fmt.Sprintf("%x", hash[:]), Ownership: ownership}
	}
	sort.Strings(info.GeneratedFiles)
	m.Models[modelRef] = info
}

// Save writes the manifest back to disk.
func (m *Manifest) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0644)
}

// RecordGen records that a model was generated at a specific version.
func (m *Manifest) RecordGen(modelRef string, version string, files []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Models[modelRef] = ModelGenInfo{
		LastGenVersion: version,
		LastGenAt:      time.Now().Format(time.RFC3339),
		GeneratedFiles: files,
	}
}

// GetGenInfo returns the generation info for a model, or zero values if not found.
func (m *Manifest) GetGenInfo(modelRef string) ModelGenInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Models[modelRef]
}

// NeedsRegen returns true if the model's current version is newer than last generated.
func (m *Manifest) NeedsRegen(modelRef string, currentVersion string) bool {
	info := m.GetGenInfo(modelRef)
	return currentVersion != info.LastGenVersion
}

// SetLastEntGenerate records when ent generate was last run.
func (m *Manifest) SetLastEntGenerate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastEntGenerate = time.Now().Format(time.RFC3339)
}
