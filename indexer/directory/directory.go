package directory

import (
	"log"
	"regexp"

	"github.com/autarch/gopal/indexer/esmodels"
)

// File represents a file within a directory. This could contain any sort of
// content.
type File struct {
	// File name with no directory.
	Name string

	// Contents of the file.
	Data []byte

	// Location of file on version control service website.
	BrowseURL string
}

// Directory describes a directory in a repository. This directory may contain
// a Go package or it may not.
type Directory struct {
	// The local directory to where the directory's contents have been cloned.
	localRoot string

	// The import path for this directory's package. Note that this can be set
	// even if the directory does not contain any packages.
	ImportPath string

	// Import path of package after resolving go-import meta tags, if any.
	ResolvedPath string

	// Cache validation tag. This tag is not necessarily an HTTP entity tag.
	// The tag is "" if there is no meaningful cache validation for the VCS.
	Etag string

	// Files.
	Files []*File

	// Subdirectories, not guaranteed to contain Go code.
	Subdirectories []string

	// Location of directory on version control service website.
	BrowseURL string

	// Format specifier for link to source line. It must contain one %s (file URL)
	// followed by one %d (source line number), or be empty string if not available.
	// Example: "%s#L%d".
	LineFmt string

	// The directory's README file data, if one exists
	readme *esmodels.About
}

func New(localRoot string) *Directory {
	return &Directory{localRoot}
}

func (d *Directory) HasReadme() bool {
	return d.Readme().exists
}

func (d *Directory) Readme() *esmodels.About {
	if d.readme != nil {
		return d.readme
	}

	log.Print("  looking for readme")

	for _, f := range d.Files {
		m := regexp.MustCompile(`^(?i)README(?:\.(md|txt))`).FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}

		t := "text/plain"
		if m[1] == "md" {
			t = "text/markdown"
		}

		d.readme = &esmodels.About{Content: string(f.Data), ContentType: t}
	}
	if d.readme == nil {
		d.readme = &esmodels.About{}
	}

	return d.readme
}
