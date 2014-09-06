package updater

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
)

// Remote directory
type RemotePath struct {
	Username string
	Password string
	Path     string
}

// A directory that is synchronized
type SyncDir struct {
	Remote        RemotePath // Remote directory (eg imqsbin@deploy.imqs.co.za:imqsbin/stable)
	LocalPath     string     // Current directory (eg c:\imqsbin)
	LocalPathNext string     // Staging directory, where we synchronize to before atomically replacing LocalPath (eg c:\imqsbin_next)
	beforeSync    func(upd *Updater, syncDir *SyncDir) error
	afterSync     func(upd *Updater, syncDir *SyncDir)
}

func (s *SyncDir) manifestHashIsReadableAndNew() bool {
	f1, e1 := ioutil.ReadFile(path.Join(s.LocalPath, ManifestFilename_Hash))
	f2, e2 := ioutil.ReadFile(path.Join(s.LocalPathNext, ManifestFilename_Hash))
	if e2 != nil {
		return false
	}
	if e1 != nil {
		// Specially allow a missing source hash, so that we can sync with an empty base directory
		if _, err := os.Stat(path.Join(s.LocalPath, ManifestFilename_Hash)); os.IsNotExist(err) {
			return true
		}
		// However, any error other than "file not found", spells trouble
		return false
	}
	return !bytes.Equal(f1, f2)
}

/* Returns true, nil if the following conditions are met:

* manifest.hash is different in LocalPath and LocalPathNext
* Inside LocalPathNext, manifest.content is consistent with manifest.hash
* Inside LocalPathNext, manifest.content is consistent with files on disk

 */
func (s *SyncDir) isReadyToApply() (bool, error) {
	if !s.manifestHashIsReadableAndNew() {
		return false, nil
	}
	manifest_truth, err := BuildManifest(s.LocalPathNext)
	if err != nil {
		return false, err
	}
	manifest_file, err := ReadManifest(s.LocalPathNext)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(manifest_truth.hash(), manifest_file.hash()) {
		return false, nil
	}
	consistent := manifest_truth.isConsistentWithHash(s.LocalPathNext)
	if consistent != nil {
		return false, consistent
	}
	return true, nil
}
