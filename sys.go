package joint

import (
	"errors"
	"io/fs"
	"os"
)

type SysJoint struct {
	dir string
	*os.File
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
	if j.Busy() {
		return nil, fs.ErrExist
	}
	if j.File, err = os.Open(JoinFast(j.dir, fpath)); err != nil {
		return
	}
	return j, nil
}

func (j *SysJoint) Close() (err error) {
	if j.File != nil {
		err = j.File.Close()
		j.File = nil
	}
	return
}

func (j *SysJoint) Size() int64 {
	var fi, err = j.File.Stat()
	if err != nil {
		return 0
	}
	return fi.Size()
}

func (j *SysJoint) ReadDir(n int) ([]fs.DirEntry, error) {
	var errs []error
	var list, err = j.File.ReadDir(n)
	if err != nil {
		errs = append(errs, err)
	}
	for i, de := range list {
		var fi, err = de.Info()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		list[i] = ToDirEntry(fi)
	}
	return list, errors.Join(errs...)
}

func (j *SysJoint) Stat() (fs.FileInfo, error) {
	var fi, err = j.File.Stat()
	return ToFileInfo(fi), err
}

func (j *SysJoint) Info(fpath string) (fs.FileInfo, error) {
	var fi, err = os.Stat(JoinFast(j.dir, fpath))
	return ToFileInfo(fi), err
}

// The End.
