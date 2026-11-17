package avif

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/appforges/gav1d/av1"
)

const picture64Corpus = "testdata/boundlet/v05y"

type picture64Case struct {
	Name, Range, Chroma, AVIF, ReferencePNG, ReferenceY4M string
	Width, Height                                         int
}
type picture64Manifest struct {
	GeneratorSHA256 string
	Files           map[string]string
	Cases           []picture64Case
}

func picture64Read(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(picture64Corpus, name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func picture64Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func picture64Load(t *testing.T) picture64Manifest {
	t.Helper()
	var m picture64Manifest
	if e := json.Unmarshal(picture64Read(t, "manifest.json"), &m); e != nil {
		t.Fatal(e)
	}
	return m
}

// This is a fixed-data test reader, not production AVIF admission. Native
// encoder output and original witnesses each have exactly one mdat extent.
func picture64Visit(t *testing.T, raw []byte, visit func(*av1.Picture)) {
	t.Helper()
	var obu []byte
	for pos := 0; pos < len(raw); {
		if len(raw)-pos < 8 {
			t.Fatal("fixture box header")
		}
		n := int(binary.BigEndian.Uint32(raw[pos:]))
		if n < 8 || n > len(raw)-pos {
			t.Fatal("fixture box length")
		}
		if string(raw[pos+4:pos+8]) == "mdat" {
			if pos+n != len(raw) {
				t.Fatal("fixture extent")
			}
			obu = raw[pos+8 : pos+n]
		}
		pos += n
	}
	if len(obu) == 0 {
		t.Fatal("fixture has no sample")
	}
	d := &av1.Decoder{Threads: 1, FrameThreads: 1, DisableScratchPooling: true, FrameSizeLimit: 256 * 256}
	var held [2]*av1.Picture
	defer func() {
		d.Flush()
		for i, p := range held {
			if p != nil && (i == 0 || p != held[0]) {
				p.Release()
			}
		}
	}()
	if e := d.Send(obu); e != nil {
		t.Fatal(e)
	}
	p, e := d.Receive()
	held[0] = p
	if e != nil || p == nil {
		t.Fatal(p, e)
	}
	visit(p)
	extra, e := d.Receive()
	held[1] = extra
	if e != nil || extra != nil {
		t.Fatal("single frame not drained", e)
	}
}
func picture64Compare(t *testing.T, got image.Image, reference []byte) {
	t.Helper()
	if len(reference) < 26 || reference[24] != 16 || reference[25] != 2 {
		t.Fatal("oracle is not RGB16")
	}
	want, e := png.Decode(bytes.NewReader(reference))
	if e != nil {
		t.Fatal(e)
	}
	if mismatch := picture64Mismatch(got, want); mismatch != "" {
		t.Fatal(mismatch)
	}
}
func picture64Mismatch(got, want image.Image) string {
	if got.Bounds() != want.Bounds() {
		return "dimensions differ"
	}
	for y := range got.Bounds().Dy() {
		for x := range got.Bounds().Dx() {
			r, g, b, a := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if r != wr || g != wg || b != wb || a != wa {
				return fmt.Sprintf("RGB16 (%d,%d): %d,%d,%d,%d != %d,%d,%d,%d", x, y, r, g, b, a, wr, wg, wb, wa)
			}
		}
	}
	return ""
}
func picture64Planes(t *testing.T, p *av1.Picture, raw []byte, c picture64Case) {
	t.Helper()
	ref, e := readY4M(raw)
	if e != nil {
		t.Fatal(e)
	}
	wantLayout := av1.LayoutI444
	if c.Chroma == "420" {
		wantLayout = av1.LayoutI420
	}
	if p.Width != c.Width || p.Height != c.Height || p.BitDepth != 10 || p.Layout != wantLayout || p.ColorPrimaries != 1 || p.Transfer != 13 || p.MatrixCoeffs != 6 || p.FullRange != (c.Range == "full") || ref.w != p.Width || ref.h != p.Height || ref.depth != 10 || ref.layout != p.Layout {
		t.Fatal("actual picture/oracle facts differ")
	}
	if !strings.Contains(string(raw[:bytes.IndexByte(raw, '\n')]), "XCOLORRANGE="+strings.ToUpper(c.Range)) {
		t.Fatal("reference range")
	}
	for pl := range 3 {
		w, h := p.Width, p.Height
		if pl > 0 && p.Layout == av1.LayoutI420 {
			w, h = (w+1)/2, (h+1)/2
		}
		for y := range h {
			for x := range w {
				got := binary.NativeEndian.Uint16(p.Data[pl][y*p.Stride[pl]+2*x:])
				want := binary.LittleEndian.Uint16(ref.planes[pl][y*ref.stride[pl]+2*x:])
				if got != want {
					t.Fatalf("YUV plane%d (%d,%d):%d != %d", pl, x, y, got, want)
				}
			}
		}
	}
}
func TestPictureToNRGBA64IndependentCorpus(t *testing.T) {
	m := picture64Load(t)
	if len(m.Cases) != 32 || len(m.Files) != 120 {
		t.Fatal("incomplete independent corpus")
	}
	if picture64Hash(picture64Read(t, "referencegen.go.txt")) != m.GeneratorSHA256 {
		t.Fatal("reference generator source changed")
	}
	for name, want := range m.Files {
		if picture64Hash(picture64Read(t, name)) != want {
			t.Fatal("independent source/oracle changed", name)
		}
	}
	for _, c := range m.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var result *image.NRGBA64
			picture64Visit(t, picture64Read(t, c.AVIF), func(p *av1.Picture) {
				picture64Planes(t, p, picture64Read(t, c.ReferenceY4M), c)
				var before [3][]byte
				for pl := range 3 {
					before[pl] = bytes.Clone(p.Data[pl])
				}
				var e error
				result, e = PictureToNRGBA64(p)
				if e != nil {
					t.Fatal(e)
				}
				if !result.Opaque() || len(result.Pix) != c.Width*c.Height*8 || cap(result.Pix) != len(result.Pix) {
					t.Fatal("output ownership/format")
				}
				picture64Compare(t, result, picture64Read(t, c.ReferencePNG))
				for pl := range 3 {
					if !bytes.Equal(before[pl], p.Data[pl]) {
						t.Fatal("source was mutated/released")
					}
				}
			})
			// The decoder has now been flushed and the actual picture released.
			picture64Compare(t, result, picture64Read(t, c.ReferencePNG))
		})
	}
}
func TestPictureToNRGBA64OriginalWitnesses(t *testing.T) {
	var originals struct{ Files map[string]string }
	if e := json.Unmarshal(picture64Read(t, "original-witnesses.json"), &originals); e != nil {
		t.Fatal(e)
	}
	for n, h := range originals.Files {
		if picture64Hash(picture64Read(t, n)) != h {
			t.Fatal("old original/reference changed", n)
		}
	}
	for _, name := range []string{"yuv420-10", "yuv444-10"} {
		t.Run(name, func(t *testing.T) {
			raw := picture64Read(t, "original-"+name+".avif")
			ref := picture64Read(t, "original-"+name+"-bilinear.png")
			var first, second *image.NRGBA64
			picture64Visit(t, raw, func(p *av1.Picture) {
				c := picture64Case{Width: 160, Height: 120, Range: "limited", Chroma: "444"}
				if name == "yuv420-10" {
					c.Chroma = "420"
				}
				picture64Planes(t, p, picture64Read(t, "original-"+name+".y4m"), c)
				var e error
				first, e = PictureToNRGBA64(p)
				if e != nil {
					t.Fatal(e)
				}
				second, e = PictureToNRGBA64(p)
				if e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(first.Pix, second.Pix) {
					t.Fatal("repeat result differs")
				}
				picture64Compare(t, first, ref)
				first.Pix[0] ^= 1
				if bytes.Equal(first.Pix, second.Pix) {
					t.Fatal("returned buffers alias")
				}
				picture64Compare(t, second, ref)
			})
			picture64Compare(t, second, ref)
			// Preserve the ordinary upstream color failure instead of quietly
			// routing it through the new explicit entry.
			legacy, e := Decode(bytes.NewReader(raw))
			if e != nil {
				t.Fatal(e)
			}
			want, e := png.Decode(bytes.NewReader(ref))
			if e != nil {
				t.Fatal(e)
			}
			different := 0
			for y := range legacy.Bounds().Dy() {
				for x := range legacy.Bounds().Dx() {
					r, g, b, _ := legacy.At(x, y).RGBA()
					wr, wg, wb, _ := want.At(x, y).RGBA()
					for i, v := range [3]uint32{r, g, b} {
						if v != [3]uint32{wr, wg, wb}[i] {
							different++
						}
					}
				}
			}
			t.Logf("ordinary Decode still differs on %d RGB16 channels; new entry is exact", different)
			if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
				expected := 19
				if name == "yuv444-10" {
					expected = 10
				}
				if different != expected {
					t.Fatal("ordinary Decode behavior changed", different, expected)
				}
			}
		})
	}
}
func fakePicture64(layout int) *av1.Picture {
	p := &av1.Picture{Width: 3, Height: 2, BitDepth: 10, Layout: layout, ColorPrimaries: 1, Transfer: 13, MatrixCoeffs: 6, FullRange: true}
	for pl := range 3 {
		w, h := p.Width, p.Height
		if pl > 0 && layout == av1.LayoutI420 {
			w, h = (w+1)/2, (h+1)/2
		}
		p.Stride[pl] = 2*w + 2
		p.Data[pl] = bytes.Repeat([]byte{255}, p.Stride[pl]*h)
		for y := range h {
			for x := range w {
				binary.NativeEndian.PutUint16(p.Data[pl][y*p.Stride[pl]+2*x:], uint16((pl*313+y*67+x*251)%1024))
			}
		}
	}
	return p
}
func TestPictureToNRGBA64Validation(t *testing.T) {
	for _, name := range []string{"nil", "zero", "negative", "width_overflow", "height_overflow", "8bit", "12bit", "422", "mono", "primaries", "transfer", "matrix", "nil_plane", "short_plane", "zero_stride", "negative_stride", "odd_stride", "huge_stride", "bad_sample", "released"} {
		t.Run(name, func(t *testing.T) {
			p := fakePicture64(av1.LayoutI420)
			want := ErrInvalid
			switch name {
			case "nil":
				p = nil
			case "zero":
				p.Width = 0
			case "negative":
				p.Height = -1
			case "width_overflow":
				p.Width = int(^uint(0) >> 1)
			case "height_overflow":
				p.Height = int(^uint(0) >> 1)
			case "8bit":
				p.BitDepth = 8
				want = ErrUnsupported
			case "12bit":
				p.BitDepth = 12
				want = ErrUnsupported
			case "422":
				p.Layout = av1.LayoutI422
				want = ErrUnsupported
			case "mono":
				p.Layout = av1.LayoutI400
				want = ErrUnsupported
			case "primaries":
				p.ColorPrimaries = 9
				want = ErrUnsupported
			case "transfer":
				p.Transfer = 16
				want = ErrUnsupported
			case "matrix":
				p.MatrixCoeffs = 0
				want = ErrUnsupported
			case "nil_plane":
				p.Data[2] = nil
			case "short_plane":
				p.Data[0] = p.Data[0][:len(p.Data[0])-3]
			case "zero_stride":
				p.Stride[1] = 0
			case "negative_stride":
				p.Stride[1] = -1
			case "odd_stride":
				p.Stride[1]++
			case "huge_stride":
				p.Stride[0] = int(^uint(0)>>1) - 1
			case "bad_sample":
				binary.NativeEndian.PutUint16(p.Data[2], 1024)
			case "released":
				p.Release()
			}
			if out, e := PictureToNRGBA64(p); out != nil || !errors.Is(e, want) {
				t.Fatal(out, e, want)
			}
		})
	}
	for _, layout := range []int{av1.LayoutI420, av1.LayoutI444} {
		p := fakePicture64(layout)
		out, e := PictureToNRGBA64(p)
		if e != nil || !out.Opaque() {
			t.Fatal("padding is not a displayed sample", e)
		}
		for pl := range 3 {
			w, h := p.Width, p.Height
			if pl > 0 && layout == av1.LayoutI420 {
				w, h = (w+1)/2, (h+1)/2
			}
			p.Data[pl] = p.Data[pl][:(h-1)*p.Stride[pl]+w*2]
		}
		minimal, e := PictureToNRGBA64(p)
		if e != nil || !bytes.Equal(out.Pix, minimal.Pix) {
			t.Fatal("exact final-row byte length rejected", e)
		}
	}
}

