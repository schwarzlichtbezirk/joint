package joint

import (
	"io/fs"
	"strings"
)

// IsTypeIso checks that file extension is ISO-disk.
func IsTypeIso(fpath string) bool {
	if len(fpath) < 4 {
		return false
	}
	var ext = fpath[len(fpath)-4:]
	return ext == ".iso" || ext == ".ISO"
}

// MakeJoint creates joint with all subsequent chain of joints.
// Please note that folders with .iso extension and non ISO-images
// with .iso extension will cause an error.
func MakeJoint(fullpath string) (j Joint, err error) {
	var localpath string
	if strings.HasPrefix(fullpath, "ftp://") {
		var addr string
		addr, localpath = SplitUrl(fullpath)
		j = &FtpJoint{}
		if err = j.Make(nil, addr); err != nil {
			return
		}
	} else if strings.HasPrefix(fullpath, "sftp://") {
		var addr string
		addr, localpath = SplitUrl(fullpath)
		j = &SftpJoint{}
		if err = j.Make(nil, addr); err != nil {
			return
		}
	} else if strings.HasPrefix(fullpath, "http://") || strings.HasPrefix(fullpath, "https://") {
		var addr string
		var ok bool
		if addr, localpath, ok = GetDavPath(fullpath); !ok {
			err = fs.ErrNotExist
			return
		}
		j = &DavJoint{}
		if err = j.Make(nil, addr); err != nil {
			return
		}
	} else {
		localpath = fullpath
		j = &SysJoint{}
	}

	var jpos = 0
	var chain = strings.Split(localpath, "/")
	for i, chunk := range chain {
		if !IsTypeIso(chunk) {
			continue
		}
		var fpath = strings.Join(chain[jpos:i+1], "/")
		var jiso = &IsoJoint{}
		if err = jiso.Make(j, fpath); err != nil {
			return
		}
		j, jpos = jiso, i+1
	}
	return
}

// SplitUrl splits URL to address string and to path as is.
func SplitUrl(urlpath string) (string, string) {
	if i := strings.Index(urlpath, "://"); i != -1 {
		if j := strings.Index(urlpath[i+3:], "/"); j != -1 {
			return urlpath[:i+3+j], urlpath[i+3+j+1:]
		}
		return urlpath, ""
	}
	return "", urlpath
}

// SplitKey splits full path to joint key to establish link, and
// remained local path.
func SplitKey(fullpath string) (key, fpath string) {
	if IsTypeIso(fullpath) {
		return fullpath, ""
	}
	var p = max(
		strings.LastIndex(fullpath, ".iso/"),
		strings.LastIndex(fullpath, ".ISO/"))
	if p != -1 {
		return fullpath[:p+4], fullpath[p+5:]
	}
	if strings.HasPrefix(fullpath, "http://") || strings.HasPrefix(fullpath, "https://") {
		key, fpath, _ = GetDavPath(fullpath)
		return
	}
	return SplitUrl(fullpath)
}

// OpenFile opens file with given full path to this file,
// that can be located inside of nested ISO-images and/or
// on FTP, SFTP, WebDAV servers.
func OpenFile(fullpath string) (j Joint, err error) {
	var key, fpath = SplitKey(fullpath)
	if key == "" {
		j = &SysJoint{}
		_, err = j.Open(fpath)
		return
	}
	var jc = GetJointCache(key)
	var f fs.File
	if f, err = jc.Open(fpath); err != nil {
		return
	}
	j = f.(Joint)
	return
}

// StatFile returns fs.FileInfo of file pointed by given full path.
func StatFile(fullpath string) (fi fs.FileInfo, err error) {
	var j Joint
	if j, err = OpenFile(fullpath); err != nil {
		return
	}
	defer j.Close()
	return j.Stat()
}

// ReadDir returns directory files fs.DirEntry list pointed by given full path.
func ReadDir(fullpath string) (ret []fs.DirEntry, err error) {
	var j Joint
	if j, err = OpenFile(fullpath); err != nil {
		return
	}
	defer j.Close()
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
