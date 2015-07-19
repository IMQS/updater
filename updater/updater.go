package updater

import (
	"errors"
	//"fmt"
	"github.com/IMQS/log"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"
)

const newDirPerms = 0774
const newFilePerms = 0664

/*
This orchestrates all update operations.

After creating an Updater, you must load its configuration, an then call Initialize().
If Initialize() succeeds, then call Run().

Outline of Updater's operation

Updater runs in a continual loop, never exiting.

The main loop looks like this:

	1. Check if there is an update available
		1.1 If update available...
			1.1.1 Download update
			1.1.2 Check that hash is correct
				1.1.2.1 If hash correct, start update
				1.1.2.2 If hash not correct, goto 2
		1.2 If update not available, goto 2
	2. Sleep for 3 minutes

The 'hash' is a 32 byte SHA256 hash of the entire contents of the directory. It is synchronized
along with the contents itself. Only if the hash matches, do we proceed with an update.
The hash is stored in a file callled 'manifest.hash', which is a 64 byte hex-encoded text file containing
the SHA256 hash.

The step "check if update available" is very cheap, since it only downloads a 64 byte text file.
If the hash differs from the current state, then we proceed with a full sync.

*/
type Updater struct {
	Config     *Config
	log        *log.Logger
	httpClient *http.Client
	beforeSync func(upd *Updater, updatedDirs []*SyncDir) error
	afterSync  func(upd *Updater, updatedDirs []*SyncDir)
}

// Create a new updater
func NewUpdater() *Updater {
	u := new(Updater)
	u.Config = NewConfig()
	u.httpClient = http.DefaultClient
	u.beforeSync = beforeSyncImqs
	u.afterSync = afterSyncImqs
	return u
}

// Return an error if we fail to open a log file, etc
func (u *Updater) Initialize() error {
	u.log = log.New(u.Config.LogFile)
	//u.log.Level = log.Debug
	u.log.Info("Updater started")
	return nil
}

// Returns true if we detected that we are not running in a non-interactive session, and so
// launched the service. This function will not return until the service exits.
func (u *Updater) RunAsService() bool {
	return runService(u.log, func() {
		u.Run()
	})
}

// Run the updater forever
func (u *Updater) Run() {
	for {
		u.Download()
		u.Apply()
		time.Sleep(time.Duration(u.Config.CheckIntervalSeconds) * time.Second)
	}
}

// Download new content, but do not deploy
func (u *Updater) Download() {
	for _, dir := range u.Config.allSyncDirs() {
		u.fetch(dir)
	}
}

func (u *Updater) fetch(syncDir *SyncDir) {
	// Allow syncing onto a clean system with nothing pre-installed
	if err := u.ensureDirExists(syncDir.LocalPath); err != nil {
		u.log.Errorf("Failed to create directory %v: %v", syncDir.LocalPath, err)
		return
	}
	if err := u.ensureDirExists(syncDir.LocalPathNext); err != nil {
		u.log.Errorf("Failed to create directory %v: %v", syncDir.LocalPathNext, err)
		return
	}

	// Actually do the downloading
	u.downloadHash(syncDir)
	if syncDir.manifestHashIsReadableAndNew() {
		u.log.Infof("New content available on %v. Fetching content.", syncDir.LocalPath)
		u.downloadContent(syncDir)
	}
}

// Run the updater once, if new content is ready to deploy
func (u *Updater) Apply() {
	ready := []*SyncDir{}
	for _, dir := range u.Config.allSyncDirs() {
		isReady, err := dir.isReadyToApply()
		if err != nil {
			u.log.Errorf("isReadyToApply failed on %v: %v", dir.LocalPath, err)
			return
		}
		if isReady {
			ready = append(ready, dir)
		}
	}
	if len(ready) == 0 {
		return
	}

	if u.beforeSync != nil {
		err := u.beforeSync(u, ready)
		if err != nil {
			u.log.Errorf("Cannot apply, beforeSync error: %v", err)
			return
		}
	}

	for _, dir := range ready {
		u.log.Infof("Mirroring %v to %v", dir.LocalPathNext, dir.LocalPath)
		msg, err := u.mirrorNextToCurrent(dir)
		if err != nil {
			u.log.Errorf("error mirroring %v to %v: %v", dir.LocalPathNext, dir.LocalPath, err)
			u.log.Errorf("stdout from shell mirror: %v", msg)
			return
		}
		u.log.Info("Mirror successful")
	}

	if u.afterSync != nil {
		u.afterSync(u, ready)
	}
}

func (u *Updater) mirrorNextToCurrent(syncDir *SyncDir) (string, error) {
	return shellMirrorDirectory(syncDir.LocalPathNext, syncDir.LocalPath)
}

func (u *Updater) downloadHash(syncDir *SyncDir) {
	url := u.Config.DeployUrl + "/" + syncDir.Remote.Path + "/" + ManifestFilename_Hash
	err := u.download_file_http(url, path.Join(syncDir.LocalPathNext, ManifestFilename_Hash))
	if err != nil {
		u.log.Warnf("Failed to fetch hash: %v", err)
	}
}

func (u *Updater) downloadContent(syncDir *SyncDir) {
	if err := u.downloadContentHttp(syncDir); err != nil {
		u.log.Warnf("Error synchronizing via http: %v", err)
	}
}