func TestPictureToNRGBA64LowBitWitness(t *testing.T) {
	ref, e := png.Decode(bytes.NewReader(picture64Read(t, "original-yuv444-10-bilinear.png")))
	if e != nil {
		t.Fatal(e)
	}
	picture64Visit(t, picture64Read(t, "original-yuv444-10.avif"), func(p *av1.Picture) {
		img, e := PictureToNRGBA64(p)
		if e != nil {
			t.Fatal(e)
		}
		r, _, _, _ := img.At(0, 0).RGBA()
		img.Pix[1] ^= 1
		changed, _, _, _ := img.At(0, 0).RGBA()
		if r>>8 != changed>>8 || picture64Mismatch(img, ref) == "" {
			t.Fatal("RGB16 low-bit difference was hidden")
		}
	})
}
func TestPictureToNRGBA64ConcurrentBorrow(t *testing.T) {
	// Deterministic fabricated unit data only; actual decoder ownership is
	// verified separately above. Readers finish before the caller releases it.
	p := fakePicture64(av1.LayoutI420)
	var before [3][]byte
	for pl := range 3 {
		before[pl] = bytes.Clone(p.Data[pl])
	}
	const readers = 4
	results := make(chan *image.NRGBA64, readers)
	errs := make(chan error, readers)
	var wg sync.WaitGroup
	for range readers {
		wg.Add(1)
		go func() { defer wg.Done(); out, e := PictureToNRGBA64(p); results <- out; errs <- e }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var first *image.NRGBA64
	for out := range results {
		if first == nil {
			first = out
			continue
		}
		if !bytes.Equal(first.Pix, out.Pix) {
			t.Fatal("concurrent output differs")
		}
		out.Pix[0] ^= 1
		if bytes.Equal(first.Pix, out.Pix) {
			t.Fatal("concurrent outputs alias")
		}
	}
	for pl := range 3 {
		if !bytes.Equal(before[pl], p.Data[pl]) {
			t.Fatal("source changed")
		}
	}
}
