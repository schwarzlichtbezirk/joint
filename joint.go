package joint

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// RFile combines fs.File interface and io.Seeker interface.
type RFile interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	fs.File
}

// FileInfo inherits fs.FileInfo and provide IsDir that returns
// true for ISO-disk files. Has IsRealDir that provide value
// from inherited fs.FileInfo.
type FileInfo interface {
	fs.FileInfo
	IsRealDir() bool // returns real file state representation
}

// Joint describes interface with joint to some file system provider.
type Joint interface {
	Make(Joint, string) error // establish connection to file system provider
	Cleanup() error           // close connection to file system provider
	Busy() bool               // file is opened
	fs.FS                     // open file with local file path
	io.Closer                 // close local file
	Size() (int64, error)     // helps to make io.SectionReader
	fs.ReadDirFile            // read directory pointed by local file path
	RFile
}

// MakeJoint creates joint with all subsequent chain of joints.
// Please note that folders with .iso extension and non ISO-images
// with .iso extension will cause an error.
func MakeJoint(fullpath string) (j Joint, err error) {
	var addr, fpath, is = SplitUrl(fullpath)
	if HasFoldPrefix(fullpath, "ftp://") {
		j = &FtpJoint{}
		if err = j.Make(nil, addr); err != nil {
			return
		}
	} else if HasFoldPrefix(fullpath, "sftp://") {
		j = &SftpJoint{}
		if err = j.Make(nil, addr); err != nil {
			return
		}
	} else if HasFoldPrefix(fullpath, "http://") || HasFoldPrefix(fullpath, "https://") {
		var root, ok = FindDavRoot(addr, fpath)
		if !ok {
			err = fs.ErrNotExist
			return
		}
		fpath = fpath[len(root)-1:]
		j = &DavJoint{}
		if err = j.Make(nil, addr+root); err != nil {
			return
		}
	} else if !is {
		j = &SysJoint{dir: addr}
	} else {
		err = fs.ErrNotExist
		return
	}

	var jpos = 0
	for {
		var p1 = strings.Index(fpath[jpos:], ".iso/")
		var p2 = strings.Index(fpath[jpos:], ".ISO/")
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
		var key = fpath[:p+4]
		var jiso = &IsoJoint{}
		if err = jiso.Make(j, key); err != nil {
			return
		}
		j, jpos = jiso, p+5
	}
	if IsTypeIso(fpath[jpos:]) {
		var key = fpath[jpos:]
		var jiso = &IsoJoint{}
		if err = jiso.Make(j, key); err != nil {
			return
		}
		j = jiso
	}
	return
}

// JointFileInfo have additional IsRealDir, which points real file representation.
type JointFileInfo interface {
	fs.FileInfo
	IsRealDir() bool
}

// fileinfo is wrapper around inherited fs.FileInfo to provide derived IsDir.
type fileinfo struct {
	fs.FileInfo
}

// Hack for gowebdav.File on case if it fill `path` field and
// does not fill `name` field.
func (fi fileinfo) Name() (name string) {
	type pather interface {
		Path() string
	}
	if name = fi.FileInfo.Name(); name == "" {
		if pi, ok := fi.FileInfo.(pather); ok {
			if p := pi.Path(); p != "" {
				name = path.Base(p)
			}
		}
	}
	return
}

func (fi fileinfo) Mode() fs.FileMode {
	var mode = fi.FileInfo.Mode()
	if mode.IsRegular() && IsTypeIso(fi.Name()) {
		mode |= fs.ModeDir
	}
	return mode
}

func (fi fileinfo) IsDir() bool {
	return fi.FileInfo.IsDir() || IsTypeIso(fi.Name())
}

func (fi fileinfo) IsRealDir() bool {
	return fi.FileInfo.IsDir()
}

func (fi fileinfo) Type() fs.FileMode {
	return fi.FileInfo.Mode().Type()
}

// Info provided for fs.DirEntry compatibility and returns object itself.
func (fi fileinfo) Info() (fs.FileInfo, error) {
	return fi, nil
}

func (fi fileinfo) String() string {
	return fs.FormatDirEntry(fi)
}

// ToFileInfo converts base fs.FileInfo to FileInfo that compatible both
// with fs.FileInfo and with fs.DirEntry interface and have derived IsDir.
func ToFileInfo(fi fs.FileInfo) FileInfo {
	if fi == nil {
		return nil
	}
	return fileinfo{fi}
}

// ToDirEntry returns FileInfo that compatible with fs.DirEntry interface,
// and have derived IsDir.
func ToDirEntry(fi fs.FileInfo) fs.DirEntry {
	if fi == nil {
		return nil
	}
	return fileinfo{fi}
}

// JointWrap helps to return joint into cache after Close-call.
// It has pointer to JointCache that it binded to.
type JointWrap struct {
	jc *JointCache
	Joint
}

// GetCache returns binded cache.
func (jw JointWrap) GetCache() *JointCache {
	return jw.jc
}

// Close calls inherited Close-function and puts joint into binded cache.
func (jw JointWrap) Close() error {
	var err = jw.Joint.Close()
	if jw.jc != nil {
		jw.jc.Put(jw)
	}
	return err
}

