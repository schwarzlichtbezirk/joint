package joint

import (
	"io"
	"io/fs"
	"strings"

	iso "github.com/kdomanski/iso9660"
	"golang.org/x/text/encoding/charmap"
)

// IsoJoint opens file with ISO9660 disk and prepares disk-structure
// to access to nested files.
// Key is external path, to ISO9660-file disk image at local filesystem.
type IsoJoint struct {
	Base  Joint
	img   *iso.Image
	cache map[string]*iso.File

	*iso.File
	*io.SectionReader
	rdn int
}

func (j *IsoJoint) Make(base Joint, isopath string) (err error) {
	if base == nil {
		base = &SysJoint{}
	}
	if _, err = base.Open(isopath); err != nil {
		return
	}
	j.Base = base
	if j.img, err = iso.OpenImage(j.Base); err != nil {
		return
	}
	j.cache = map[string]*iso.File{}
	if j.cache[""], err = j.img.RootDir(); err != nil {
		return
	}
	return
}

func (j *IsoJoint) Cleanup() error {
	if j.Busy() {
		j.Close()
	}
	var err = j.Base.Cleanup()
	j.Base = nil
	return err
}

func (j *IsoJoint) Busy() bool {
	return j.File != nil
}

func (j *IsoJoint) Open(fpath string) (file fs.File, err error) {
	if j.Busy() {
		return nil, fs.ErrExist
	}
	if fpath == "." { // dot folder does not accepted
		fpath = ""
	}
	if j.File, err = j.OpenFile(fpath); err != nil {
		return
	}
	if fpath == "" { // open base ISO-disk to read
		j.SectionReader = io.NewSectionReader(j.Base, 0, j.Base.Size())
	} else if sr := j.File.Reader(); sr != nil {
		j.SectionReader = sr.(*io.SectionReader)
	}
	j.rdn = 0 // start new sequence
	return j, nil
}

func (j *IsoJoint) Close() error {
	j.File = nil
	j.SectionReader = nil
	return nil
}

func (j *IsoJoint) OpenFile(fpath string) (*iso.File, error) {
	if file, ok := j.cache[fpath]; ok {
		return file, nil
	}
	if !fs.ValidPath(fpath) {
		return nil, fs.ErrInvalid
	}

	var dec = charmap.Windows1251.NewDecoder()
	var curdir string
	var chunks = strings.Split(fpath, "/")
	var file = j.cache[curdir] // get root directory
	for _, chunk := range chunks {
		if !file.IsDir() {
			return nil, fs.ErrNotExist
		}
		var curpath = JoinFast(curdir, chunk)
		if f, ok := j.cache[curpath]; ok {
			file = f // the file must be unchanged otherwise
		} else {
			var list, err = file.GetChildren()
			if err != nil {
				return nil, err
			}
			var found = false
			for _, file = range list {
				var name, _ = dec.String(file.Name())
				j.cache[JoinFast(curdir, name)] = file
				if name == chunk {
					found = true
					break
				}
			}
			if !found {
				return nil, fs.ErrNotExist
			}
		}
		curdir = curpath
	}
	return file, nil
}

// Size of file. Resolve duality between File.Size() and SectionReader.Size().
func (j *IsoJoint) Size() int64 {
	return j.File.Size()
}

func (j *IsoJoint) ReadDir(n int) (list []fs.DirEntry, err error) {
	var files []*iso.File // children entries cached by previous calls
	if files, err = j.File.GetChildren(); err != nil {
		return
	}

	if n < 0 {
		n = len(files) - j.rdn
	} else if n > len(files)-j.rdn {
		n = len(files) - j.rdn
		err = io.EOF
	}
	if n <= 0 { // on case all files readed or some deleted
		return
	}
	list = make([]fs.DirEntry, n)
	for i := 0; i < n; i++ {
		list[i] = IsoFileInfo{files[j.rdn+i]}
	}
	j.rdn += n
	return
}

func (j *IsoJoint) Stat() (fs.FileInfo, error) {
	if j.File.IsDir() && j.SectionReader != nil { // base ISO-disk
		return j.Base.Stat()
	}
	return IsoFileInfo{j.File}, nil
}

func (j *IsoJoint) Info(fpath string) (fs.FileInfo, error) {
	var file, err = j.OpenFile(fpath)
	if err != nil {
		return nil, err
	}
	return IsoFileInfo{file}, nil
}

type IsoFileInfo struct {
	*iso.File
}

func (fi IsoFileInfo) Name() string {
	var dec = charmap.Windows1251.NewDecoder()
	var name, _ = dec.String(fi.File.Name())
	return name
}

func (fi IsoFileInfo) Mode() fs.FileMode {
	var mode = fi.File.Mode()
	if mode.IsRegular() && IsTypeIso(fi.File.Name()) {
		mode |= fs.ModeDir
	}
	return mode
}

func (fi IsoFileInfo) IsDir() bool {
	return fi.File.IsDir() || IsTypeIso(fi.File.Name())
}

func (fi IsoFileInfo) IsRealDir() bool {
	return fi.File.IsDir()
}

func (fi IsoFileInfo) Type() fs.FileMode {
	return fi.Mode().Type()
}

// Info provided for fs.DirEntry compatibility and returns object itself.
func (fi IsoFileInfo) Info() (fs.FileInfo, error) {
	return fi, nil
}

func (fi IsoFileInfo) Sys() interface{} {
	return fi
}

func (fi IsoFileInfo) String() string {
	return fs.FormatDirEntry(fi)
}

// The End.
