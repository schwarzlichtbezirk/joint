package joint_test

import (
	"fmt"
	"io"
	"log"
	"os"

	jnt "github.com/schwarzlichtbezirk/joint"
)

func ExampleJoinFast() {
	fmt.Println(jnt.JoinFast("any/path", "fox.txt"))
	fmt.Println(jnt.JoinFast("any/path/", "fox.txt"))
	fmt.Println(jnt.JoinFast("any", "/path/fox.txt"))
	fmt.Println(jnt.JoinFast("any/path/", "/fox.txt"))
	fmt.Println(jnt.JoinFast("/any/path", "fox.txt"))
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
	}
	for _, s := range list {
		var addr, fpath = jnt.SplitUrl(s)
		fmt.Printf("addr: %s, path: %s\n", addr, fpath)
	}
	// Output:
	// addr: ftp://music:x@192.168.1.1:21, path: Music/DJ.m3u
	// addr: sftp://music:x@192.168.1.1:22, path: Video
	// addr: https://github.com, path: schwarzlichtbezirk/joint
	// addr: https://pkg.go.dev, path: github.com/schwarzlichtbezirk/joint
}

func ExampleSplitKey() {
	var list = []string{
		"some/path/fox.txt",
		"testdata/external.iso",
		"testdata/external.iso/fox.txt",
		"testdata/external.iso/disk/internal.iso/fox.txt",
		"ftp://music:x@192.168.1.1:21/Music",
		"ftp://music:x@192.168.1.1:21/testdata/external.iso/disk/internal.iso/docs/doc1.txt",
	}
	for _, s := range list {
		var key, fpath = jnt.SplitKey(s)
		fmt.Printf("key: '%s', path: '%s'\n", key, fpath)
	}
	// Output:
	// key: '', path: 'some/path/fox.txt'
	// key: 'testdata/external.iso', path: ''
	// key: 'testdata/external.iso', path: 'fox.txt'
	// key: 'testdata/external.iso/disk/internal.iso', path: 'fox.txt'
	// key: 'ftp://music:x@192.168.1.1:21', path: 'Music'
	// key: 'ftp://music:x@192.168.1.1:21/testdata/external.iso/disk/internal.iso', path: 'docs/doc1.txt'
}

func ExampleFtpEscapeBrackets() {
	fmt.Println(jnt.FtpEscapeBrackets("Music/Denney [2018]"))
	// Output: Music/Denney [[]2018[]]
}

func ExampleOpenFile() {
	var j, err = jnt.OpenFile("testdata/external.iso/fox.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer j.Close()

	io.Copy(os.Stdout, j)
	// Output: The quick brown fox jumps over the lazy dog.
}

func ExampleStatFile() {
	var fi, err = jnt.StatFile("testdata/external.iso/fox.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("name: %s, size: %d\n", fi.Name(), fi.Size())
	// Output:
	// name: fox.txt, size: 44
}

func ExampleReadDir() {
	var files, err = jnt.ReadDir("testdata/external.iso/data")
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

func ExampleNewJointCache() {
	var jc = jnt.NewJointCache("testdata/external.iso")
	defer jc.Close()
}

func ExampleJointCache_Open() {
	var jc = jnt.GetJointCache("testdata/external.iso")
	var f, err = jc.Open("fox.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	io.Copy(os.Stdout, f)
	// Output: The quick brown fox jumps over the lazy dog.
}

// The End.
