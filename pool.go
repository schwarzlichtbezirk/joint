package joint

import (
	"errors"
	"io/fs"
	"strings"
	"sync"
)

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

// IsTypeIso checks that endpoint-file in given path has ISO-extension.
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
	for {
		var p1 = strings.Index(localpath[jpos:], ".iso/")
		var p2 = strings.Index(localpath[jpos:], ".ISO/")
		if p1 == p2 { // p1 == -1 && p2 == -1
			break
		}
		var p int
		if p1 == -1 {
			p = p2
		} else if p2 == -1 {
			p = p1
		} else {
			p = min(p1, p2)
		}
		var key = localpath[:p+4]
		var jiso = &IsoJoint{}
		if err = jiso.Make(j, key); err != nil {
			return
		}
		j, jpos = jiso, p+5
	}
	if IsTypeIso(localpath[jpos:]) {
		var key = localpath[jpos:]
		var jiso = &IsoJoint{}
		if err = jiso.Make(j, key); err != nil {
			return
		}
		j = jiso
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

// JointPool is map with joint caches.
// Each key in map is path to file system resource,
// value - cached for this resource list of joints.
type JointPool struct {
	jpmap map[string]*JointCache
	jpmux sync.RWMutex
}

func NewJointPool() *JointPool {
	return &JointPool{
		jpmap: map[string]*JointCache{},
	}
}

// Keys returns list of all joints key paths.
func (jp *JointPool) Keys() []string {
	jp.jpmux.RLock()
	defer jp.jpmux.RUnlock()

	var list = make([]string, len(jp.jpmap))
	var i = 0
	for key := range jp.jpmap {
		list[i] = key
		i++
	}
	return list
}

// GetCache returns cache from pool for given key path, or creates new one.
func (jp *JointPool) GetCache(key string) (jc *JointCache) {
	jp.jpmux.Lock()
	defer jp.jpmux.Unlock()

	var ok bool
	if jc, ok = jp.jpmap[key]; !ok {
		jc = NewJointCache(key)
		jp.jpmap[key] = jc
	}
	return
}

// Close resets all caches.
func (jp *JointPool) Close() error {
	jp.jpmux.Lock()
	defer jp.jpmux.Unlock()
	var errs = make([]error, 0, len(jp.jpmap))
	for _, jc := range jp.jpmap {
		if err := jc.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Clear is same as Close, and removes all entries in the map.
func (jp *JointPool) Clear() error {
	var err = jp.Close()
	clear(jp.jpmap)
	return err
}

// GetJoint returns joint for given key.
func (jp *JointPool) GetJoint(key string) (j Joint, err error) {
	return jp.GetCache(key).Get()
}

// Open opens file with given full path to this file,
// that can be located inside of nested ISO-images and/or
// on FTP, SFTP, WebDAV servers.
// Open implements fs.FS interface,
// and returns file that can be casted to joint wrapper.
func (jp *JointPool) Open(fullpath string) (f fs.File, err error) {
	var key, fpath = SplitKey(fullpath)
	if key == "" {
		var j = &SysJoint{}
		return j.Open(fpath)
	}
	var jc = jp.GetCache(key)
	if f, err = jc.Open(fpath); err != nil {
		return
	}
	return
}

// Stat returns fs.FileInfo of file pointed by given full path.
// Stat implements fs.StatFS interface.
func (jp *JointPool) Stat(fullpath string) (fi fs.FileInfo, err error) {
	var f fs.File
	if f, err = jp.Open(fullpath); err != nil {
		return
	}
	defer f.Close()
	return f.Stat()
}

// ReadDir returns directory files fs.DirEntry list pointed by given full path.
// ReadDir implements ReadDirFS interface.
func (jp *JointPool) ReadDir(fullpath string) (ret []fs.DirEntry, err error) {
	var f fs.File
	if f, err = jp.Open(fullpath); err != nil {
		return
	}
	defer f.Close()
	var j = f.(Joint)
	return j.ReadDir(-1)
}

// Sub returns new file subsystem with given absolute root directory.
// Sub implements fs.SubFS interface,
// and returns object that can be casted to *SubPool.
func (jp *JointPool) Sub(dir string) (fs.FS, error) {
	var fi, err = jp.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() && IsTypeIso(dir) {
		return nil, fs.ErrNotExist
	}
	return &SubPool{
		JointPool: jp,
		dir:       dir,
	}, nil
}

// SubPool releases io/fs interfaces in the way that
// can be used for http-handlers. It has pointer to pool,
// so several derived file subsystems can share same pool
// of caches.
type SubPool struct {
	*JointPool
	dir string
}

// NewSubPool creates new SubPool objects with given pool of caches
// and given absolute root directory.
func NewSubPool(jp *JointPool, dir string) *SubPool {
	return &SubPool{jp, dir}
}

// Dir returns root directory of this file subsystem.
func (sp *SubPool) Dir() string {
	return sp.dir
}

// Open implements fs.FS interface,
// and returns file that can be casted to joint wrapper.
func (sp *SubPool) Open(fullpath string) (f fs.File, err error) {
	fullpath = JoinFast(sp.dir, fullpath)
	return sp.JointPool.Open(fullpath)
}

// Stat implements fs.StatFS interface.
func (sp *SubPool) Stat(fullpath string) (fi fs.FileInfo, err error) {
	fullpath = JoinFast(sp.dir, fullpath)
	return sp.JointPool.Stat(fullpath)
}

// ReadDir implements ReadDirFS interface.
func (sp *SubPool) ReadDir(fullpath string) (ret []fs.DirEntry, err error) {
	fullpath = JoinFast(sp.dir, fullpath)
	return sp.JointPool.ReadDir(fullpath)
}

// Sub returns new file subsystem with given relative root directory.
// Sub implements fs.SubFS interface,
// and returns object that can be casted to *SubPool.
func (sp *SubPool) Sub(dir string) (fs.FS, error) {
	dir = JoinFast(sp.dir, dir)
	var fi, err = sp.JointPool.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() && IsTypeIso(dir) {
		return nil, fs.ErrNotExist
	}
	return &SubPool{
		JointPool: sp.JointPool,
		dir:       dir,
	}, nil
}

// The End.
