package joint

import (
	"io"
	"io/fs"
	"strings"

	"github.com/studio-b12/gowebdav"
)

type DavFileInfo = gowebdav.File

// DavPath is map of WebDAV servises root paths by services URLs.
var DavPath = map[string]string{}

func GetDavPath(davurl string) (dpath, fpath string, ok bool) {
	defer func() {
		if ok && dpath != davurl+"/" {
			fpath = davurl[len(dpath):]
		}
	}()
	var addr, route = SplitUrl(davurl)
	if dpath, ok = DavPath[addr]; ok {
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
			var jc = GetJointCache(dpath, NewDavJoint)
			jc.Put(JointWrap{jc, &DavJoint{
				client: client,
			}})
			DavPath[addr] = dpath
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

	path string // truncated file path from full URL
	io.ReadCloser
	pos int64
	end int64
}

func NewDavJoint() Joint {
	return &DavJoint{}
}

func (j *DavJoint) Make(urladdr string) (err error) {
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
	j.path = fpath
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
	var files []fs.FileInfo
	if files, err = j.client.ReadDir(j.path); err != nil {
		return
	}
	ret = make([]fs.DirEntry, 0, len(files))
	for i, fi := range files {
		if i == n {
			break
		}
		ret = append(ret, fs.FileInfoToDirEntry(fi))
	}
	return
}

func (j *DavJoint) Info(fpath string) (fi fs.FileInfo, err error) {
	return j.client.Stat(fpath)
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

// The End.
