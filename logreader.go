package logreader

import (
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A LogReader reads data from separate log files as a whole. Files with
// .bz2/.gz suffixes unpacked automatically on the fly.
type LogReader struct {
	io.Reader
	Pattern string
	files   []*os.File
}

// Close closes the reader and its underlying files
func (l *LogReader) Close() {
	for _, v := range l.files {
		v.Close()
	}
}

// NewPlainTextReader wraps os.File, returning plain text Reader and error.
// gzip and bzip2 archives are wrapped with respective unpacking functions.
func NewPlainTextReader(f *os.File) (io.Reader, error) {
	switch {
	case strings.HasSuffix(f.Name(), ".gz"):
		return gzip.NewReader(f)
	case strings.HasSuffix(f.Name(), ".bz2"):
		return bzip2.NewReader(f), nil
	}
	return f, nil
}

// NewPlainTextReader returns new Reader as well as error. Reader is the
// logical concatenation of the log files matching the pattern provided.
// They're read sequentially. Once all inputs are drained, Read will return
// EOF.
func NewLogReader(pattern string) (l *LogReader, err error) {
	logfiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(logfiles)))
	files := make([]*os.File, len(logfiles))
	readers := make([]io.Reader, len(logfiles))
	for i, v := range logfiles {
		file, err := os.Open(v)
		if err != nil {
			return nil, err
		}
		files[i] = file
		r, err := NewPlainTextReader(file)
		if err != nil {
			return nil, err
		}
		readers[i] = r
	}
	mr := io.MultiReader(readers...)
	l = &LogReader{
		Reader:  mr,
		Pattern: pattern,
		files:   files,
	}
	return l, nil
}
