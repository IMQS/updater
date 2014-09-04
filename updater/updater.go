package updater

import (
	"errors"
	//"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
)

/* This orchestrates all update operations
After creating an Updater, you must load its configuration, an then call Initialize().
If Initialize() succeeds, then call Run().

Outline of Updater's operation

Updater runs in a continual loop, until it receives a shutdown signal. A shutdown signal
is either a Ctrl+C, when running on the command line, or a Windows Service STOP event.

The main loop looks like this:

	1. Check if there is an update available
		1.1 If update available...
			1.1.1 Download update
			1.1.2 Check that hash is correct
				1.1.2.1 If hash correct, start update
				1.1.2.2 If hash not correct, goto 2
		1.2 If update not available, goto 2
	2. Sleep for 3 minutes

The 'hash' is a 20 byte SHA1 hash of the entire contents of the directory. It is synchronized
along with the contents itself. Only if the hash matches, do we proceed with an update.
The hash is stored in a file callled 'manifest.hash', which is a 40 byte hex-encoded text file containing
the SHA1 hash.

The step "check if update available" is very cheap, since it only downloads a 40 byte text file.
If the hash differs from the current state, then we proceed with a full sync.

*/
type Updater struct {
	Config     *Config
	log        *log.Logger
	httpClient *http.Client
}

// Create a new updater
func NewUpdater() *Updater {
	u := new(Updater)
	u.Config = new(Config)
	u.httpClient = &http.Client{}
	return u
}

// Return an error if we fail to find an rsync executable, or open a log file, etc
func (u *Updater) Initialize() error {
	logPath := u.Config.LogFile + "-a.log"
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return errors.New("Unable to open log file '" + logPath + "'")
	}
	u.log = log.New(logFile, "Update", log.LstdFlags)
	u.log.Print("Updater started")
	return nil
}

// Run the updater.
func (u *Updater) Run() {
	u.fetch(&u.Config.BinDir)
	u.applyIfReady(&u.Config.BinDir)
}

func (u *Updater) fetch(syncDir *SyncDir) {
	u.downloadHash(syncDir)
	if syncDir.manifestHashIsReadableAndNew() {
		u.log.Printf("New content available on %v. Fetching content.", syncDir.LocalPath)
		u.downloadContent(syncDir)
	}
}

func (u *Updater) applyIfReady(syncDir *SyncDir) {
	isReady, err := syncDir.isReadyToApply()
	if !isReady {
		if err != nil {
			u.log.Printf("cannot apply %v: %v", syncDir.LocalPath, err)
		}
		return
	}

	// TODO: run before update

	u.log.Printf("Mirroring %v to %v", syncDir.LocalPathNext, syncDir.LocalPath)
	msg, err := u.mirrorNextToCurrent(syncDir)
	if err != nil {
		u.log.Printf("error mirroring %v to %v: %v", syncDir.LocalPathNext, syncDir.LocalPath, err)
		u.log.Print(msg)
		return
	}
	u.log.Printf("Mirror successful")

	// TODO: run after update
}

func (u *Updater) mirrorNextToCurrent(syncDir *SyncDir) (string, error) {
	return shellMirrorDirectory(syncDir.LocalPathNext, syncDir.LocalPath)
}

func (u *Updater) downloadHash(syncDir *SyncDir) {
	url := u.Config.DeployUrl + "/" + syncDir.Remote.Path + "/" + ManifestFilename_Hash
	err := u.download_file_http(url, path.Join(syncDir.LocalPathNext, ManifestFilename_Hash))
	if err != nil {
		u.log.Printf("Failed to fetch '%v': %v", url, err)
	}
}

func (u *Updater) downloadContent(syncDir *SyncDir) {
	if err := u.downloadContentHttp(syncDir); err != nil {
		u.log.Printf("Error synchronizing via http: %v", err)
	}
}

func (u *Updater) downloadContentHttp(syncDir *SyncDir) error {
	baseUrl := u.Config.DeployUrl + "/" + syncDir.Remote.Path
	// Download the manifest
	err := u.download_file_http(baseUrl+"/"+ManifestFilename_Content, path.Join(syncDir.LocalPathNext, ManifestFilename_Content))
	if err != nil {
		return err
	}
	// Ensure manifest and hash are consistent
	if err = isManifestPairConsistent(syncDir.LocalPathNext); err != nil {
		return err
	}
	// Do not attempt to use an old manifest file. Always build the manifest of our old contents from the content itself.
	manifest_prev, err := BuildManifest(syncDir.LocalPath)
	if err != nil {
		return err
	}
	// Read the 'next' manifest from file
	manifest_next, err := ReadManifest(syncDir.LocalPathNext)
	if err != nil {
		return err
	}
	n_existing := 0
	n_ready := 0
	n_new := 0
	n_removed := 0
	// Retrieve (via copy or download) files in 'next' manifest
	hashToFile := manifest_prev.hashToFileMap()
	for _, file := range manifest_next.Files {
		if file.hashEqualsDiskFile(syncDir.LocalPathNext) {
			n_ready++
			continue
		}
		outFile := path.Join(syncDir.LocalPathNext, file.Name)
		if err = os.MkdirAll(path.Dir(outFile), 0666); err != nil {
			return err
		}
		prev := hashToFile[file.Hash]
		if prev != nil {
			copyFile(path.Join(syncDir.LocalPath, prev.Name), outFile)
			n_existing++
		} else {
			if err = u.download_file_http(baseUrl+"/"+file.Name, outFile); err != nil {
				return err
			}
			n_new++
		}
	}
	// Delete files not present in 'next' manifest
	// This would only happen if an update was interrupted while downloading, and then when it
	// started up again, the server had already moved on. While rare, this is definitely a real-world scenario.
	// Here we don't care about hashes - we simply want to know which files to delete.
	manifest_next_ondisk, err := BuildManifestWithoutHashes(syncDir.LocalPathNext)
	if err != nil {
		return err
	}
	nameToFile := manifest_next.nameToFileMap()
	for _, file := range manifest_next_ondisk.Files {
		if nameToFile[file.Name] == nil {
			if err := os.Remove(path.Join(syncDir.LocalPathNext, file.Name)); err != nil {
				return err
			}
			n_removed++
		}
	}

	u.log.Printf("Finished synchronize. %v files new. %v files existing. %v files ready. %v files removed", n_new, n_existing, n_ready, n_removed)

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

	return ioutil.WriteFile(filename, body, 0666)
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
