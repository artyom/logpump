package logreader

import (
	"sort"
	"testing"
)

func TestLogNameSlice_Sort(t *testing.T) {

	for n, testCase := range testCases {
		data := LogNameSlice(make([]string, len(testCase.in)))
		copy(data, testCase.in)
		sort.Sort(data)

		t.Log("sorted:")
		for i, item := range data {
			t.Log(i, item)
		}

		for i, item := range data {
			if item != testCase.out[i] {
				t.Fatalf("case %d, item %d differs: want %q, got %q",
					n, i, testCase.out[i], item)
			}
		}
	}
}

var (
	testCases = []struct {
		in, out []string
	}{
		{
			[]string{
				"/tmp/logs/logfile.log.7",
				"/tmp/logs/logfile.log.2.bz2",
				"/tmp/logs/logfile.log.gz",
				"/tmp/logs/logfile.log.3",
				"/tmp/logs/logfile.log.8.gz",
				"/tmp/logs/logfile.log.1",
				"/tmp/logs/logfile.log.screwit",
				"/tmp/logs/logfile.log",
				"/tmp/logs/logfile.log.9",
				"/tmp/logs/logfile.log.0",
				"/tmp/logs/logfile.log.10",
			}, []string{
				"/tmp/logs/logfile.log",
				"/tmp/logs/logfile.log.gz",
				"/tmp/logs/logfile.log.0",
				"/tmp/logs/logfile.log.1",
				"/tmp/logs/logfile.log.2.bz2",
				"/tmp/logs/logfile.log.3",
				"/tmp/logs/logfile.log.7",
				"/tmp/logs/logfile.log.8.gz",
				"/tmp/logs/logfile.log.9",
				"/tmp/logs/logfile.log.10",
				"/tmp/logs/logfile.log.screwit",
			},
		},
		{
			[]string{
				"@400000005390b5a019167e34.s",
				"@4000000053811d630b6a533c.s",
				"current",
				"@40000000539682cb3b723554.s",
			},
			[]string{
				"@4000000053811d630b6a533c.s",
				"@400000005390b5a019167e34.s",
				"@40000000539682cb3b723554.s",
				"current",
			},
		},
	}
)
