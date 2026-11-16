package avif

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"testing"

	"github.com/appforges/gav1d/av1"
)

func TestBoundletReferenceRoundingRealPicture(t *testing.T) {
	// Immutable AVIF and independently decoded libavif/libaom reference from
	// Boundlet's V05T geometry corpus; no native tools run in this regression.
	raw, err := os.ReadFile("testdata/boundlet/v05t-rgb-255x255-limited-420.avif")
	if err != nil {
		t.Fatal(err)
	}
	var obus []byte
	for pos := 0; pos < len(raw); {
		n := int(binary.BigEndian.Uint32(raw[pos:]))
		if n < 8 || n > len(raw)-pos {
			t.Fatal("invalid frozen fixture")
		}
		if string(raw[pos+4:pos+8]) == "mdat" {
			obus = raw[pos+8 : pos+n]
			break
		}
		pos += n
	}
	d := &av1.Decoder{Threads: 1, FrameThreads: 1, DisableScratchPooling: true}
	defer d.Flush()
	if err = d.Send(obus); err != nil {
		t.Fatal(err)
	}
	pic, err := d.Receive()
	if err != nil {
		t.Fatal(err)
	}
	defer pic.Release()
	got, err := PictureToNRGBA(pic)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("testdata/boundlet/v05t-rgb-255x255-limited-420-reference.png")
	if err != nil {
		t.Fatal(err)
	}
	want, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatal("dimensions")
	}
	for y := range pic.Height {
		for x := range pic.Width {
			r, g, b, a := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if r != wr || g != wg || b != wb || a != wa {
				t.Fatalf("RGB mismatch %d,%d", x, y)
			}
		}
	}
	// The generic upstream rendering entry is deliberately unchanged. Retain
	// the exact original half-channel witness rather than hiding its difference.
	legacy, err := pictureToImage(pic, nil, ColorInfo{Primaries: 1, Transfer: 13, Matrix: 6, FullRange: false})
	if err != nil {
		t.Fatal(err)
	}
	r, _, _, _ := legacy.At(49, 236).RGBA()
	fixed, _, _, _ := got.At(49, 236).RGBA()
	if r != 49*257 || fixed != 50*257 {
		t.Fatalf("legacy/fixed red %d/%d", r, fixed)
	}
}
