package updater

// This deals with the manifest files. It creates them, analyzes them, etc.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"path"
)

const ManifestFilename_Content = "manifest.content"
const ManifestFilename_Hash = "manifest.hash"

var ErrManifestInconsistent = errors.New("Manifest content and hash are inconsistent")

type ManifestFile struct {
	Name string // Filename, relative to root
	Hash string // hex-encoded SHA256 hash of file contents
}

// Returns true if the file exists, and its hash is the same as Hash
func (f *ManifestFile) hashEqualsDiskFile(rootDir string) bool {
	body, err := ioutil.ReadFile(path.Join(rootDir, f.Name))
	if err != nil {
		return false
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]) == f.Hash
}

// This stores enough information for a client to know when the contents of a file tree
// has been modified. It is necessary to store directories as well as files, so that
// we avoid corner cases such as the deletion of a directory, and subsequent replacement
// by a file of the same name.
type Manifest struct {
	Files []ManifestFile
	Dirs  []string
}

func BuildManifest(rootDir string) (*Manifest, error) {
	m := new(Manifest)
	if err := m.scanPathRecursive(rootDir, ""); err != nil {
		return nil, err
	}
	if err := m.calculateHashes(rootDir); err != nil {
		return nil, err
	}
	return m, nil
}

func BuildManifestWithoutHashes(rootDir string) (*Manifest, error) {
	m := new(Manifest)
	if err := m.scanPathRecursive(rootDir, ""); err != nil {
		return nil, err
	}
	return m, nil
}

func ReadManifest(rootDir string) (*Manifest, error) {
	body, err := ioutil.ReadFile(path.Join(rootDir, ManifestFilename_Content))
	if err != nil {
		return nil, err
	}
	m := &Manifest{}
	err = json.Unmarshal(body, m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Returns nil if the hash file and the manifest file in the given directory are consistent with each other
func isManifestPairConsistent(rootDir string) error {
	m, err := ReadManifest(rootDir)
	if err != nil {
		return err
	}
	return m.isConsistentWithHash(rootDir)
}

// Returns nil if this manifest is consistent with the hash file found in 'rootDir'
func (m *Manifest) isConsistentWithHash(rootDir string) error {
	hashHex, err := ioutil.ReadFile(path.Join(rootDir, ManifestFilename_Hash))
	if err != nil {
		return err
	}
	hash, err := hex.DecodeString(string(hashHex))
	if err != nil {
		return err
	}
	if !bytes.Equal(m.hash(), hash) {
		return ErrManifestInconsistent
	}
	return nil
}

func (m *Manifest) Write(rootDir string) error {
	if str, err := json.MarshalIndent(m, "", "\t"); err != nil {
		return err
	} else {
		if err := ioutil.WriteFile(path.Join(rootDir, ManifestFilename_Content), str, 0666); err != nil {
			return err
		}
		if err := ioutil.WriteFile(path.Join(rootDir, ManifestFilename_Hash), []byte(hex.EncodeToString(m.hash())), 0666); err != nil {
			return err
		}
		return nil
	}
}

// Return a map from hex-encoded hash to ManifestFile
// If there are duplicate entries, then the last one wins
func (m *Manifest) hashToFileMap() map[string]*ManifestFile {
	res := map[string]*ManifestFile{}
	for i := range m.Files {
		res[m.Files[i].Hash] = &m.Files[i]
	}
	return res
}

// Return a map from name to ManifestFile
func (m *Manifest) nameToFileMap() map[string]*ManifestFile {
	res := map[string]*ManifestFile{}
	for i := range m.Files {
		res[m.Files[i].Name] = &m.Files[i]
	}
	return res
}

// Return a map from directory name to bool (values are always 'true')
func (m *Manifest) nameToDirMap() map[string]bool {
	res := map[string]bool{}
	for i := range m.Dirs {
		res[m.Dirs[i]] = true
	}
	return res
}

// Why not just compute the hash over the JSON encoding?
// Because at some point, the server might want to start sending additional
// data inside that JSON envelope, and we wouldn't want a situation where
// the client thinks he has the wrong data because he still uses the old JSON
// representation.
func (m *Manifest) hash() []byte {
	h := sha256.New()
	for _, file := range m.Files {
		io.WriteString(h, file.Name)
		h.Write([]byte(file.Hash))
	}
	for _, dir := range m.Dirs {
		io.WriteString(h, dir)
	}
	return h.Sum(nil)
}

// Adds the files to the manifest, but does not compute their hashes.
// Use calculateHashes to populate the hashes
func (m *Manifest) scanPathRecursive(rootDir, relDir string) error {
	if items, err := ioutil.ReadDir(path.Join(rootDir, relDir)); err != nil {
		return err
	} else {
		for _, item := range items {
			relName := path.Join(relDir, item.Name())
			if relName == ManifestFilename_Content || relName == ManifestFilename_Hash {
				continue
			}

			if item.IsDir() {
				m.Dirs = append(m.Dirs, relName)
				if err := m.scanPathRecursive(rootDir, relName); err != nil {
					return err
				}
			} else {
				file := ManifestFile{
					Name: relName,
				}
				m.Files = append(m.Files, file)
			}
		}
	}
	return nil
}

func (m *Manifest) calculateHashes(rootDir string) error {
	for i := range m.Files {
		if bytes, err := ioutil.ReadFile(path.Join(rootDir, m.Files[i].Name)); err != nil {
			return err
		} else {
			hash := sha256.Sum256(bytes)
			m.Files[i].Hash = hex.EncodeToString(hash[:])
		}
	}
	return nil
}
