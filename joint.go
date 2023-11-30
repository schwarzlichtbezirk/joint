package joint

import (
	"errors"
	"io"
	"io/fs"
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

// Joint describes interface with joint to some file system provider.
type Joint interface {
	Make(Joint, string) error // establish connection to file system provider
	Cleanup() error           // close connection to file system provider
	Busy() bool               // file is opened
	fs.FS                     // open file with local file path
	io.Closer                 // close local file
	fs.ReadDirFile            // read directory pointed by local file path
	RFile
}

type JointWrap struct {
	jc *JointCache
	Joint
}

func (jw JointWrap) Close() error {
	var err = jw.Joint.Close()
	if jw.jc != nil {
		jw.jc.Put(jw)
	}
	return err
}

// Eject joint from the cache.
func (jw JointWrap) Eject() bool {
	if jw.jc == nil {
		return false
	}
	jw.jc.mux.Lock()
	defer jw.jc.mux.Unlock()
	for i, jf := range jw.jc.cache {
		if jf.Joint == jw.Joint {
			jw.jc.expire[i].Stop()
			jw.jc.expire = append(jw.jc.expire[:i], jw.jc.expire[i+1:]...)
			jw.jc.cache = append(jw.jc.cache[:i], jw.jc.cache[i+1:]...)
			return true
		}
	}
	return false
}

type Config struct {
	// Timeout to establish connection to FTP-server.
	DialTimeout time.Duration `json:"dial-timeout" yaml:"dial-timeout" xml:"dial-timeout"`
	// Expiration duration to keep opened iso-disk structures in cache from last access to it.
	DiskCacheExpire time.Duration `json:"disk-cache-expire" yaml:"disk-cache-expire" xml:"disk-cache-expire"`
}

var Cfg = Config{
	DialTimeout:     5 * time.Second,
	DiskCacheExpire: 2 * time.Minute,
}

// JointCache implements cache with opened joints to some file system resource.
type JointCache struct {
	key    string
	master func() Joint
	cache  []JointWrap
	expire []*time.Timer
	mux    sync.Mutex
}

func NewJointCache(key string, master func() Joint) *JointCache {
	return &JointCache{
		key:    key,
		master: master,
	}
}

// Open implements fs.FS interface.
func (jc *JointCache) Open(fpath string) (f fs.File, err error) {
	var jw JointWrap
	if jw, err = jc.Get(); err != nil {
		return
	}
	if fpath == "." {
		fpath = ""
	}
	if _, err = jw.Open(fpath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			jc.Put(jw) // reuse joint
		} else {
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
func (jc *JointCache) ReadDir(fpath string) (des []fs.DirEntry, err error) {
	var f fs.File
	if f, err = jc.Open(fpath); err != nil {
		return
	}
	defer f.Close()

	return f.(Joint).ReadDir(-1)
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
	for i, jw := range jc.cache {
		errs[i] = jw.Joint.Cleanup()
	}
	jc.cache = nil
	return errors.Join(errs...)
}

// Checks whether it is contained joint in cache.
func (jc *JointCache) Has(j Joint) bool {
	if jw, ok := j.(JointWrap); ok {
		j = jw.Joint
	}
	jc.mux.Lock()
	defer jc.mux.Unlock()
	for _, jw := range jc.cache {
		if jw.Joint == j {
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
		jw = jc.cache[0]
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
		jw = JointWrap{jc, jc.master()}
		if err = jw.Make(nil, jc.key); err != nil {
			return
		}
	}
	return
}

// Put disk joint to cache.
func (jc *JointCache) Put(j Joint) {
	var jw, ok = j.(JointWrap) // get wrapper if it has
	if !ok {
		jw.Joint = j
	}
	jw.jc = jc // ensure that jc is owned in all cases
	jc.mux.Lock()
	defer jc.mux.Unlock()
	jc.cache = append(jc.cache, jw)
	jc.expire = append(jc.expire, time.AfterFunc(Cfg.DiskCacheExpire, func() {
		if j, ok := jc.Pop(); ok {
			j.Cleanup()
		}
	}))
}

// jp is map with joint caches.
// Each key is path to file system resource,
// value - cached for this resource list of joints.
var jp = map[string]*JointCache{}
var jpmux sync.RWMutex

// GetJointCache returns cache from pool for given key path, or creates new one.
func GetJointCache(key string, master func() Joint) (jc *JointCache) {
	jpmux.Lock()
	defer jpmux.Unlock()

	var ok bool
	if jc, ok = jp[key]; !ok {
		jc = NewJointCache(key, master)
		jp[key] = jc
	}
	return
}

// JointPool returns copy of joints pool.
func JointPool() map[string]*JointCache {
	jpmux.RLock()
	defer jpmux.RUnlock()

	var m = make(map[string]*JointCache, len(jp))
	for key, jc := range jp {
		m[key] = jc
	}
	return m
}

// JointPoolKeys returns list of all joints key paths.
func JointPoolKeys() []string {
	jpmux.RLock()
	defer jpmux.RUnlock()

	var list = make([]string, len(jp))
	var i = 0
	for key := range jp {
		list[i] = key
		i++
	}
	return list
}

// ClearJointPool resets all caches.
func ClearJointPool() {
	jpmux.Lock()
	defer jpmux.Unlock()
	for _, jc := range jp {
		jc.Close()
	}
	clear(jp)
}

// The End.
