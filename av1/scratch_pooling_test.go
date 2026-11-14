package av1

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisableScratchPoolingActualStreams(t *testing.T) {
	f, err := os.Open("testdata/streams/md5.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	count := 0
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		count++
		t.Run(fields[0], func(t *testing.T) {
			d := Decoder{Threads: 1, FrameThreads: 1, DisableScratchPooling: true}
			// Any scratch/frame pool Get on the disabled path fails, independently
			// of GC's nondeterministic retention of sync.Pool contents.
			d.tc.New = func() any { panic("disabled tile pool accessed") }
			d.fb.New = func() any { panic("disabled frame pool accessed") }
			defer d.Flush()
			for range 2 {
				sum := md5.Sum(decodeIVFWith(t, &d, filepath.Join("testdata/streams", fields[0])))
				if hex.EncodeToString(sum[:]) != fields[1] {
					t.Fatal("changed decoded planes", sum)
				}
				d.Flush()
			}
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("missing stream references")
	}
}

func TestUnpooledTaskCleanup(t *testing.T) {
	f := &frameContext[uint8, int16]{}
	task := f.newTaskContext()
	if task.scratch == nil {
		t.Fatal("missing private scratch")
	}
	f.putTaskContext(task)
	if task.scratch != nil {
		t.Fatal("cleanup retained scratch")
	}
	// No pool pointer exists on this path. Buffer cleanup must not call Put.
	f.bufs = &frameBufs[uint8]{}
	f.releasePlanes()
	if f.bufs != nil {
		t.Fatal("cleanup retained frame buffers")
	}
}
