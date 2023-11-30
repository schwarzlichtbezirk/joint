package joint

import (
	"io/fs"
	"os"
)

type SysJoint struct {
	dir string
	*os.File
}

func NewSysJoint() Joint {
	return &SysJoint{}
}

func (j *SysJoint) Make(base Joint, dir string) (err error) {
	j.dir = dir
	return
}

func (j *SysJoint) Cleanup() error {
	var err1 error
	if j.Busy() {
		err1 = j.Close()
	}
	return err1
}

func (j *SysJoint) Busy() bool {
	return j.File != nil
}

// Opens file at local file system.
func (j *SysJoint) Open(fpath string) (file fs.File, err error) {
	if j.File, err = os.Open(JoinFast(j.dir, fpath)); err != nil {
		return
	}
	file = j
	return
}

func (j *SysJoint) Close() (err error) {
	if j.File != nil {
		err = j.File.Close()
		j.File = nil
	}
	return
}

func (j *SysJoint) Info(fpath string) (fs.FileInfo, error) {
	return os.Stat(JoinFast(j.dir, fpath))
}

// The End.
