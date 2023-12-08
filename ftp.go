package joint

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

var (
	ErrFtpWhence = errors.New("invalid whence at FTP seeker")
	ErrFtpNegPos = errors.New("negative position at FTP seeker")
)

// FtpEscapeBrackets escapes square brackets at FTP-path.
// FTP-server does not recognize path with square brackets
// as is to get a list of files, so such path should be escaped.
func FtpEscapeBrackets(s string) string {
	var n = 0
	for _, c := range s {
		if c == '[' || c == ']' {
			n++
		}
	}
	if n == 0 {
		return s
	}
	var esc = make([]rune, 0, len(s)+n*2)
	for _, c := range s {
		if c == '[' {
			esc = append(esc, '[', '[', ']')
		} else if c == ']' {
			esc = append(esc, '[', ']', ']')
		} else {
			esc = append(esc, c)
		}
	}
	return string(esc)
}

// FtpJoint create connection to FTP-server, login with provided by
// given URL credentials, and gets a once current directory.
// Key is address of FTP-service, i.e. ftp://user:pass@example.com.
type FtpJoint struct {
	conn *ftp.ServerConn

	path    string // path inside of FTP-service
	entries []*ftp.Entry
	io.ReadCloser
	pos int64
	end int64
	rdn int
}

func (j *FtpJoint) Make(base Joint, urladdr string) (err error) {
	var u *url.URL
	if u, err = url.Parse(urladdr); err != nil {
		return
	}
	if j.conn, err = ftp.Dial(u.Host, ftp.DialWithTimeout(Cfg.DialTimeout)); err != nil {
		return
	}
	var pass, _ = u.User.Password()
	if err = j.conn.Login(u.User.Username(), pass); err != nil {
		return
	}
	if u.Path != "" && u.Path != "/" { // skip empty path
		var fpath = strings.Trim(u.Path, "/")
		if err = j.conn.ChangeDir(fpath); err != nil {
			return
		}
	}
	return
}

func (j *FtpJoint) Cleanup() error {
	var err1 error
	if j.Busy() {
		err1 = j.Close()
	}
	var err2 = j.conn.Quit()
	return errors.Join(err1, err2)
}

func (j *FtpJoint) Busy() bool {
	return j.path != ""
}

func (j *FtpJoint) Open(fpath string) (file fs.File, err error) {
	if j.Busy() {
		return nil, fs.ErrExist
	}
	j.path = fpath
	j.entries = nil // delete previous readdir result
	j.rdn = 0       // start new sequence
	return j, nil
}

func (j *FtpJoint) Close() (err error) {
	j.path = ""
	if j.ReadCloser != nil {
		err = j.ReadCloser.Close()
		j.ReadCloser = nil
	}
	j.pos = 0
	j.end = 0
	return
}

func (j *FtpJoint) ReadDir(n int) (ret []fs.DirEntry, err error) {
	if j.entries == nil {
		var fpath = FtpEscapeBrackets(j.path)
		if j.entries, err = j.conn.List(fpath); err != nil {
			return
		}
		if len(j.entries) < 2 {
			return nil, io.ErrUnexpectedEOF
		}
		j.entries = j.entries[2:] // skip "." and ".." directories
	}

	if n < 0 {
		n = len(j.entries) - j.rdn
	} else if n > len(j.entries)-j.rdn {
		n = len(j.entries) - j.rdn
		err = io.EOF
	}
	if n <= 0 { // on case all files readed or some deleted
		return
	}
	ret = make([]fs.DirEntry, n)
	for i := 0; i < n; i++ {
		ret[i] = fs.FileInfoToDirEntry(FtpFileInfo{j.entries[j.rdn+i]})
	}
	j.rdn += n
	return
}

func (j *FtpJoint) Stat() (fi fs.FileInfo, err error) {
	var ent *ftp.Entry
	if ent, err = j.conn.GetEntry(j.path); err != nil {
		return
	}
	fi = FtpFileInfo{ent}
	return
}

func (j *FtpJoint) Info(fpath string) (fi fs.FileInfo, err error) {
	var ent *ftp.Entry
	if ent, err = j.conn.GetEntry(fpath); err != nil {
		return
	}
	fi = FtpFileInfo{ent}
	return
}

func (j *FtpJoint) Size() int64 {
	if j.end == 0 {
		j.end, _ = j.conn.FileSize(j.path)
	}
	return j.end
}

func (j *FtpJoint) Read(b []byte) (n int, err error) {
	if j.ReadCloser == nil {
		if j.ReadCloser, err = j.conn.RetrFrom(j.path, uint64(j.pos)); err != nil {
			return
		}
	}
	n, err = j.ReadCloser.Read(b)
	j.pos += int64(n)
	return
}

func (j *FtpJoint) Write(p []byte) (n int, err error) {
	var buf = bytes.NewReader(p)
	err = j.conn.StorFrom(j.path, buf, uint64(j.pos))
	var n64, _ = buf.Seek(0, io.SeekCurrent)
	j.pos += n64
	n = int(n64)
	return
}

func (j *FtpJoint) Seek(offset int64, whence int) (abs int64, err error) {
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = j.pos + offset
	case io.SeekEnd:
		if j.end == 0 {
			if j.end, err = j.conn.FileSize(j.path); err != nil {
				return
			}
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

func (j *FtpJoint) ReadAt(b []byte, off int64) (n int, err error) {
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

func (j *FtpJoint) CurrentDir() (wd string, err error) {
	return j.conn.CurrentDir()
}

func (j *FtpJoint) ChangeDir(wd string) (err error) {
	return j.conn.ChangeDir(wd)
}

func (j *FtpJoint) ChangeDirToParent() (err error) {
	return j.conn.ChangeDirToParent()
}

// FtpFileInfo encapsulates ftp.Entry structure and provides fs.FileInfo implementation.
type FtpFileInfo struct {
	*ftp.Entry
}

// fs.FileInfo implementation.
func (fi FtpFileInfo) Name() string {
	return path.Base(fi.Entry.Name)
}

// fs.FileInfo implementation.
func (fi FtpFileInfo) Size() int64 {
	return int64(fi.Entry.Size)
}

// fs.FileInfo implementation.
func (fi FtpFileInfo) Mode() fs.FileMode {
	switch fi.Type {
	case ftp.EntryTypeFile:
		return 0444
	case ftp.EntryTypeFolder:
		return fs.ModeDir
	case ftp.EntryTypeLink:
		return fs.ModeSymlink
	}
	return 0
}

// fs.FileInfo implementation.
func (fi FtpFileInfo) ModTime() time.Time {
	return fi.Entry.Time
}

// fs.FileInfo implementation.
func (fi FtpFileInfo) IsDir() bool {
	return fi.Entry.Type == ftp.EntryTypeFolder
}

// fs.FileInfo implementation. Returns structure pointer itself.
func (fi FtpFileInfo) Sys() interface{} {
	return fi
}

// The End.
