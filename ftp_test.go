package joint_test

// ATTENTION!
// To pass this test in its entirety should be set environment variable
// JOINT_FTP to address of test FTP-service with read access. For example,
// `set JOINT_FTP=ftp://user:password@192.168.1.1:21`
// Then copy 'testdata' folder with ISO-file to the FTP root folder.

import (
	"io/fs"
	"os"
	"testing"

	jnt "github.com/schwarzlichtbezirk/joint"
)

// Environment variable name with address of FTP service.
const ftpenv = "JOINT_FTP"

func TestFtpJoint(t *testing.T) {
	var err error

	var ftpaddr string
	if ftpaddr = os.Getenv(ftpenv); ftpaddr == "" {
		return // skip test if JOINT_FTP is not set
	}

	var j = &jnt.FtpJoint{}
	if err = j.Make(nil, ftpaddr); err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	if j.Busy() {
		t.Fatal("joint is busy before opening")
	}

	var f fs.File
	if f, err = j.Open("testdata"); err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if !j.Busy() {
		t.Fatal("joint is not busy after opening")
	}

	var list []fs.DirEntry
	if list, err = j.ReadDir(-1); err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 file in 'testdata' directory, found %d files", len(list))
	}

	var fi = list[0].(jnt.FtpFileInfo)
	if fi.Name() != "external.iso" {
		t.Fatal("expected 'external.iso' file in 'testdata' directory")
	}

	if fi.IsRealDir() {
		t.Fatal("file 'external.iso' should be recognized as real file")
	}

	if !fi.IsDir() {
		t.Fatal("file 'external.iso' should be recognized as virtual folder")
	}

	if fi.Mode() != 0444|fs.ModeDir || fi.Type() != fs.ModeDir {
		t.Fatal("file 'external.iso' have wrong file mode")
	}
}

// Read files chunks on FtpJoint.
func TestFtpReadChunk(t *testing.T) {
	var err error

	var ftpaddr string
	if ftpaddr = os.Getenv(ftpenv); ftpaddr == "" {
		return // skip test if JOINT_FTP is not set
	}

	var j1 = &jnt.FtpJoint{}
	if err = j1.Make(nil, ftpaddr); err != nil {
		t.Fatal(err)
	}
	defer j1.Cleanup()

	var j2 = &jnt.IsoJoint{}
	if err = readChunk(j2, j1); err != nil {
		t.Fatal(err)
	}
}
