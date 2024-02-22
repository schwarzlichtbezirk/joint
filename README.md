# Joint

Provides access to files in ISO-9660 images, FTP-servers, SFTP-servers, WebDAV-servers by standard file system interfaces. Contains cache with reusable connections to endpoints.

[![Go Reference](https://pkg.go.dev/badge/github.com/schwarzlichtbezirk/joint.svg)](https://pkg.go.dev/github.com/schwarzlichtbezirk/joint)
[![Go Report Card](https://goreportcard.com/badge/github.com/schwarzlichtbezirk/joint)](https://goreportcard.com/report/github.com/schwarzlichtbezirk/joint)
[![Hits-of-Code](https://hitsofcode.com/github/schwarzlichtbezirk/joint?branch=main)](https://hitsofcode.com/github/schwarzlichtbezirk/joint/view?branch=main)

## Goals

You can read the contents of a folder on an FTP-server either sequentially through one connection, but then this will take a lot of time, or by opening multiple connections, in which case too many connections will be opened for multiple requests. This library uses joints, which hold the connection to the FTP-server after reading a file, or some kind of access, after closing the file, they are placed into the cache, and can be reused later. If the connection has not been used for a certain time, it is reset.

## Examples

### HTTP file server with WebDAV content

```go
package main

import (
    "log"
    "net/http"

    jnt "github.com/schwarzlichtbezirk/hms/joint"
)

// Open http://localhost:8080/ in browser
// to get a list of files in WebDAV-server for given user.
func main() {
    var sp = jnt.NewSubPool(nil, "https://music:x@example.keenetic.link/webdav/")
    defer sp.Close()
    http.Handle("/", http.FileServer(http.FS(sp)))
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### HTTP file server with ISO-image content

```go
package main

import (
    "log"
    "net/http"

    jnt "github.com/schwarzlichtbezirk/hms/joint"
)

// Open http://localhost:8080/ in browser
// to get a list of files in ISO-image.
func main() {
    // create map with caches for all currently unused joints
    var jp = jnt.NewJointPool()
    defer jp.Close()
    // file system, that shares content of "testdata" folder
    // and all embedded into ISO-disks files
    var sp, err = jp.Sub("testdata")
    if err != nil {
        log.Fatal(err)
    }
    http.Handle("/", http.FileServer(http.FS(sp)))
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

// http://localhost:8080/iso/ - content of ISO-image
// http://localhost:8080/dav/ - content of WebDAV-server
// http://localhost:8080/ftp/ - content of FTP-server
// http://localhost:8080/sftp/ - content of SFTP-server
func main() {
    // create map with caches for all currently unused joints
    var jp = jnt.NewJointPool()
    defer jp.Close()
    // handle list of resources as binded file systems
    http.Handle("/iso/", http.StripPrefix("/iso/", http.FileServer(
        http.FS(jnt.NewSubPool(jp, "testdata/external.iso")))))
    http.Handle("/dav/", http.StripPrefix("/dav/", http.FileServer(
        http.FS(jnt.NewSubPool(jp, "https://music:x@example.keenetic.link/webdav/")))))
    http.Handle("/ftp/", http.StripPrefix("/ftp/", http.FileServer(
        http.FS(jnt.NewSubPool(jp, "ftp://music:x@192.168.1.1:21")))))
    http.Handle("/sftp/", http.StripPrefix("/sftp/", http.FileServer(
        http.FS(jnt.NewSubPool(jp, "sftp://music:x@192.168.1.1:22")))))
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Files reading by joints

```go
package main

import (
    "fmt"
    "io"
    "io/fs"
    "log"

    jnt "github.com/schwarzlichtbezirk/joint"
)

func main() {
    var err error

    // Create joint to ISO-9660 image.
    var j jnt.Joint = &jnt.IsoJoint{}
    if err = j.Make(nil, "testdata/external.iso"); err != nil {
        log.Fatal(err)
    }
    // Cleanup drops joint's link at the end. Any not cached joints
    // should be destroyed by Cleanup call.
    defer j.Cleanup()

    // Working with file object returned by Open-function.
    // Open-function returns joint casted to fs.File.
    var f fs.File
    if f, err = j.Open("fox.txt"); err != nil {
        log.Fatal(err)
    }
    var b []byte
    if b, err = io.ReadAll(f); err != nil { // read from file
        log.Fatal(err)
    }
    fmt.Println(string(b))
    f.Close()

    // Working with joint explicitly. If joint is received from cache,
    // Close-call will return joint back to cache.
    if _, err = j.Open("data/lorem1.txt"); err != nil {
        log.Fatal(err)
    }
    if _, err = io.Copy(os.Stdout, j); err != nil { // read from joint
        log.Fatal(err)
    }
    j.Close()
}
```

### Open nested ISO-image by joints

```go
package main

import (
    "io"
    "io/fs"
    "log"
    "os"

    jnt "github.com/schwarzlichtbezirk/joint"
)

func main() {
    var err error

    // Create joint to external ISO-image.
    // This joint will be removed by top-level joint.
    var j1 jnt.Joint = &jnt.IsoJoint{}
    if err = j1.Make(nil, "testdata/external.iso"); err != nil {
        log.Fatal(err)
    }
    defer j1.Cleanup()

    // Create top-level joint to internal ISO-image placed inside of first.
    var j2 jnt.Joint = &jnt.IsoJoint{}
    if err = j2.Make(j1, "disk/internal.iso"); err != nil {
        log.Fatal(err)
    }
    // Top-level calls inherited Cleanup, so this call can be one.
    defer j2.Cleanup()

    // Open file at internal ISO-image.
    var f fs.File
    if f, err = j2.Open("fox.txt"); err != nil {
        log.Fatal(err)
    }
    defer f.Close()

    io.Copy(os.Stdout, f)
}
```

### Simple file reading

```go
package main

import (
    "io"
    "log"
    "os"

    jnt "github.com/schwarzlichtbezirk/joint"
)

var jp = jnt.NewJointPool()

func main() {
    // Open file and get the joint by one call.
    var j, err = jp.Open("testdata/external.iso/disk/internal.iso/docs/doc1.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer j.Close()

    io.Copy(os.Stdout, j)
}
```

---
(c) schwarzlichtbezirk, 2023-2024.
