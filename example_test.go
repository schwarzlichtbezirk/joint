package joint_test

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	jnt "github.com/schwarzlichtbezirk/joint"
)

// JoinPath can be used for high performance path concatenation.
func ExampleJoinPath() {
	fmt.Println(jnt.JoinPath("any/path", "fox.txt"))
	fmt.Println(jnt.JoinPath("any/path/", "fox.txt"))
	fmt.Println(jnt.JoinPath("any", "/path/fox.txt"))
	fmt.Println(jnt.JoinPath("any/path/", "/fox.txt"))
	fmt.Println(jnt.JoinPath("/any/path", "fox.txt"))
	// Output:
	// any/path/fox.txt
	// any/path/fox.txt
	// any/path/fox.txt
	// any/path/fox.txt
	// /any/path/fox.txt
}

func ExampleSplitUrl() {
	var list = []string{
		"ftp://music:x@192.168.1.1:21/Music/DJ.m3u",
		"sftp://music:x@192.168.1.1:22/Video",
		"https://github.com/schwarzlichtbezirk/joint",
		"https://pkg.go.dev/github.com/schwarzlichtbezirk/joint",
		"C:\\Windows\\System",
	}
	for _, s := range list {
		var addr, fpath, url = jnt.SplitUrl(s)
		fmt.Printf("addr: %s, path: %s, url: %t\n", addr, fpath, url)
	}
	// Output:
	// addr: ftp://music:x@192.168.1.1:21, path: Music/DJ.m3u, url: true
	// addr: sftp://music:x@192.168.1.1:22, path: Video, url: true
	// addr: https://github.com, path: schwarzlichtbezirk/joint, url: true
	// addr: https://pkg.go.dev, path: github.com/schwarzlichtbezirk/joint, url: true
	// addr: C:, path: Windows\System, url: false
}

func ExampleSplitKey() {
	var list = []string{
		"some/path/fox.txt",
		"testdata/external.iso",
		"testdata/external.iso/fox.txt",
		"testdata/external.iso/disk/internal.iso/fox.txt",
		"ftp://music:x@192.168.1.1:21/Music",
		"ftp://music:x@192.168.1.1:21/testdata/external.iso/disk/internal.iso/docs/doc1.txt",
		"https://music:x@example.keenetic.link/webdav/Global%20Underground/Nubreed/",
	}
	jnt.SetDavRoot("https://music:x@example.keenetic.link", "/webdav/")
	for _, s := range list {
		var key, fpath, _ = jnt.SplitKey(s)
		fmt.Printf("key: '%s', path: '%s'\n", key, fpath)
	}
	// Output:
	// key: '', path: 'some/path/fox.txt'
	// key: 'testdata/external.iso', path: ''
	// key: 'testdata/external.iso', path: 'fox.txt'
	// key: 'testdata/external.iso/disk/internal.iso', path: 'fox.txt'
	// key: 'ftp://music:x@192.168.1.1:21', path: 'Music'
	// key: 'ftp://music:x@192.168.1.1:21/testdata/external.iso/disk/internal.iso', path: 'docs/doc1.txt'
	// key: 'https://music:x@example.keenetic.link/webdav/', path: 'Global%20Underground/Nubreed/'
}

func ExampleFtpEscapeBrackets() {
	fmt.Println(jnt.FtpEscapeBrackets("Music/Denney [2018]"))
	// Output: Music/Denney [[]2018[]]
}

func ExampleJointCache_Open() {
	var jc = jnt.NewJointCache("testdata/external.iso")
	defer jc.Close()

	var f, err = jc.Open("fox.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	io.Copy(os.Stdout, f)
	// Output: The quick brown fox jumps over the lazy dog.
}

// Opens file on joints pool. Path to file can starts with
// WebDAV/SFTP/FTP service address or at local filesystem, and
// include ISO9660 disks as chunks of path.
func ExampleJointPool_Open() {
	var jp = jnt.NewJointPool() // can be global declaration
	defer jp.Close()

	var f, err = jp.Open("testdata/external.iso/fox.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	io.Copy(os.Stdout, f)
	// Output: The quick brown fox jumps over the lazy dog.
}

func ExampleJointPool_Stat() {
	var jp = jnt.NewJointPool() // can be global declaration
	defer jp.Close()

	var fi, err = jp.Stat("testdata/external.iso/fox.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("name: %s, size: %d\n", fi.Name(), fi.Size())
	// Output:
	// name: fox.txt, size: 44
}

func ExampleJointPool_ReadDir() {
	var jp = jnt.NewJointPool() // can be global declaration
	defer jp.Close()

	var files, err = jp.ReadDir("testdata/external.iso/data")
	if err != nil {
		log.Fatal(err)
	}
	for _, de := range files {
		if de.IsDir() {
			fmt.Printf("dir:  %s\n", de.Name())
		} else {
			var fi, _ = de.Info()
			fmt.Printf("file: %s, %d bytes\n", de.Name(), fi.Size())
		}
	}
	// Output:
	// dir:  docs
	// dir:  empty
	// file: lorem1.txt, 2747 bytes
	// file: lorem2.txt, 2629 bytes
	// file: lorem3.txt, 2714 bytes
	// dir:  доки
	// file: рыба.txt, 1789 bytes
}

// Open http://localhost:8080/ in browser
// to get a list of files in ISO-image.
func ExampleJointPool_Sub() {
	// create map with caches for all currently unused joints
	var jp = jnt.NewJointPool()
	// file system, that shares content of "testdata" folder
	// and all embedded into ISO-disks files
	var sp, err = jp.Sub("testdata")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(sp)))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
