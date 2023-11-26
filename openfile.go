package joint

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// RFile combines fs.File interface and io.Seeker interface.
type RFile interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	fs.File
}

// OpenFile opens file from file system, or looking for iso-disk in the given path,
// opens it, and opens nested into iso-disk file. Or opens file at cloud.
func OpenFile(anypath string) (j Joint, err error) {
	if strings.HasPrefix(anypath, "ftp://") {
		var addr, fpath = SplitUrl(anypath)
		var jc = GetJointCache(addr, NewFtpJoint)
		if j, err = jc.Get(); err != nil {
			return
		}
		if _, err = j.Open(fpath); err != nil {
			return
		}
		return
	} else if strings.HasPrefix(anypath, "sftp://") {
		var addr, fpath = SplitUrl(anypath)
		var jc = GetJointCache(addr, NewSftpJoint)
		if j, err = jc.Get(); err != nil {
			return
		}
		if _, err = j.Open(fpath); err != nil {
			return
		}
		return
	} else if strings.HasPrefix(anypath, "http://") || strings.HasPrefix(anypath, "https://") {
		var addr, fpath, ok = GetDavPath(anypath)
		if !ok {
			err = fs.ErrNotExist
			return
		}
		var jc = GetJointCache(addr, NewDavJoint)
		if j, err = jc.Get(); err != nil {
			return
		}
		if _, err = j.Open(fpath); err != nil {
			return
		}
		return
	} else {
		var f *os.File
		if f, err = os.Open(anypath); err == nil { // primary filesystem file
			j = &SysJoint{"", f}
			return
		}
		var file io.Closer = io.NopCloser(nil) // empty closer stub

		// looking for nested file
		var isopath = anypath
		for errors.Is(err, fs.ErrNotExist) && isopath != "." && isopath != "/" {
			isopath = path.Dir(isopath)
			file, err = os.Open(isopath)
		}
		if err != nil {
			return
		}
		file.Close()

		var fpath string
		if isopath == anypath {
			fpath = "" // get root of disk
		} else {
			fpath = anypath[len(isopath)+1:] // without slash prefix
		}

		var jc = GetJointCache(isopath, NewIsoJoint)
		if j, err = jc.Get(); err != nil {
			return
		}
		if _, err = j.Open(fpath); err != nil {
			return
		}
		return
	}
}

// StatFile returns fs.FileInfo of file in file system,
// or file nested in disk image, or cloud file.
func StatFile(anypath string) (fi fs.FileInfo, err error) {
	var j Joint
	if j, err = OpenFile(anypath); err != nil {
		return
	}
	defer func() {
		if err != nil {
			j.Cleanup()
		} else {
			j.Close()
		}
	}()
	return j.Stat()
}

// ReadDir returns directory files fs.DirEntry list. It scan file system path,
// or looking for iso-disk in the given path, opens it, and scan files nested
// into iso-disk local directory. Or reads directory at cloud path.
func ReadDir(anypath string) (ret []fs.DirEntry, err error) {
	var j Joint
	if j, err = OpenFile(anypath); err != nil {
		return
	}
	defer func() {
		if err != nil {
			j.Cleanup()
		} else {
			j.Close()
		}
	}()
	return j.ReadDir(-1)
}

type FS string

// JoinFast performs fast join of two path chunks.
func JoinFast(dir, base string) string {
	if dir == "" || dir == "." {
		return base
	}
	if base == "" || base == "." {
		return dir
	}
	if dir[len(dir)-1] == '/' {
		if base[0] == '/' {
			return dir + base[1:]
		} else {
			return dir + base
		}
	}
	if base[0] == '/' {
		return dir + base
	}
	return dir + "/" + base
}

func (fsys FS) Open(fpath string) (r fs.File, err error) {
	return OpenFile(JoinFast(string(fsys), fpath))
}

func (fsys FS) Stat(fpath string) (fi fs.FileInfo, err error) {
	return StatFile(JoinFast(string(fsys), fpath))
}

func (fsys FS) ReadDir(fpath string) (ret []fs.DirEntry, err error) {
	return ReadDir(JoinFast(string(fsys), fpath))
}

// The End.