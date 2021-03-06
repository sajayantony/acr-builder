package commands

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	build "github.com/Azure/acr-builder/pkg"
	"github.com/Azure/acr-builder/pkg/constants"
)

// ArchiveSource defines source in the form of an archive file
// Currently we only support tar.gz
type ArchiveSource struct {
	url       string
	targetDir string
	tracker   *DirectoryTracker
}

// NewArchiveSource creates a new archive source
func NewArchiveSource(url string, targetDir string) build.Source {
	return &ArchiveSource{url: url, targetDir: targetDir}
}

// Obtain downloads and extract the source
func (s *ArchiveSource) Obtain(runner build.Runner) error {
	response, err := http.Get(s.url)
	if err != nil {
		return fmt.Errorf("Failed to get archive file from %s, error: %s", s.url, err)
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			logrus.Warnf("Error closing http response from archive download, url: %s, error: %s", s.url, err)
		}
	}()

	baseDir := s.targetDir
	if baseDir == "" {
		baseDir = "."
	}
	if err := unTAR(baseDir, response.Body); err != nil {
		return fmt.Errorf("Failed to extract archive from %s, error: %s", s.url, err)
	}

	tracker, err := ChdirWithTracking(runner, s.targetDir)
	if err != nil {
		return err
	}
	s.tracker = tracker
	return nil
}

// Return chdir back, currently not delete the extacted source
func (s *ArchiveSource) Return(runner build.Runner) error {
	if s.tracker != nil {
		return s.tracker.Return(runner)
	}
	return nil
}

// Export exports the variables
func (s *ArchiveSource) Export() []build.EnvVar {
	exports := []build.EnvVar{}
	if s.targetDir != "" {
		exports = append(exports, build.EnvVar{
			Name:  constants.ExportsWorkingDir,
			Value: s.targetDir,
		})
	}
	return exports
}

func unTAR(baseDir string, r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	defer func() {
		err := gzr.Close()
		if err != nil {
			logrus.Warnf("Error closing gz archive, error: %s", err)
		}
	}()
	if err != nil {
		return err
	}
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return nil

		case err != nil:
			return err

		case header == nil:
			continue
		}

		target := filepath.Join(baseDir, header.Name)

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer func() {
				err := f.Close()
				if err != nil {
					logrus.Warnf("Error closing file %s, error: %s", target, err)
				}
			}()
			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

		default:
			logrus.Debugf("Ignoring unexpected file %s, type: %d", header.Name, header.Typeflag)
		}
	}
}
