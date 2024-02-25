package joint_test

// ATTENTION!
// To pass this test in its entirety should be set environment variable
// JOINT_DAV to URL of test WebDAV-service with credentials. For example,
// `set JOINT_DAV=https://user:password@example.keenetic.link/webdav/`
// Then copy 'testdata' folder with ISO-file to the WebDAV root folder as is.

import (
	"io/fs"
	"os"
	"testing"

	jnt "github.com/schwarzlichtbezirk/joint"
)

// Environment variable name with address of WebDAV service.
const davenv = "JOINT_DAV"

func TestDavJoint(t *testing.T) {
	var err error

	var davaddr string
	if davaddr = os.Getenv(davenv); davaddr == "" {
		t.Log("environment variable JOINT_DAV does not set, test on WebDAV joints skipped")
		return // skip test if JOINT_DAV is not set
	}

	var j = &jnt.DavJoint{}
	if err = j.Make(nil, davaddr); err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	var fi fs.FileInfo
	if fi, err = j.Info("testdata/external.iso"); err != nil {
		t.Fatal(err)
	}

	if fi.Size() != isosize {
		t.Fatal("ISO-file size does not match")
	}

	if err = checkReadDir(j); err != nil {
		t.Fatal(err)
	}
}

// Check file reading in external ISO-disk placed at WebDAV.
func TestDavExtReadFile(t *testing.T) {
	var err error

	var davaddr string
	if davaddr = os.Getenv(davenv); davaddr == "" {
		return // skip test if JOINT_DAV is not set
	}

	var j1 jnt.Joint = &jnt.DavJoint{}
	if err = j1.Make(nil, davaddr); err != nil {
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

// Check file reading in internal ISO-disk placed at WebDAV.
func TestDavIntReadFile(t *testing.T) {
	var err error

	var davaddr string
	if davaddr = os.Getenv(davenv); davaddr == "" {
		return // skip test if JOINT_DAV is not set
	}

	var j1 jnt.Joint = &jnt.DavJoint{}
	if err = j1.Make(nil, davaddr); err != nil {
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

// Check directory list in external ISO-disk placed at WebDAV.
func TestDavExtDirList(t *testing.T) {
	var err error

	var davaddr string
	if davaddr = os.Getenv(davenv); davaddr == "" {
		return // skip test if JOINT_DAV is not set
	}

	var j1 jnt.Joint = &jnt.DavJoint{}
	if err = j1.Make(nil, davaddr); err != nil {
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

// Check directory list in internal ISO-disk placed at WebDAV.
func TestDavIntDirList(t *testing.T) {
	var err error

	var davaddr string
	if davaddr = os.Getenv(davenv); davaddr == "" {
		return // skip test if JOINT_DAV is not set
	}

	var j1 jnt.Joint = &jnt.DavJoint{}
	if err = j1.Make(nil, davaddr); err != nil {
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

// Read files chunks on DavJoint.
func TestDavReadChunk(t *testing.T) {
	var err error

	var davaddr string
	if davaddr = os.Getenv(davenv); davaddr == "" {
		return // skip test if JOINT_DAV is not set
	}

	var j1 jnt.Joint = &jnt.DavJoint{}
	if err = j1.Make(nil, davaddr); err != nil {
		t.Fatal(err)
	}
	defer j1.Cleanup()

	var j2 jnt.Joint = &jnt.IsoJoint{}
	if err = readChunk(j2, j1); err != nil {
		t.Fatal(err)
	}
}
