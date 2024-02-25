package joint_test

// ATTENTION!
// To pass this test in its entirety should be set environment variable
// JOINT_FTP to address of test FTP-service with credentials. For example,
// `set JOINT_FTP=ftp://user:password@192.168.1.1:21`
// Then copy 'testdata' folder with ISO-file to the FTP root folder as is.

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
		t.Log("environment variable JOINT_FTP does not set, test on FTP joints skipped")
		return // skip test if JOINT_FTP is not set
	}

	var j = &jnt.FtpJoint{}
	if err = j.Make(nil, ftpaddr); err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	if err = j.ChangeDir("testdata"); err != nil {
		t.Fatal(err)
	}

	var fi fs.FileInfo
	if fi, err = j.Info("external.iso"); err != nil {
		t.Fatal(err)
	}

	if fi.Size() != isosize {
		t.Fatal("ISO-file size does not match")
	}

	if err = j.ChangeDirToParent(); err != nil {
		t.Fatal(err)
	}

	if err = checkReadDir(j); err != nil {
		t.Fatal(err)
	}
}

// Check file reading in external ISO-disk placed at FTP.
func TestFtpExtReadFile(t *testing.T) {
	var err error

	var ftpaddr string
	if ftpaddr = os.Getenv(ftpenv); ftpaddr == "" {
		return // skip test if JOINT_FTP is not set
	}

	var j1 jnt.Joint = &jnt.FtpJoint{}
	if err = j1.Make(nil, ftpaddr); err != nil {
		t.Fatal(err)
	}
	defer j1.Cleanup() // Cleanup can be called twice

	var j2 jnt.Joint = &jnt.IsoJoint{}
	if err = j2.Make(j1, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}
	defer j2.Cleanup()

	for _, fpath := range extfiles {
		if err = checkFile(j2, fpath); err != nil {
			t.Fatal(err)
		}
	}
}

// Check file reading in internal ISO-disk placed at FTP.
func TestFtpIntReadFile(t *testing.T) {
	var err error

	var ftpaddr string
	if ftpaddr = os.Getenv(ftpenv); ftpaddr == "" {
		return // skip test if JOINT_FTP is not set
	}

	var j1 jnt.Joint = &jnt.FtpJoint{}
	if err = j1.Make(nil, ftpaddr); err != nil {
		t.Fatal(err)
	}

	var j2 jnt.Joint = &jnt.IsoJoint{}
	if err = j2.Make(j1, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}

	var j3 jnt.Joint = &jnt.IsoJoint{}
	if err = j3.Make(j2, "disk/internal.iso"); err != nil {
		t.Fatal(err)
	}
	defer j3.Cleanup() // only top-level joint must be called for Cleanup

	for _, fpath := range intfiles {
		if err = checkFile(j3, fpath); err != nil {
			t.Fatal(err)
		}
	}
}

// Check directory list in external ISO-disk placed at FTP.
func TestFtpExtDirList(t *testing.T) {
	var err error

	var ftpaddr string
	if ftpaddr = os.Getenv(ftpenv); ftpaddr == "" {
		return // skip test if JOINT_FTP is not set
	}

	var j1 jnt.Joint = &jnt.FtpJoint{}
	if err = j1.Make(nil, ftpaddr); err != nil {
		t.Fatal(err)
	}

	var j2 jnt.Joint = &jnt.IsoJoint{}
	if err = j2.Make(j1, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}
	defer j2.Cleanup()

	for fpath := range extdirs {
		if err = checkDir(j2, fpath, extdirs); err != nil {
			t.Fatal(err)
		}
	}
}

// Check directory list in internal ISO-disk placed at FTP.
func TestFtpIntDirList(t *testing.T) {
	var err error

	var ftpaddr string
	if ftpaddr = os.Getenv(ftpenv); ftpaddr == "" {
		return // skip test if JOINT_FTP is not set
	}

	var j1 jnt.Joint = &jnt.FtpJoint{}
	if err = j1.Make(nil, ftpaddr); err != nil {
		t.Fatal(err)
	}

	var j2 jnt.Joint = &jnt.IsoJoint{}
	if err = j2.Make(j1, "testdata/external.iso"); err != nil {
		t.Fatal(err)
	}

	var j3 jnt.Joint = &jnt.IsoJoint{}
	if err = j3.Make(j2, "disk/internal.iso"); err != nil {
		t.Fatal(err)
	}
	defer j3.Cleanup() // only top-level joint must be called for Cleanup

	for fpath := range intdirs {
		if err = checkDir(j3, fpath, intdirs); err != nil {
			t.Fatal(err)
		}
	}
}

// Read files chunks on FtpJoint.
func TestFtpReadChunk(t *testing.T) {
	var err error

	var ftpaddr string
	if ftpaddr = os.Getenv(ftpenv); ftpaddr == "" {
		return // skip test if JOINT_FTP is not set
	}

	var j1 jnt.Joint = &jnt.FtpJoint{}
	if err = j1.Make(nil, ftpaddr); err != nil {
		t.Fatal(err)
	}
	defer j1.Cleanup()

	var j2 jnt.Joint = &jnt.IsoJoint{}
	if err = readChunk(j2, j1); err != nil {
		t.Fatal(err)
	}
}
