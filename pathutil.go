package joint

import (
	"path/filepath"
	"strings"
)

// HasFoldPrefix tests whether the string s begins with prefix
// without case sensitivity.
func HasFoldPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[0:len(prefix)], prefix)
}

// JoinPath performs fast join of two path chunks.
func JoinPath(dir, base string) string {
	if dir == "" || dir == "." {
		return base
	}
	if base == "" || base == "." {
		return dir
	}
	if dir[len(dir)-1] == '/' {
		if base[0] == '/' {
			return dir + base[1:]
		} else {
			return dir + base
		}
	}
	if base[0] == '/' {
		return dir + base
	}
	return dir + "/" + base
}

// IsTypeIso checks that endpoint-file in given path has ISO-extension.
func IsTypeIso(fpath string) bool {
	if len(fpath) < 4 {
		return false
	}
	var ext = fpath[len(fpath)-4:]
	return ext == ".iso" || ext == ".ISO"
}

// SplitUrl splits URL to address string and to path as is.
// For file path it splits to volume name and path at this volume.
func SplitUrl(urlpath string) (string, string, bool) {
	if i := strings.Index(urlpath, "://"); i != -1 {
		if j := strings.Index(urlpath[i+3:], "/"); j != -1 {
			return urlpath[:i+3+j], urlpath[i+3+j+1:], true
		}
		return urlpath, "", true
	}
	if vol := filepath.VolumeName(urlpath); len(vol) > 0 {
		if len(urlpath) > len(vol)+1 {
			return vol, urlpath[len(vol)+1:], false
		}
		return vol, "", false
	}
	return "", urlpath, false
}

// SplitKey splits full path to joint key to establish link, and
// remained local path. Also returns boolean value that given path
// is not at primary file system.
func SplitKey(fullpath string) (string, string, bool) {
	if IsTypeIso(fullpath) {
		return fullpath, "", true
	}
	var p = max(
		strings.LastIndex(fullpath, ".iso/"),
		strings.LastIndex(fullpath, ".ISO/"))
	if p != -1 {
		return fullpath[:p+4], fullpath[p+5:], true
	}
	var key, fpath, isurl = SplitUrl(fullpath)
	if isurl {
		if HasFoldPrefix(fullpath, "http://") || HasFoldPrefix(fullpath, "https://") {
			if root, ok := FindDavRoot(key, fpath); ok {
				return key + root, fpath[len(root)-1:], true
			}
		}
	}
	return key, fpath, isurl
}
