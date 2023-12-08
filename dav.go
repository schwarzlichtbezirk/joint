package joint

import (
	"io"
	"io/fs"
	"strings"
	"sync"

	"github.com/studio-b12/gowebdav"
)

type DavFileInfo = gowebdav.File

// davpath is global map of WebDAV servises root paths by services URLs.
// External links are external in all cases, so this object is singleton.
var davpath = map[string]string{}
var davmux sync.RWMutex

// GetDavPath searches true path to WebDAV resource
// by checking step-by-step elements of the path.
func GetDavPath(davurl string) (dpath, fpath string, ok bool) {
	defer func() {
		if ok && dpath != davurl+"/" {
			fpath = davurl[len(dpath):]
		}
	}()
	var addr, route = SplitUrl(davurl)
	davmux.RLock()
	dpath, ok = davpath[addr]
	davmux.RUnlock()
	if ok {
		return
	}

	dpath = addr
	var chunks = strings.Split("/"+route, "/")
	if chunks[len(chunks)-1] == "" {
		chunks = chunks[:len(chunks)-1]
	}
	for _, chunk := range chunks {
		dpath += chunk + "/"
		var client = gowebdav.NewClient(dpath, "", "")
		if fi, err := client.Stat(""); err == nil && fi.IsDir() {
			davmux.Lock()
			davpath[addr] = dpath
			davmux.Unlock()
			ok = true
			return
		}
	}
	return
}

// DavJoint keeps gowebdav.Client object.
// Key is URL to service, address + service route,
// i.e. https://user:pass@example.com/webdav/.
type DavJoint struct {
	client *gowebdav.Client

	path  string // truncated file path from full URL
	files []fs.FileInfo
	io.ReadCloser
	pos int64
	end int64
	rdn int
}

func (j *DavJoint) Make(base Joint, urladdr string) (err error) {
	j.client = gowebdav.NewClient(urladdr, "", "") // user & password gets from URL
	err = j.client.Connect()
	return
}

func (j *DavJoint) Cleanup() error {
	var err1 error
	if j.Busy() {
		err1 = j.Close()
	}
	j.client = nil
	return err1
}

func (j *DavJoint) Busy() bool {
	return j.path != ""
}

func (j *DavJoint) Open(fpath string) (file fs.File, err error) {
	if j.Busy() {
		return nil, fs.ErrExist
	}
	j.path = fpath
	j.files = nil // delete previous readdir result
	j.rdn = 0     // start new sequence
	return j, nil
}

func (j *DavJoint) Close() (err error) {
	j.path = ""
	if j.ReadCloser != nil {
		err = j.ReadCloser.Close()
		j.ReadCloser = nil
	}
	j.pos = 0
	j.end = 0
	return
}

func (j *DavJoint) ReadDir(n int) (ret []fs.DirEntry, err error) {
	if j.files == nil {
		if j.files, err = j.client.ReadDir(j.path); err != nil {
			return
		}
	}

	if n < 0 {
		n = len(j.files) - j.rdn
	} else if n > len(j.files)-j.rdn {
		n = len(j.files) - j.rdn
		err = io.EOF
	}
	if n <= 0 { // on case all files readed or some deleted
		return
	}
	ret = make([]fs.DirEntry, n)
	for i := 0; i < n; i++ {
		ret[i] = fs.FileInfoToDirEntry(j.files[j.rdn+i])
	}
	j.rdn += n
	return
}

func (j *DavJoint) Read(b []byte) (n int, err error) {
	if j.ReadCloser == nil {
		if j.ReadCloser, err = j.client.ReadStreamRange(j.path, j.pos, 0); err != nil {
			return
		}
	}
	n, err = j.ReadCloser.Read(b)
	j.pos += int64(n)
	return
}

func (j *DavJoint) Seek(offset int64, whence int) (abs int64, err error) {
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = j.pos + offset
	case io.SeekEnd:
		if j.end == 0 {
			var fi fs.FileInfo
			if fi, err = j.client.Stat(j.path); err != nil {
				return
			}
			j.end = fi.Size()
		}
		abs = j.end + offset
	default:
		err = ErrFtpWhence
		return
	}
	if abs < 0 {
		err = ErrFtpNegPos
		return
	}
	if abs != j.pos && j.ReadCloser != nil {
		j.ReadCloser.Close()
		j.ReadCloser = nil
	}
	j.pos = abs
	return
}

func (j *DavJoint) ReadAt(b []byte, off int64) (n int, err error) {
	if off < 0 {
		err = ErrFtpNegPos
		return
	}
	if off != j.pos && j.ReadCloser != nil {
		j.ReadCloser.Close()
		j.ReadCloser = nil
	}
	j.pos = off
	return j.Read(b)
}

func (j *DavJoint) Stat() (fi fs.FileInfo, err error) {
	fi, err = j.client.Stat(j.path)
	return
}

func (j *DavJoint) Info(fpath string) (fi fs.FileInfo, err error) {
	return j.client.Stat(fpath)
}

// The End.
