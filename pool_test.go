package joint_test

import (
	"io/fs"
	"testing"

	jnt "github.com/schwarzlichtbezirk/joint"
)

func TestMakeJoint(t *testing.T) {
	var j, err = jnt.MakeJoint("testdata/external.iso/disk/internal.iso")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Cleanup()

	var f fs.File
	if f, err = j.Open("docs/doc2.txt"); err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var b [9]byte // buffer for "totam rem" chunk from file content
	if _, err = j.ReadAt(b[:], 99); err != nil {
		t.Fatal(err)
	}
	if string(b[:]) != "totam rem" {
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

func TestOpenFile(t *testing.T) {
	var jp = jnt.NewJointPool()
	defer jp.Close()

	var f, err = jp.Open("testdata/external.iso/disk/internal.iso/docs/doc1.txt")
	if err != nil {
		t.Fatal(err)
	}

	var jw, ok = f.(jnt.JointWrap)
	if !ok {
		t.Fatal("can not cast joint to wrapper")
	}

	var b [11]byte // buffer for "ipsum dolor" chunk from file content
	if _, err = jw.ReadAt(b[:], 6); err != nil {
		t.Fatal(err)
	}
	if string(b[:]) != "ipsum dolor" {
		t.Fatal("read string does not match to pattern")
	}

	var jc = jw.GetCache()
	if jc == nil {
		t.Fatal("joint cache does not set")
	}
	if jc.Key() != "testdata/external.iso/disk/internal.iso" {
		t.Fatalf("joint cache key '%s' does not match to expected", jc.Key())
	}
	var c1 = jc.Count()
	f.Close()
	var c2 = jc.Count()
	if c2-c1 != 1 {
		t.Fatalf("joint cache have %d before close, after %d", c1, c2)
	}
}
