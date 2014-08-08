package logreader

import (
	"compress/bzip2"
	"compress/gzip"
	. "github.com/artyom/logreader/commonprefix"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	sort.Sort(sort.Reverse(LogNameSlice(logfiles)))
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

// LogNameSlice attaches the methods of Interface to []string, sorting in increasing order.
//
//	/tmp/logs/logfile.log
//	/tmp/logs/logfile.log.gz
//	/tmp/logs/logfile.log.0
//	/tmp/logs/logfile.log.1
//	/tmp/logs/logfile.log.2.bz2
//	/tmp/logs/logfile.log.3
//	/tmp/logs/logfile.log.4
//	/tmp/logs/logfile.log.5
//	/tmp/logs/logfile.log.6
//	/tmp/logs/logfile.log.7
//	/tmp/logs/logfile.log.8.gz
//	/tmp/logs/logfile.log.9
//	/tmp/logs/logfile.log.10
//	/tmp/logs/logfile.log.11
//	/tmp/logs/logfile.log.12
//	/tmp/logs/logfile.log.13
//	/tmp/logs/logfile.log.14
//	/tmp/logs/logfile.log.15
//	/tmp/logs/logfile.log.screwit
type LogNameSlice []string

func (p LogNameSlice) Len() int      { return len(p) }
func (p LogNameSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (p LogNameSlice) Less(i, j int) bool {
	left, right := p[i], p[j]

	left = strings.TrimSuffix(left, ".gz")
	left = strings.TrimSuffix(left, ".bz2")

	right = strings.TrimSuffix(right, ".gz")
	right = strings.TrimSuffix(right, ".bz2")

	prefix := CommonPrefix([]string{left, right})
	left = strings.TrimPrefix(left, prefix)
	right = strings.TrimPrefix(right, prefix)

	// trying to compare suffixes as digits
	left_i, left_err := strconv.ParseUint(left, 10, 8)
	right_i, right_err := strconv.ParseUint(right, 10, 8)
	if left_err == nil && right_err == nil {
		return left_i < right_i
	}

	return left < right
}
