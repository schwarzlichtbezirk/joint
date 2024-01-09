package joint

import (
	"errors"
	"io"
	"io/fs"
	"net/url"
	"strings"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SftpFileStat = sftp.FileStat

var (
	pwdmap = map[string]string{}
	pwdmux sync.RWMutex
)

// SftpPwd return SFTP current directory. It's used cache to avoid
// extra calls to SFTP-server to get current directory for every call.
func SftpPwd(ftpaddr string, client *sftp.Client) (pwd string, err error) {
	pwdmux.RLock()
	pwd, ok := pwdmap[ftpaddr]
	pwdmux.RUnlock()
	if ok {
		return
	}
	if pwd, err = client.Getwd(); err != nil {
		return
	}
	pwdmux.Lock()
	pwdmap[ftpaddr] = pwd
	pwdmux.Unlock()
	return
}

// SftpJoint create SSH-connection to SFTP-server, login with provided by
// given URL credentials, and gets a once current directory.
// Key is address of SFTP-service, i.e. sftp://user:pass@example.com.
type SftpJoint struct {
	conn   *ssh.Client
	client *sftp.Client
	pwd    string

	path  string // path inside of SFTP-service without PWD
	files []fs.FileInfo
	*sftp.File
	rdn int
}

func (j *SftpJoint) Make(base Joint, urladdr string) (err error) {
	var u *url.URL
	if u, err = url.Parse(urladdr); err != nil {
		return
	}
	var pass, _ = u.User.Password()
	var config = &ssh.ClientConfig{
		User: u.User.Username(),
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if j.conn, err = ssh.Dial("tcp", u.Host, config); err != nil {
		return
	}
	if j.client, err = sftp.NewClient(j.conn); err != nil {
		return
	}
	if j.pwd, err = SftpPwd(u.Host, j.client); err != nil {
		return
	}
	if u.Path != "" && u.Path != "/" { // skip empty path
		var fpath = strings.Trim(u.Path, "/")
		j.pwd = JoinPath(j.pwd, fpath)
	}
	return
}

func (j *SftpJoint) Cleanup() error {
	var err1 error
	if j.Busy() {
		err1 = j.Close()
	}
	var err2 = j.client.Close()
	var err3 = j.conn.Close()
	return errors.Join(err1, err2, err3)
}

func (j *SftpJoint) Busy() bool {
	return j.File != nil
}

// Opens new connection for any some one file with given full SFTP URL.
func (j *SftpJoint) Open(fpath string) (file fs.File, err error) {
	if j.Busy() {
		return nil, fs.ErrExist
	}
	j.path = fpath
	if j.File, err = j.client.Open(JoinPath(j.pwd, fpath)); err != nil {
		return
	}
	j.files = nil // delete previous readdir result
	j.rdn = 0     // start new sequence
	return j, nil
}

func (j *SftpJoint) Close() (err error) {
	j.path = ""
	if j.File != nil {
		err = j.File.Close()
		j.File = nil
	}
	return
}

func (j *SftpJoint) Size() int64 {
	var fi, err = j.File.Stat()
	if err != nil {
		return 0
	}
	return fi.Size()
}

func (j *SftpJoint) ReadDir(n int) (list []fs.DirEntry, err error) {
	if j.files == nil {
		if j.files, err = j.client.ReadDir(JoinPath(j.pwd, j.path)); err != nil {
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
	list = make([]fs.DirEntry, n)
	for i := 0; i < n; i++ {
		list[i] = ToDirEntry(j.files[j.rdn+i])
	}
	j.rdn += n
	return
}

func (j *SftpJoint) Stat() (fs.FileInfo, error) {
	var fi, err = j.File.Stat()
	return ToFileInfo(fi), err
}

func (j *SftpJoint) Info(fpath string) (fs.FileInfo, error) {
	var fi, err = j.client.Stat(JoinPath(j.pwd, fpath))
	return ToFileInfo(fi), err
}
