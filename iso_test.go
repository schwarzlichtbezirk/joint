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

const testpath = "testdata"

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

var dirext = map[string][]string{
	"":           {"fox.txt", "data", "disk"},
	"data":       {"lorem1.txt", "lorem2.txt", "lorem3.txt", "рыба.txt", "docs", "доки", "empty"},
	"disk":       {"internal.iso"},
	"data/docs":  {"doc1.txt", "doc2.txt"},
	"data/доки":  {"док1.txt", "док2.txt"},
	"data/empty": {},
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

func checkDir(j jnt.Joint, fpath string) (err error) {
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

	var list, ok = dirext[fpath]
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

// Check file reading in external ISO-disk.
func TestExtReadFile(t *testing.T) {
	var err error

	var j jnt.Joint = &jnt.IsoJoint{}
	if err = j.Make(jnt.JoinFast(testpath, "external.iso")); err != nil {
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

// Check directory list in external ISO-disk.
func TestExtDirList(t *testing.T) {
	var err error

	var j jnt.Joint = &jnt.IsoJoint{}
	if err = j.Make(jnt.JoinFast(testpath, "external.iso")); err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	for fpath := range dirext {
		if err = checkDir(j, fpath); err != nil {
			t.Fatal(err)
		}
	}
}
