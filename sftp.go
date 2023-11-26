package joint

import (
	"errors"
	"io/fs"
	"net/url"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SftpFileStat = sftp.FileStat

// SftpPwd return SFTP current directory. It's used cache to avoid
// extra calls to SFTP-server to get current directory for every call.
func SftpPwd(ftpaddr string, client *sftp.Client) (pwd string) {
	pwdmux.RLock()
	pwd, ok := pwdmap[ftpaddr]
	pwdmux.RUnlock()
	if !ok {
		var err error
		if pwd, err = client.Getwd(); err == nil {
			pwdmux.Lock()
			pwdmap[ftpaddr] = pwd
			pwdmux.Unlock()
		}
	}
	return
}

// SftpJoint create SSH-connection to SFTP-server, login with provided by
// given URL credentials, and gets a once current directory.
// Key is address of SFTP-service, i.e. sftp://user:pass@example.com.
type SftpJoint struct {
	conn   *ssh.Client
	client *sftp.Client
	pwd    string

	path string // path inside of SFTP-service without PWD
	*sftp.File
}

func NewSftpJoint() Joint {
	return &SftpJoint{}
}

func (j *SftpJoint) Make(urladdr string) (err error) {
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
	j.pwd = SftpPwd(u.Host, j.client)
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
	j.path = fpath
	if j.File, err = j.client.Open(JoinFast(j.pwd, fpath)); err != nil {
		return
	}
	file = j
	return
}

func (j *SftpJoint) Close() (err error) {
	err = j.File.Close()
	j.path = ""
	j.File = nil
	return
}

func (j *SftpJoint) ReadDir(n int) (ret []fs.DirEntry, err error) {
	var files []fs.FileInfo
	if files, err = j.client.ReadDir(JoinFast(j.pwd, j.path)); err != nil {
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

func (j *SftpJoint) Info(fpath string) (fs.FileInfo, error) {
	return j.client.Stat(JoinFast(j.pwd, fpath))
}

// The End.