/*
An optimization note on re-using existing files:
During preparation of 'next', we look for hashes of existing files in 'current'. If we find a file
in 'current', then we copy it to 'next'. However, if that file already exists in 'next', and
it's Modification Time and Size are the same as 'current', then we assume that the two files
are identical. This saves a lot of read bandwidth when performing an incremental update.
Because we leave 'next' intact from one update to the next, both directories tend to have
very similar content. The bottom line is that updates touch only what they need to.

Throughout this function we use two words:
actual	The files and hashes on disk
ideal	The files and hashes specified in a JSON manifest file
*/
func (u *Updater) downloadContentHttp(syncDir *SyncDir) error {
	baseUrl := u.Config.DeployUrl + "/" + syncDir.Remote.Path
	// Download the manifest
	err := u.download_file_http(baseUrl+"/"+ManifestFilename_Content, path.Join(syncDir.LocalPathNext, ManifestFilename_Content))
	if err != nil {
		return err
	}
	// Ensure manifest and hash are consistent (ie the two files manifest.content and manifest.hash)
	if err = isManifestPairConsistent(syncDir.LocalPathNext); err != nil {
		return err
	}
	// Do not attempt to use an old manifest file. Always build the manifest of our old contents from the content itself.
	actual_manifest_prev, err := BuildManifest(syncDir.LocalPath)
	if err != nil {
		return err
	}
	// Read the 'next' manifest from file
	ideal_manifest_next, err := ReadManifest(syncDir.LocalPathNext)
	if err != nil {
		return err
	}
	n_existing := 0
	n_ready := 0
	n_new := 0
	n_removed := 0
	n_removed_dir := 0

	// Delete files not present in 'next' manifest
	actual_manifest_next, err := BuildManifest(syncDir.LocalPathNext)
	if err != nil {
		return err
	}
	nameToFile := ideal_manifest_next.nameToFileMap()
	for _, file := range actual_manifest_next.Files {
		if nameToFile[file.Name] == nil {
			fullName := path.Join(syncDir.LocalPathNext, file.Name)
			u.log.Debugf("Deleting %v", fullName)
			if err := os.Remove(fullName); err != nil {
				return err
			}
			n_removed++
		}
	}

	// Delete directories not present in 'next' manifest
	nameToDir := ideal_manifest_next.nameToDirMap()
	for _, dir := range actual_manifest_next.Dirs {
		if !nameToDir[dir] {
			fullName := path.Join(syncDir.LocalPathNext, dir)
			u.log.Debugf("Deleting directory %v", fullName)
			if err := os.RemoveAll(fullName); err != nil {
				return err
			}
			n_removed_dir++
		}
	}

	// Create directories in 'next' manifest
	for _, dir := range ideal_manifest_next.Dirs {
		fullName := path.Join(syncDir.LocalPathNext, dir)
		if _, err := os.Stat(fullName); err != nil {
			u.log.Debugf("Creating directory %v", fullName)
			if err := u.ensureDirExists(fullName); err != nil {
				return err
			}
		}
	}

	// Retrieve (via copy or download) files in 'next' manifest
	actual_hashToFilePrev := actual_manifest_prev.hashToFileMap()
	actual_hashToFileNext := actual_manifest_next.hashToFileMap()
	for _, file := range ideal_manifest_next.Files {
		outFile := path.Join(syncDir.LocalPathNext, file.Name)
		actual_prev := actual_hashToFilePrev[file.Hash]
		actual_next := actual_hashToFileNext[file.Hash]
		if actual_prev != nil {
			prevFullPath := path.Join(syncDir.LocalPath, actual_prev.Name)
			if areFileDatesAndSizesEqual(prevFullPath, outFile) {
				u.log.Debugf("%v satisfied by %v", outFile, prevFullPath)
				n_ready++
			} else {
				u.log.Debugf("Copying %v to %v", prevFullPath, outFile)
				if err := copyFile(prevFullPath, outFile); err != nil {
					return err
				}
				n_existing++
			}
		} else if actual_next != nil && actual_next.Name == file.Name {
			u.log.Debugf("%v already downloaded", file.Name)
			n_ready++
		} else {
			u.log.Debugf("Downloading %v", file.Name)
			if err = u.download_file_http(baseUrl+"/"+file.Name, outFile); err != nil {
				return err
			}
			n_new++
		}
	}

	u.log.Infof("Download complete. %v files new. %v files existing. %v files ready. %v files removed. %v dirs removed", n_new, n_existing, n_ready, n_removed, n_removed_dir)

	return nil
}

func (u *Updater) download_file_http(url, filename string) error {
	res, err := u.httpClient.Get(url)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("Error reading " + url + ": " + res.Status)
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, body, newFilePerms)
}

func (u *Updater) ensureDirExists(dir string) error {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, newDirPerms|os.ModeDir)
	}
	return err
}

func areFileDatesAndSizesEqual(src, dst string) bool {
	isrc, err := os.Stat(src)
	if err != nil {
		return false
	}

	idst, err := os.Stat(dst)
	if err != nil {
		return false
	}

	return isrc.ModTime() == idst.ModTime() && isrc.Size() == idst.Size()
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
