package storagenode

import (
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/kakao/varlog/pkg/types"
)

func newTempVolume(t *testing.T) Volume {
	t.Helper()
	volume, err := NewVolume(t.TempDir())
	if err != nil {
		t.Error(err)
	}
	return volume
}

type pathEntry struct {
	name     string
	isDir    bool
	children []pathEntry
}

func createPathEntries(root string, pathEntries []pathEntry, t *testing.T) {
	for _, pathEntry := range pathEntries {
		path := filepath.Join(root, pathEntry.name)
		if pathEntry.isDir {
			if err := os.MkdirAll(path, os.FileMode(0700)); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := ioutil.WriteFile(path, []byte(""), os.FileMode(0700)); err != nil {
				t.Fatal(err)
			}
		}
		createPathEntries(path, pathEntry.children, t)
	}
}

func TestVolume(t *testing.T) {
	Convey("Volume", t, func() {
		Convey("TempVolume", func() {
			volume := newTempVolume(t)
			So(len(volume), ShouldBeGreaterThan, 0)
			So(os.Remove(string(volume)), ShouldBeNil)
		})

		Convey("ReadLogStreamPaths - cid=1, snid=1", func() {
			// cid_1 (D)
			//   snid_1 (D)
			//     lsid_1 (D)
			//     lsid_2 (D)
			//     lsid_3 (F)
			//   snid_2 (D)
			//     lsid_1 (D)
			//     lsid_2 (D)
			//     lsid_3 (F)
			//   snid_3 (F)
			// cid_2 (D)
			//   snid_1 (D)
			//     lsid_1 (D)
			//     lsid_2 (D)
			//     lsid_3 (F)
			//   snid_2 (D)
			//     lsid_1 (D)
			//     lsid_2 (D)
			//     lsid_3 (F)
			//   snid_3 (F)
			// cid_3 (F)
			// ---
			// <volume>/cid_1/snid_1/lsid_1
			// <volume>/cid_1/snid_1/lsid_2
			cidChildren := []pathEntry{
				{
					name:  "snid_1",
					isDir: true,
					children: []pathEntry{
						{name: "lsid_1", isDir: true},
						{name: "lsid_2", isDir: true},
						{name: "lsid_3"},
					},
				},
				{
					name:  "snid_2",
					isDir: true,
					children: []pathEntry{
						{name: "lsid_1", isDir: true},
						{name: "lsid_2", isDir: true},
						{name: "lsid_3"},
					},
				},
				{
					name: "snid_3",
				},
			}
			pathEntries := []pathEntry{
				{name: "cid_1", isDir: true, children: cidChildren},
				{name: "cid_2", isDir: true, children: cidChildren},
				{name: "cid_3"},
			}
			volume := newTempVolume(t)
			createPathEntries(string(volume), pathEntries, t)
			logStreamPaths := volume.ReadLogStreamPaths(types.ClusterID(1), types.StorageNodeID(1))
			So(len(logStreamPaths), ShouldEqual, 2)
			So(logStreamPaths, ShouldContain, filepath.Join(string(volume), "cid_1", "snid_1", "lsid_1"))
			So(logStreamPaths, ShouldContain, filepath.Join(string(volume), "cid_1", "snid_1", "lsid_2"))
		})
	})
}

func TestValidDir(t *testing.T) {
	writableDir := newTempVolume(t)
	fp, err := os.Create(filepath.Join(string(writableDir), "_file"))
	if err != nil {
		t.Fatal(err)
	}
	tmpfile := fp.Name()
	if err := fp.Close(); err != nil {
		t.Fatal(err)
	}

	notWritableDir := newTempVolume(t)
	if err := os.Chmod(string(notWritableDir), 0400); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := os.RemoveAll(string(writableDir)); err != nil {
			t.Error(err)
		}

		if err := os.Chmod(string(notWritableDir), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(string(notWritableDir)); err != nil {
			t.Error(err)
		}

	}()

	var tests = []struct {
		in string
		ok bool
	}{
		{"", false},
		{tmpfile, false},
		{string(writableDir), true},
		{string(notWritableDir), false},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.in, func(t *testing.T) {
			actual := ValidDir(test.in)
			if test.ok != (actual == nil) {
				t.Errorf("input=%v, expected=%v, actual=%v", test.in, test.ok, actual)
			}
		})
	}
}

func TestParseLogStreamPath(t *testing.T) {
	type outputST struct {
		volume Volume
		cid    types.ClusterID
		snid   types.StorageNodeID
		lsid   types.LogStreamID
		isErr  bool
	}

	tests := []struct {
		input  string
		output outputST
	}{
		{
			input:  "abc",
			output: outputST{isErr: true},
		},
		{
			input:  "/abc/cid_1",
			output: outputST{isErr: true},
		},
		{
			input:  "/abc/cid_1/snid_2",
			output: outputST{isErr: true},
		},
		{
			input: "/abc/cid_1/snid_2/lsid_3",
			output: outputST{
				volume: Volume("/abc"),
				cid:    types.ClusterID(1),
				snid:   types.StorageNodeID(2),
				lsid:   types.LogStreamID(3),
			},
		},
		{
			input: "/cid_1/snid_2/lsid_3",
			output: outputST{
				volume: Volume("/"),
				cid:    types.ClusterID(1),
				snid:   types.StorageNodeID(2),
				lsid:   types.LogStreamID(3),
			},
		},
		{
			input:  "/abc/cid_1/snid_2/lsid_",
			output: outputST{isErr: true},
		},
		{
			input:  "/abc/cid_1/snid_2/lsid_",
			output: outputST{isErr: true},
		},
		{
			input:  "/abc/cid_1/snid_2/lsid_" + strconv.FormatUint(uint64(math.MaxUint32)+1, 10),
			output: outputST{isErr: true},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.input, func(t *testing.T) {
			vol, cid, snid, lsid, err := ParseLogStreamPath(test.input)
			if test.output.isErr != (err != nil) {
				t.Errorf("expected error=%v, actual error=%+v", test.output.isErr, err)
			}
			if err != nil {
				return
			}
			if test.output.volume != vol {
				t.Errorf("expected volume=%v, actual volume=%v", test.output.volume, vol)
			}
			if test.output.cid != cid {
				t.Errorf("expected cid=%v, actual cid=%v", test.output.cid, cid)
			}
			if test.output.snid != snid {
				t.Errorf("expected snid=%v, actual snid=%v", test.output.snid, snid)
			}
			if test.output.lsid != lsid {
				t.Errorf("expected lsid=%v, actual lsid=%v", test.output.lsid, lsid)
			}
		})
	}
}
