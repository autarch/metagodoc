// Package directory is a shim replacement for the gosrc.Directory structure
// from the gddo. This is used by our copy of the doc package from that
// project.
package directory

import (
	"bytes"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// File represents a file.
type File struct {
	// File name with no directory.
	Name string

	// Contents of the file.
	Data []byte

	// Location of file on version control service website.
	BrowseURL string
}

type Directory struct {
	Path       string
	ImportPath string
	Files      []*File
}

func New(dir string, importPath, rootURL string) *Directory {
	return &Directory{
		Path:       dir,
		ImportPath: importPath,
		Files:      goFiles(dir, rootURL),
	}
}

func goFiles(dir, rootURL string) []*File {
	contents, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Panic(err)
	}

	var files []*File
	for _, f := range contents {
		if !isDocFile(f.Name()) {
			continue
		}

		c, err := ioutil.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			log.Panic(err)
		}

		url := strings.Join([]string{rootURL, f.Name()}, "/")
		files = append(files, &File{Name: f.Name(), Data: c, BrowseURL: url})
	}
	return files
}

func isDocFile(n string) bool {
	if strings.HasSuffix(n, ".go") && n[0] != '_' && n[0] != '.' {
		return true
	}
	return false
}

func (dir *Directory) Import(ctx *build.Context, mode build.ImportMode) (*build.Package, error) {
	safeCopy := *ctx
	ctx = &safeCopy
	ctx.JoinPath = path.Join
	ctx.IsAbsPath = path.IsAbs
	ctx.SplitPathList = func(list string) []string { return strings.Split(list, ":") }
	ctx.IsDir = func(path string) bool { return path == "." }
	ctx.HasSubdir = func(root, dir string) (rel string, ok bool) { return "", false }
	ctx.ReadDir = dir.readDir
	ctx.OpenFile = dir.openFile
	return ctx.ImportDir(".", mode)
}

type fileInfo struct{ f *File }

func (fi fileInfo) Name() string       { return fi.f.Name }
func (fi fileInfo) Size() int64        { return int64(len(fi.f.Data)) }
func (fi fileInfo) Mode() os.FileMode  { return 0 }
func (fi fileInfo) ModTime() time.Time { return time.Time{} }
func (fi fileInfo) IsDir() bool        { return false }
func (fi fileInfo) Sys() interface{}   { return nil }

func (dir *Directory) readDir(name string) ([]os.FileInfo, error) {
	if name != "." {
		return nil, os.ErrNotExist
	}
	fis := make([]os.FileInfo, len(dir.Files))
	for i, f := range dir.Files {
		fis[i] = fileInfo{f}
	}
	return fis, nil
}

func (dir *Directory) openFile(path string) (io.ReadCloser, error) {
	name := strings.TrimPrefix(path, "./")
	for _, f := range dir.Files {
		if f.Name == name {
			return ioutil.NopCloser(bytes.NewReader(f.Data)), nil
		}
	}
	return nil, os.ErrNotExist
}
