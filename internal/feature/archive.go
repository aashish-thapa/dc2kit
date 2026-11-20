package feature

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// Payload limits. A feature is a few shell scripts; anything near these
// bounds is not one, and the bytes come from a registry we do not control.
const (
	maxPayloadBytes = 64 << 20
	maxPayloadFiles = 2000
)

// extractArchive reads a feature layer (tar, optionally gzipped) into
// memory. Only regular files and directories are accepted, and every path
// must stay inside the payload root: the result is later written into a
// build context, so a `../` entry would write outside it.
func extractArchive(blob []byte) ([]File, error) {
	reader, err := maybeGunzip(blob)
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(io.LimitReader(reader, maxPayloadBytes+1))

	var files []File
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read feature archive: %w", err)
		}
		name, err := safeRelativePath(hdr.Name)
		if err != nil {
			return nil, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return nil, fmt.Errorf("feature archive entry %q: only regular files are allowed", hdr.Name)
		}
		if name == "" {
			continue
		}
		total += hdr.Size
		if total > maxPayloadBytes || len(files) >= maxPayloadFiles {
			return nil, fmt.Errorf("feature archive exceeds %d bytes or %d files", maxPayloadBytes, maxPayloadFiles)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", hdr.Name, err)
		}
		files = append(files, File{Path: name, Mode: hdr.FileInfo().Mode().Perm(), Data: data})
	}
	return files, nil
}

func maybeGunzip(blob []byte) (io.Reader, error) {
	buffered := bufio.NewReader(bytes.NewReader(blob))
	magic, _ := buffered.Peek(2)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(buffered)
		if err != nil {
			return nil, fmt.Errorf("feature archive: %w", err)
		}
		return gz, nil
	}
	return buffered, nil
}

// safeRelativePath cleans an archive path and refuses one that is absolute
// or climbs out of the root. "./" alone maps to the empty root path.
func safeRelativePath(name string) (string, error) {
	if strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("feature archive entry %q: absolute or backslash path", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("feature archive entry %q escapes the payload root", name)
	}
	return cleaned, nil
}