type Config struct {
	// Timeout to establish connection to FTP-server.
	DialTimeout time.Duration `json:"dial-timeout" yaml:"dial-timeout" xml:"dial-timeout"`
	// Expiration duration to keep opened iso-disk structures in cache from last access to it.
	DiskCacheExpire time.Duration `json:"disk-cache-expire" yaml:"disk-cache-expire" xml:"disk-cache-expire"`
}

// Cfg is singleton with timeouts settings for all joints.
var Cfg = Config{
	DialTimeout:     5 * time.Second,
	DiskCacheExpire: 2 * time.Minute,
}

// JointCache implements cache with opened joints to some file system resource.
type JointCache struct {
	key    string
	cache  []Joint
	expire []*time.Timer
	mux    sync.Mutex
}

func NewJointCache(key string) *JointCache {
	return &JointCache{
		key: key,
	}
}

// Key is the base address or path for cache file system.
func (jc *JointCache) Key() string {
	return jc.key
}

// Open implements fs.FS interface,
// and returns file that can be casted to joint wrapper.
// Note that internal ISO-files are considered as directories and it should
// be provided another JointCache to work with their file system.
// Use JointPool on this case.
func (jc *JointCache) Open(fpath string) (f fs.File, err error) {
	var jw JointWrap
	if jw, err = jc.Get(); err != nil {
		return
	}
	if _, err = jw.Open(fpath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			jc.Put(jw) // reuse joint
		} else if !errors.Is(err, fs.ErrExist) { // not already opened
			jw.Cleanup() // drop the joint
		}
		return
	}
	f = jw // put joint back to cache after Close
	return
}

// Stat implements fs.StatFS interface.
func (jc *JointCache) Stat(fpath string) (fi fs.FileInfo, err error) {
	var f fs.File
	if f, err = jc.Open(fpath); err != nil {
		return
	}
	defer f.Close()

	return f.Stat()
}

// ReadDir implements fs.ReadDirFS interface.
func (jc *JointCache) ReadDir(fpath string) (list []fs.DirEntry, err error) {
	var f fs.File
	if f, err = jc.Open(fpath); err != nil {
		return
	}
	defer f.Close()

	list, err = f.(Joint).ReadDir(-1)
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return
}

// Count is number of free joints in cache for one key path.
func (jc *JointCache) Count() int {
	jc.mux.Lock()
	defer jc.mux.Unlock()
	return len(jc.cache)
}

// Close performs close-call to all cached disk joints.
func (jc *JointCache) Close() (err error) {
	jc.mux.Lock()
	defer jc.mux.Unlock()

	for _, t := range jc.expire {
		t.Stop()
	}
	jc.expire = nil

	var errs = make([]error, len(jc.cache))
	for i, j := range jc.cache {
		errs[i] = j.Cleanup()
	}
	jc.cache = nil
	return errors.Join(errs...)
}

// Checks whether it is contained joint in cache.
func (jc *JointCache) Has(j Joint) bool {
	if jw, ok := j.(JointWrap); ok {
		j = jw.Joint // strip wrapper to avoid overlapping
	}

	jc.mux.Lock()
	defer jc.mux.Unlock()

	for _, f := range jc.cache {
		if f == j {
			return true
		}
	}
	return false
}

// Pop retrieves cached disk joint, and returns ok if it has.
func (jc *JointCache) Pop() (jw JointWrap, ok bool) {
	jc.mux.Lock()
	defer jc.mux.Unlock()

	var l = len(jc.cache)
	if l > 0 {
		jc.expire[0].Stop()
		copy(jc.expire, jc.expire[1:])
		jc.expire = jc.expire[:l-1]
		jw.Joint = jc.cache[0]
		jw.jc = jc // ensure that jc is owned while jw is outside of cache
		copy(jc.cache, jc.cache[1:])
		jc.cache = jc.cache[:l-1]
		ok = true
	}
	return
}

// Get retrieves cached disk joint, or makes new one.
func (jc *JointCache) Get() (jw JointWrap, err error) {
	jw, ok := jc.Pop()
	if !ok {
		if jw.Joint, err = MakeJoint(jc.key); err != nil {
			return
		}
		jw.jc = jc // ensure that jc is owned while jw is outside of cache
	}
	return
}

// Put disk joint to cache.
func (jc *JointCache) Put(j Joint) {
	if jw, ok := j.(JointWrap); ok {
		j = jw.Joint // strip wrapper to avoid overlapping
	}

	jc.mux.Lock()
	defer jc.mux.Unlock()

	for _, f := range jc.cache { // ensure that joint does not present
		if f == j {
			return
		}
	}

	jc.cache = append(jc.cache, j)
	jc.expire = append(jc.expire, time.AfterFunc(Cfg.DiskCacheExpire, func() {
		if jw, ok := jc.Pop(); ok {
			jw.Joint.Cleanup()
		}
	}))
}

// Eject joint from the cache.
func (jc *JointCache) Eject(j Joint) bool {
	if jw, ok := j.(JointWrap); ok {
		j = jw.Joint // strip wrapper to avoid overlapping
	}

	jc.mux.Lock()
	defer jc.mux.Unlock()

	for i, f := range jc.cache {
		if f == j {
			jc.expire[i].Stop()
			jc.expire = append(jc.expire[:i], jc.expire[i+1:]...)
			jc.cache = append(jc.cache[:i], jc.cache[i+1:]...)
			return true
		}
	}
	return false
}
