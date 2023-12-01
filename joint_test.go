package joint_test

import (
	"io/fs"
	"testing"

	jnt "github.com/schwarzlichtbezirk/joint"
)

// Test how JointWrapper is work.
func TestWrapper(t *testing.T) {
	var err error
	var jw jnt.JointWrap

	var jc = jnt.NewJointCache(extpath)
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

	var jc = jnt.NewJointCache(extpath)
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
	if !jw.Eject() {
		t.Fatal("can not eject joint from cache")
	}
	if jc.Has(jw) {
		t.Fatalf("joint does not ejected")
	}
}

func TestMakeJoint(t *testing.T) {
	var j, err = jnt.MakeJoint("testdata/external.iso/disk/internal.iso")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	var f fs.File
	if f, err = j.Open("fox.txt"); err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var b [9]byte // buffer for "brown fox" chunk from file content
	if _, err = j.ReadAt(b[:], 10); err != nil {
		t.Fatal(err)
	}
	if string(b[:]) != "brown fox" {
		t.Fatal("read string does not match to pattern")
	}

	// check up joints chain
	var ok bool
	var j0, j1 *jnt.IsoJoint
	if j0, ok = j.(*jnt.IsoJoint); !ok {
		t.Fatal("can not cast to ISO joint")
	}
	if j1, ok = j0.Base.(*jnt.IsoJoint); !ok {
		t.Fatal("can not cast base to ISO joint")
	}
	if _, ok = j1.Base.(*jnt.SysJoint); !ok {
		t.Fatal("can not cast primary joint to system joint")
	}
}
