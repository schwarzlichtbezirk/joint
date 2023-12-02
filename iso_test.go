package joint_test

import (
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"path"
	"testing"

	jnt "github.com/schwarzlichtbezirk/joint"
)

// precalculated CRC32 codes with IEEE polynomial of files in ISO-images.
var filecrc = map[string]uint32{
	"fox.txt":      0x519025e9,
	"doc1.txt":     0x98b2c5bd,
	"doc2.txt":     0xf3cb012f,
	"lorem1.txt":   0x7e4d5e9b,
	"lorem2.txt":   0xa764fec7,
	"lorem3.txt":   0x71a2e97e,
	"рыба.txt":     0xcd6fc22a, // cyrillic name with Windows-1251 encoding for ISO-9660
	"док1.txt":     0x3d4fdf17, // cyrillic name
	"док2.txt":     0x42d2236a, // cyrillic name
	"internal.iso": 0xf4c1b74d,
}

func checkFile(j jnt.Joint, fpath string) (err error) {
	if j.Busy() {
		return fmt.Errorf("joint '%s' is busy before opening", fpath)
	}

	if _, err = j.Open(fpath); err != nil {
		return err
	}
	defer j.Close()

	if !j.Busy() {
		return fmt.Errorf("joint '%s' is not busy after opening", fpath)
	}

	var fi fs.FileInfo
	if fi, err = j.Stat(); err != nil {
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("path '%s' is directory, file expected", fpath)
	}

	var data []byte
	if data, err = io.ReadAll(j); err != nil {
		return err
	}

	var crc = crc32.ChecksumIEEE(data)
	var master, ok = filecrc[path.Base(fpath)]
	if !ok {
		return fmt.Errorf("file with path '%s' is not found", fpath)
	}
	if crc != master {
		return fmt.Errorf("file content of '%s' does not match by CRC-code to precalculated value", fpath)
	}
	return nil
}

func checkDir(j jnt.Joint, fpath string, dirs map[string][]string) (err error) {
	if j.Busy() {
		return fmt.Errorf("joint '%s' is busy before opening", fpath)
	}

	if _, err = j.Open(fpath); err != nil {
		return err
	}
	defer j.Close()

	if !j.Busy() {
		return fmt.Errorf("joint '%s' is not busy after opening", fpath)
	}

	var fi fs.FileInfo
	if fi, err = j.Stat(); err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("path '%s' is file, directory expected", fpath)
	}

	var files []fs.DirEntry
	if files, err = j.ReadDir(-1); err != nil {
		return err
	}

	var list, ok = dirs[fpath]
	if !ok {
		return fmt.Errorf("directory with path '%s' is not found", fpath)
	}
	if len(files) != len(list) {
		return fmt.Errorf("expected %d files, found %d files at directory '%s'", len(list), len(files), fpath)
	}
	var found = true
	for _, fi := range files {
		var name = fi.Name()
		for _, n := range list {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected file '%s' at path '%s' is not found", name, fpath)
		}
	}
	return nil
}

// Open file "fox.txt" with content "The quick brown fox
// jumps over the lazy dog." and read chunks from it.
func TestReadChunk(t *testing.T) {
	var err error

	var j jnt.Joint = &jnt.IsoJoint{}
	if err = j.Make(nil, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	if _, err = j.Open("fox.txt"); err != nil {
		t.Fatal(err)
	}
	defer j.Close()

	var b1 [9]byte // buffer for "brown fox" chunk from file content
	if _, err = j.ReadAt(b1[:], 10); err != nil {
		t.Fatal(err)
	}
	if string(b1[:]) != "brown fox" {
		t.Fatal("read string does not match to pattern")
	}

	var b2 [8]byte // buffer for "lazy dog" chunk from file content
	if _, err = j.Seek(35, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err = j.Read(b2[:]); err != nil {
		t.Fatal(err)
	}
	if string(b2[:]) != "lazy dog" {
		t.Fatal("read string does not match to pattern")
	}
}

// Check file reading in external ISO-disk.
func TestExtReadFile(t *testing.T) {
	var err error

	var j jnt.Joint = &jnt.IsoJoint{}
	if err = j.Make(nil, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	var files = []string{
		"fox.txt",
		"data/lorem1.txt",
		"data/lorem2.txt",
		"data/lorem3.txt",
		"data/рыба.txt",
		"data/docs/doc1.txt",
		"data/docs/doc2.txt",
		"data/доки/док1.txt",
		"data/доки/док2.txt",
		"disk/internal.iso",
	}
	for _, fpath := range files {
		if err = checkFile(j, fpath); err != nil {
			t.Fatal(err)
		}
	}
}

// Check file reading in internal ISO-disk.
func TestIntReadFile(t *testing.T) {
	var err error

	var j1 jnt.Joint = &jnt.IsoJoint{}
	if err = j1.Make(nil, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}

	var j2 jnt.Joint = &jnt.IsoJoint{}
	if err = j2.Make(j1, "disk/internal.iso"); err != nil {
		t.Fatal(err)
	}
	defer j2.Cleanup() // only top-level joint must be called for Cleanup

	var files = []string{
		"fox.txt",
		"docs/doc1.txt",
		"docs/doc2.txt",
		"доки/док1.txt",
		"доки/док2.txt",
	}
	for _, fpath := range files {
		if err = checkFile(j2, fpath); err != nil {
			t.Fatal(err)
		}
	}
}

// Check directory list in external ISO-disk.
func TestExtDirList(t *testing.T) {
	var err error

	var j jnt.Joint = &jnt.IsoJoint{}
	if err = j.Make(nil, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	var dirs = map[string][]string{
		"":           {"fox.txt", "data", "disk"},
		"data":       {"lorem1.txt", "lorem2.txt", "lorem3.txt", "рыба.txt", "docs", "доки", "empty"},
		"disk":       {"internal.iso"},
		"data/docs":  {"doc1.txt", "doc2.txt"},
		"data/доки":  {"док1.txt", "док2.txt"},
		"data/empty": {},
	}
	for fpath := range dirs {
		if err = checkDir(j, fpath, dirs); err != nil {
			t.Fatal(err)
		}
	}
}

// Check directory list in internal ISO-disk.
func TestIntDirList(t *testing.T) {
	var err error

	var j1 jnt.Joint = &jnt.IsoJoint{}
	if err = j1.Make(nil, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}

	var j2 jnt.Joint = &jnt.IsoJoint{}
	if err = j2.Make(j1, "disk/internal.iso"); err != nil {
		t.Fatal(err)
	}
	defer j2.Cleanup() // only top-level joint must be called for Cleanup

	var dirs = map[string][]string{
		"":     {"fox.txt", "docs", "доки"},
		"docs": {"doc1.txt", "doc2.txt"},
		"доки": {"док1.txt", "док2.txt"},
	}
	for fpath := range dirs {
		if err = checkDir(j2, fpath, dirs); err != nil {
			t.Fatal(err)
		}
	}
}
