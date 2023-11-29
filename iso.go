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
	file  RFile
	img   *iso.Image
	cache map[string]*iso.File

	*iso.File
	*io.SectionReader
}

func NewIsoJoint() Joint {
	return &IsoJoint{}
}

func (j *IsoJoint) Make(isopath string) (err error) {
	if j.file, err = OpenFile(isopath); err != nil {
		return
	}
	if j.img, err = iso.OpenImage(j.file); err != nil {
		return
	}
	j.cache = map[string]*iso.File{}
	if j.cache[""], err = j.img.RootDir(); err != nil {
		return
	}
	return
}

func (j *IsoJoint) Cleanup() error {
	j.Close()
	var err = j.file.Close()
	j.file = nil
	return err
}

func (j *IsoJoint) Busy() bool {
	return j.File != nil
}

func (j *IsoJoint) Open(fpath string) (file fs.File, err error) {
	if j.File, err = j.OpenFile(fpath); err != nil {
		return
	}
	if sr := j.File.Reader(); sr != nil {
		j.SectionReader = sr.(*io.SectionReader)
	}
	file = j
	return
}

func (j *IsoJoint) Close() error {
	j.File = nil
	j.SectionReader = nil
	return nil
}

func (j *IsoJoint) OpenFile(intpath string) (file *iso.File, err error) {
	if file, ok := j.cache[intpath]; ok {
		return file, nil
	}

	var dec = charmap.Windows1251.NewDecoder()
	var curdir string
	var chunks = strings.Split(intpath, "/")
	file = j.cache[curdir] // get root directory
	for _, chunk := range chunks {
		if !file.IsDir() {
			err = fs.ErrNotExist
			return
		}
		var curpath = JoinFast(curdir, chunk)
		if f, ok := j.cache[curpath]; ok {
			file = f
		} else {
			var list []*iso.File
			if list, err = file.GetChildren(); err != nil {
				return
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
				err = fs.ErrNotExist
				return
			}
		}
		curdir = curpath
	}
	return
}

func (j *IsoJoint) ReadDir(n int) (ret []fs.DirEntry, err error) {
	var files []*iso.File
	if files, err = j.File.GetChildren(); err != nil {
		return
	}
	ret = make([]fs.DirEntry, 0, len(files))
	for i, file := range files {
		if i == n {
			break
		}
		ret = append(ret, fs.FileInfoToDirEntry(IsoFileInfo{
			File: file,
		}))
	}
	return
}

func (j *IsoJoint) Stat() (fs.FileInfo, error) {
	return IsoFileInfo{j.File}, nil
}

func (j *IsoJoint) Info(fpath string) (fi fs.FileInfo, err error) {
	var file *iso.File
	if file, err = j.OpenFile(fpath); err != nil {
		return
	}
	fi = IsoFileInfo{
		File: file,
	}
	return
}

type IsoFileInfo struct {
	*iso.File
}

func (fi IsoFileInfo) Name() string {
	var dec = charmap.Windows1251.NewDecoder()
	var name, _ = dec.String(fi.File.Name())
	return name
}

func (fi IsoFileInfo) Sys() interface{} {
	return fi
}

// The End.
