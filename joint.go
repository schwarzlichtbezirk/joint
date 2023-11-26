package joint

import (
	"errors"
	"io"
	"io/fs"
	"sync"
	"time"
)

// Joint describes interface with joint to some file system provider.
type Joint interface {
	Make(string) error // establish connection to file system provider
	Cleanup() error    // close connection to file system provider
	Busy() bool        // file is opened
	fs.FS              // open file with local file path
	io.Closer          // close local file
	fs.ReadDirFile     // read directory pointed by local file path
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
	return jw.Open(fpath)
}

// Stat implements fs.StatFS interface.
func (jc *JointCache) Stat(fpath string) (fi fs.FileInfo, err error) {
	var jw JointWrap
	if jw, err = jc.Get(); err != nil {
		return
	}

	if fpath == "." {
		fpath = ""
	}
	if _, err = jw.Open(fpath); err != nil {
		return
	}
	defer jw.Close()

	return jw.Stat()
}

// ReadDir implements fs.ReadDirFS interface.
func (jc *JointCache) ReadDir(fpath string) (des []fs.DirEntry, err error) {
	var jw JointWrap
	if jw, err = jc.Get(); err != nil {
		return
	}

	if fpath == "." {
		fpath = ""
	}
	if _, err = jw.Open(fpath); err != nil {
		return
	}
	defer jw.Close()

	return jw.ReadDir(-1)
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

// Pop retrieves cached disk joint, and returns ok if it has.
func (jc *JointCache) Pop() (jw JointWrap, ok bool) {
	jc.mux.Lock()
	defer jc.mux.Unlock()
	var l = len(jc.cache)
	if l > 0 {
		jc.expire[0].Stop()
		jc.expire = jc.expire[1:]
		jw = jc.cache[0]
		jc.cache = jc.cache[1:]
		ok = true
	}
	return
}

// Get retrieves cached disk joint, or makes new one.
func (jc *JointCache) Get() (jw JointWrap, err error) {
	jw, ok := jc.Pop()
	if !ok {
		jw = JointWrap{jc, jc.master()}
		if err = jw.Make(jc.key); err != nil {
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

// The End.
