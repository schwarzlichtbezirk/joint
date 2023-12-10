package joint_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	jnt "github.com/schwarzlichtbezirk/joint"
)

// Files list to test joints cache.
var jcfiles = []string{
	"fox.txt",
	"data/lorem1.txt",
	"data/рыба.txt",
	"data/docs/doc1.txt",
	"data/доки/док2.txt",
}

// Test how JointWrapper is work.
func TestWrapper(t *testing.T) {
	var err error
	var jw jnt.JointWrap

	var jc = jnt.NewJointCache("testdata/external.iso")
	defer jc.Close()

	// create joint
	if jw, err = jc.Get(); err != nil {
		t.Fatal(err)
	}
	if jc.Count() != 0 {
		t.Fatalf("expected %d joints in cache, got %d", 0, jc.Count())
	}
	// use joint
	f, err := jw.Open("fox.txt")
	if err != nil {
		t.Fatal(err)
	}
	// close file and do not put joint to cache
	f.Close()
	if jc.Count() != 0 {
		t.Fatalf("expected %d joints in cache, got %d", 0, jc.Count())
	}
	// use joint again
	if _, err = jw.Open("fox.txt"); err != nil {
		t.Fatal(err)
	}
	// close joint and put it to cache
	jw.Close()
	if jc.Count() != 1 {
		t.Fatalf("expected %d joints in cache, got %d", 1, jc.Count())
	}

	// get joint from cache and use it
	if jw, err = jc.Get(); err != nil {
		t.Fatal(err)
	}
	if _, err = jw.Open("fox.txt"); err != nil {
		t.Fatal(err)
	}
	if !jw.Busy() {
		t.Fatal("joint is not busy after opening")
	}
	// ensure that Cleanup closes file and does not put joint to cache
	jw.Cleanup()
	if jw.Busy() {
		t.Fatal("joint is busy after closing")
	}
	if jc.Count() != 0 {
		t.Fatalf("expected %d joints in cache, got %d", 0, jc.Count())
	}
}

// Test get and put cache functions.
func TestCacheGetPut(t *testing.T) {
	var err error
	var j1, j2 any
	var jw jnt.JointWrap

	var jc = jnt.NewJointCache("testdata/external.iso")
	defer jc.Close()

	// create joint
	if jw, err = jc.Get(); err != nil {
		t.Fatal(err)
	}
	j1 = jw.Joint
	if jc.Count() != 0 {
		t.Fatalf("expected %d joints in cache, got %d", 0, jc.Count())
	}
	if _, err = jw.Open("fox.txt"); err != nil {
		t.Fatal(err)
	}
	// put joint to cache after use
	jw.Close()
	if jc.Count() != 1 {
		t.Fatalf("expected %d joints in cache, got %d", 1, jc.Count())
	}
	// get used joint from cache
	if jw, err = jc.Get(); err != nil {
		t.Fatal(err)
	}
	j2 = jw.Joint
	if jc.Count() != 0 {
		t.Fatalf("expected %d joints in cache, got %d", 0, jc.Count())
	}
	// ensure that it is the same object
	if j1 != j2 {
		t.Fatal("joint must be reused, got new object", 0, jc.Count())
	}
	// force to put and eject joint
	jc.Put(jw)
	if !jc.Has(jw) {
		t.Fatalf("joint does not found in the cache")
	}
	if !jc.Eject(jw) {
		t.Fatal("can not eject joint from cache")
	}
	if jc.Has(jw) {
		t.Fatalf("joint does not ejected")
	}
}

func TestCacheOpen(t *testing.T) {
	var err error

	var jc = jnt.NewJointCache("testdata/external.iso")
	defer jc.Close()

	var files = make([]fs.File, len(jcfiles))
	if jc.Count() != 0 {
		t.Fatalf("expected %d joints in cache, got %d", 0, jc.Count())
	}
	// open several files at once
	for i, fpath := range jcfiles {
		if files[i], err = jc.Open(fpath); err != nil {
			t.Fatal(err)
		}
	}
	if jc.Count() != 0 {
		t.Fatalf("expected %d joints in cache, got %d", 0, jc.Count())
	}
	// close all opened files
	for _, f := range files {
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// all joints of those files should be in cache
	if jc.Count() != len(jcfiles) {
		t.Fatalf("expected %d joints in cache, got %d", len(jcfiles), jc.Count())
	}
	// open some file again
	var f fs.File
	if f, err = jc.Open(jcfiles[0]); err != nil {
		t.Fatal(err)
	}
	// joint for this file should been taken from cache
	if jc.Count() != len(jcfiles)-1 {
		t.Fatalf("expected %d joints in cache, got %d", len(jcfiles)-1, jc.Count())
	}
	f.Close()
	// cache should be restored
	if jc.Count() != len(jcfiles) {
		t.Fatalf("expected %d joints in cache, got %d", len(jcfiles), jc.Count())
	}
}

// Note that JointCache file system has undefined behaviour
// for internal ISO-files.
func TestCacheFS(t *testing.T) {
	var err error

	var jc = jnt.NewJointCache("testdata/external.iso/disk/internal.iso")
	defer jc.Close()

	// test FS at the end
	if err = fstest.TestFS(jc, intfiles...); err != nil {
		t.Fatal(err)
	}
}
