/*
Package updater is the IMQS system updater.

This is responsible for keeping all IMQS deployments up to date.

Outline

This system uses a vanilla HTTP server such as Apache, Nginx, or S3 to host update files. The updater
on the client systems uses plain old HTTP/HTTPS to check for new updates, and to download
updates when available. Part of this toolchain is a preparation tool that builds a manifest file
with a list of SHA256 hashes of all the files in the directory tree. This manifest is downloaded
by the updater client, and is used to quickly determine which files have changed since the last update.

If a client determines that an update is available, then it downloads all the files that it does
not already have. Once all files are downloaded, and their hash signatures verified, the system
mirrors the new directory onto the old directory.

We can split this system up neatly into three parts: Uploader, Server, Downloader

The Uploader

A new release is built into the build tree. For example, a new release, build 1034,
is output to c:\builds\imqsbin\versions\1034. Next, that release is symlinked
to a branch, such as c:\builds\imqsbin\branches\alpha. From here on it is BitTorrent
Sync's job to upload 'alpha' onto the HTTP server.

The Server

The server is literally anything that can serve up HTTP/HTTPS. For atomicity reasons,
however, we use a linux server instead of S3. BitTorrent Sync uploads new content onto
a linux machine in EC2 called deploy.imqs.co.za. This new content is uploaded
into a staging area on the server. A cronjob wakes up once a minute and checks whether
the data in the staging area has consistent hashes. This means that computing
manifest.hash for that directory yields the same value that is presently inside
the actual manifest.hash file. If the hashes are consistent, and the hash differs
from that presently being served up by the HTTP server, then the cronjob switches
out the old directory for the new, and the server starts to serve up a new release.

An example URL for an imqsbin directory is https://deploy.imqs.co.za/files/imqsbin/stable

The Downloader

The downloader runs as a Windows Service. It wakes up every 3 minutes, and checks for
new content. Checking for new content involves downloading the latest manifest.hash file.
This file is a hex-encoded SHA256 hash of the release, so it is a 64-byte download.
If this file differs from what is currently on the server, then the updater proceeds
to download all content that it does not already have. New content is downloaded
into a staging directory. This staging directory is a complete image of the new release.
Once the downloader is finished, it checks to see whether the staging areas has
consistent hashes. This is the same check that the server's cronjob runs before publishing
a new release. If the hashes are consistent, then the downloader stops all services,
and mirrors the staging directory onto the real directory. It then runs install.rb,
and restarts all services.

Infra-file diffs

It might be worthwhile integrating binary diffs into this system, so that one doesn't need
to download an entire copy of a changed file. One could simply prepare a list of files
that would end up at URLs such as the following:

https://deploy.imqs.co.za/updates/diff/c629cee85a65c4b818221038835d74791151727c-4b37e3919462a3153d7527013e020c08f42df700

This is an example URL that is the bspatch file that patches the file on the left side,
to the file on the right side. The left and right side are hashes of the respective files.
It would probably be best to integrate the diff computation into the Upload phase.
Before uploading the new content, one would compute diffs against the current files, and add
those diffs to be synced up. It's as simple as running bsdiff on all files that have changed.

Difficult Issues

Adding more information to the manifest is tricky. If one does so naively, then existing
updaters will refuse to install the update, because they believe that the hash and the manifest
are inconsistent.

There is a way to get around this:

Publish updated binaries, which understand the new information in the .manifest file. However,
for that initial update, publish the .hash file using the old technique. Then, once all servers
are up to date, switch to publishing .hash files which include the new information. This would
be hairy, since it would mean that not a single server could skip an update. An alternate approach
would be to version the hash files, such as manifest.hash.1. One could then publish two or more
different hash files, and the client could use whichever one it understands. Let's add these
version numbers once we need to.
*/
package updater
