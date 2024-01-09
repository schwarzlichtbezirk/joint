package joint

import (
	"errors"
	"io/fs"
	"sort"
	"sync"
)

// JointPool is map with joint caches.
// Each key in map is address or path to file system resource,
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
	var key, fpath, isurl = SplitKey(fullpath)
	if !isurl {
		var j = &SysJoint{dir: key}
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
func (jp *JointPool) ReadDir(fullpath string) (list []fs.DirEntry, err error) {
	var f fs.File
	if f, err = jp.Open(fullpath); err != nil {
		return
	}
	defer f.Close()

	list, err = f.(Joint).ReadDir(-1)
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return
}

// Sub returns new file subsystem with given absolute root directory.
// It's assumed that this call can be used to get access to some
// WebDAV/SFTP/FTP service.
// Sub implements fs.SubFS interface,
// and returns object that can be casted to *SubPool.
func (jp *JointPool) Sub(dir string) (fs.FS, error) {
	var fi, err = jp.Stat(dir)
	if err != nil {
		return nil, err
	}
	var jfi = fi.(FileInfo)
	if jfi.IsRealDir() && IsTypeIso(dir) {
		return nil, fs.ErrNotExist
	}
	return &SubPool{
		JointPool: jp,
		dir:       dir,
	}, nil
}

// SubPool releases io/fs interfaces in the way that
// can be used for http-handlers. It has pointer to pool
// to share same pool for several derived file subsystems.
// SubPool is developed to pass fstest.TestFS tests, and
// all its functions receives only valid FS-paths.
type SubPool struct {
	*JointPool
	dir string
}

// NewSubPool creates new SubPool objects with given pool of caches
// and given absolute root directory.
func NewSubPool(jp *JointPool, dir string) *SubPool {
	if jp == nil {
		jp = NewJointPool()
	}
	return &SubPool{jp, dir}
}

// Dir returns root directory of this file subsystem.
func (sp *SubPool) Dir() string {
	return sp.dir
}

// Open implements fs.FS interface,
// and returns file that can be casted to joint wrapper.
func (sp *SubPool) Open(fpath string) (f fs.File, err error) {
	if sp.dir != "" && sp.dir != "." && !fs.ValidPath(fpath) {
		return nil, fs.ErrInvalid
	}
	return sp.JointPool.Open(JoinPath(sp.dir, fpath))
}

// Stat implements fs.StatFS interface.
func (sp *SubPool) Stat(fpath string) (fi fs.FileInfo, err error) {
	if sp.dir != "" && sp.dir != "." && !fs.ValidPath(fpath) {
		return nil, fs.ErrInvalid
	}
	return sp.JointPool.Stat(JoinPath(sp.dir, fpath))
}

// ReadDir implements ReadDirFS interface.
func (sp *SubPool) ReadDir(fpath string) (ret []fs.DirEntry, err error) {
	if sp.dir != "" && sp.dir != "." && !fs.ValidPath(fpath) {
		return nil, fs.ErrInvalid
	}
	return sp.JointPool.ReadDir(JoinPath(sp.dir, fpath))
}

// Sub returns new file subsystem with given relative root directory.
// Performs given directory check up.
// Sub implements fs.SubFS interface,
// and returns object that can be casted to *SubPool.
func (sp *SubPool) Sub(dir string) (fs.FS, error) {
	if sp.dir != "" && sp.dir != "." && !fs.ValidPath(dir) {
		return nil, fs.ErrInvalid
	}
	return sp.JointPool.Sub(JoinPath(sp.dir, dir))
}
