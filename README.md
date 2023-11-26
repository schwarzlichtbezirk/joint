# Joint

Provides a single interface to get access to files in ISO-9660 images, FTP-servers, SFTP-servers, WebDAV-servers. Contains cache with reusable connections to endpoints.

## Examples

### HTTP file server with ISO-image content

```go
package main

import (
    "log"
    "net/http"

    jnt "github.com/schwarzlichtbezirk/hms/joint"
)

const isopath = "testdata/external.iso"

// Open http://localhost:8080/ in browser
// to get a list of files in ISO-image.
func main() {
    var jc = jnt.NewJointCache(isopath, jnt.NewIsoJoint)
    http.Handle("/", http.FileServer(http.FS(jc)))
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### HTTP file server with WebDAV content

```go
package main

import (
    "log"
    "net/http"

    jnt "github.com/schwarzlichtbezirk/hms/joint"
)

const davpath = "https://music:x@192.168.1.1/webdav/"

// Open http://localhost:8080/ in browser
// to get a list of files in WebDAV-server for given user.
func main() {
    var jc = jnt.NewJointCache(davpath, jnt.NewDavJoint)
    http.Handle("/", http.FileServer(http.FS(jc)))
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### HTTP file server with ISO, WebDAV, FTP and SFTP content

```go
package main

import (
    "log"
    "net/http"

    jnt "github.com/schwarzlichtbezirk/hms/joint"
)

const (
    isopath = "testdata/external.iso"
    davpath = "https://music:x@192.168.1.1/webdav/"
    ftppath = "ftp://music:x@192.168.1.1:21"
    sftppath = "sftp://music:x@192.168.1.1:22"
)

// http://localhost:8080/iso/ - content of ISO-image
// http://localhost:8080/dav/ - content of WebDAV-server
// http://localhost:8080/ftp/ - content of FTP-server
// http://localhost:8080/sftp/ - content of SFTP-server
func main() {
    http.Handle("/iso/", http.StripPrefix("/iso/", http.FileServer(
        http.FS(jnt.NewJointCache(isopath, jnt.NewIsoJoint)))))
    http.Handle("/dav/", http.StripPrefix("/dav/", http.FileServer(
        http.FS(jnt.NewJointCache(davpath, jnt.NewDavJoint)))))
    http.Handle("/ftp/", http.StripPrefix("/ftp/", http.FileServer(
        http.FS(jnt.NewJointCache(ftppath, jnt.NewFtpJoint)))))
    http.Handle("/sftp/", http.StripPrefix("/sftp/", http.FileServer(
        http.FS(jnt.NewJointCache(sftppath, jnt.NewSftpJoint)))))
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```
