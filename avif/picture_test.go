package avif

import (
	"bytes"
	"errors"
	"testing"

	"github.com/appforges/gav1d/av1"
)

func testBorrowedPicture() *av1.Picture {
	return &av1.Picture{Width: 2, Height: 2, BitDepth: 8, Layout: av1.LayoutI444,
		ColorPrimaries: 1, Transfer: 13, MatrixCoeffs: 6, FullRange: true,
		Stride: [3]int{2, 2, 2}, Data: [3][]byte{{16, 235, 81, 145}, {128, 128, 90, 54}, {128, 128, 240, 34}}}
}

func TestPictureToNRGBAOwnership(t *testing.T) {
	p := testBorrowedPicture() // Deterministic unit fixture, not a trusted API source.
	var before [3][]byte
	for i := range 3 {
		before[i] = bytes.Clone(p.Data[i])
	}
	a, err := PictureToNRGBA(p)
	if err != nil || !a.Opaque() || a.Bounds().Dx() != p.Width || a.Bounds().Dy() != p.Height {
		t.Fatal(a, err)
	}
	b, err := PictureToNRGBA(p)
	if err != nil || !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal(err)
	}
	for i := range 3 {
		if !bytes.Equal(before[i], p.Data[i]) {
			t.Fatal("input modified")
		}
	}
	a.Pix[0] ^= 255
	if bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("outputs alias")
	}
	for i := range 3 {
		if !bytes.Equal(before[i], p.Data[i]) {
			t.Fatal("output aliases input")
		}
	}
	p.Release()
	if len(b.Pix) == 0 || !b.Opaque() {
		t.Fatal("release invalidated independent output")
	}
	if out, err := PictureToNRGBA(p); out != nil || !errors.Is(err, ErrInvalid) {
		t.Fatal("released picture accepted", err)
	}
}

func TestPictureToNRGBARejectsBeforeConversion(t *testing.T) {
	for _, name := range []string{"nil", "depth", "layout", "primaries", "transfer", "matrix", "size", "stride", "short", "overflow"} {
		t.Run(name, func(t *testing.T) {
			p := testBorrowedPicture()
			want := ErrInvalid
			switch name {
			case "nil":
				p = nil
			case "depth":
				p.BitDepth = 10
				want = ErrUnsupported
			case "layout":
				p.Layout = av1.LayoutI422
				want = ErrUnsupported
			case "primaries":
				p.ColorPrimaries = 9
				want = ErrUnsupported
			case "transfer":
				p.Transfer = 16
				want = ErrUnsupported
			case "matrix":
				p.MatrixCoeffs = 9
				want = ErrUnsupported
			case "size":
				p.Width = 0
			case "stride":
				p.Stride[1] = 0
			case "short":
				p.Data[2] = p.Data[2][:1]
			case "overflow":
				p.Width = int(^uint(0) >> 1)
			}
			if out, err := PictureToNRGBA(p); out != nil || !errors.Is(err, want) {
				t.Fatal(out, err, want)
			}
		})
	}
}
