package updater

import (
	//"log"
)

// This simply cannot live here.. it needs to be in a SINGLE place
var imqsServices = []string{
	"ImqsAuth",
	"ImqsCpp",
	"ImqsDocs",
	"ImqsMongo",
}

func beforeSyncBin(upd *Updater, syncDir *SyncDir) {
}

func afterSyncBin(upd *Updater, syncDir *SyncDir) {
}
