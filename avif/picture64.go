package avif

import (
	"encoding/binary"
	"image"
	"math"

	"github.com/appforges/gav1d/av1"
)

const (
	nrgba64Primaries              = 1                 // BT.709 color primaries; unknown or other primaries are unsupported.
	nrgba64Transfer               = 13                // IEC 61966-2-1/sRGB transfer; not HDR/PQ/HLG.
	nrgba64Matrix                 = 6                 // SMPTE 170M/BT.601 YCbCr matrix, not RGB identity.
	nrgba64Depth                  = 10                // This entry admits ten-bit samples only, not arbitrary high bit depths.
	nrgba64Samples                = 1 << nrgba64Depth // Lookup tables cover every legal source sample.
	nrgba64SampleBytes            = 2                 // AV1 high-bit-depth plane samples are native-endian uint16 values.
	nrgba64PixelBytes             = 8                 // Go NRGBA64 stores big-endian R/G/B/A, two bytes per channel.
	nrgba64Opaque                 = 65535             // Full sixteen-bit alpha and RGB output range.
	nrgba64LimitedYBias   float32 = 64                // 16 << (10-8): limited-range luma black code.
	nrgba64LimitedYRange  float32 = 876               // 219 << (10-8): limited-range luma span.
	nrgba64UVBias         float32 = 512               // Center chroma code for either range.
	nrgba64LimitedUVRange float32 = 896               // 224 << (10-8): limited-range chroma span.
	nrgba64Near           float32 = 9.0 / 16          // Centered bilinear nearest-sample weight.
	nrgba64Adjacent       float32 = 3.0 / 16          // Each horizontal/vertical neighbor weight.
	nrgba64Diagonal       float32 = 1.0 / 16          // Diagonal neighbor weight.
)

// PictureToNRGBA64 synchronously converts a live ten-bit I420/I444 Picture to an
// independently owned opaque NRGBA64. It accepts only CICP 1/13/6 (BT.709
// primaries, sRGB transfer, BT.601 matrix), full or limited range, using the
// reviewed explicit float32/FMA RGB16 policy and centered bilinear chroma.
// Ordinary Decode and the eight-bit PictureToNRGBA keep their existing paths.
//
// pic is required, borrowed read-only and must stay live for the entire call.
// The caller must not mutate or Release it concurrently. This function neither
// decodes nor retains, Flushes or Releases pic. The result aliases no input or
// previous result and stays valid after the caller releases the picture.
//
// Container selection, static/alpha verification, transforms, ICC processing,
// authorization and resource admission belong to the caller; a Picture cannot
// establish those facts. This entry has no context, CPU/RSS or pixel quota.
// Bad dimensions/planes/byte strides/samples return ErrInvalid; unsupported
// depth/layout/color returns ErrUnsupported. Errors return no partial image.
func PictureToNRGBA64(pic *av1.Picture) (*image.NRGBA64, error) {
	if pic == nil || pic.Width <= 0 || pic.Height <= 0 {
		return nil, ErrInvalid
	}
	if pic.BitDepth != nrgba64Depth || (pic.Layout != av1.LayoutI420 && pic.Layout != av1.LayoutI444) || pic.ColorPrimaries != nrgba64Primaries || pic.Transfer != nrgba64Transfer || pic.MatrixCoeffs != nrgba64Matrix {
		return nil, ErrUnsupported
	}
	// Divide before multiplying. This also bounds each two-byte source row
	// and the ceil(width/2) expression without overflowing an int.
	maxPixels := int(^uint(0)>>1) / nrgba64PixelBytes
	if pic.Width > maxPixels/pic.Height {
		return nil, ErrInvalid
	}
	for pl := range 3 {
		w, h := pic.Width, pic.Height
		if pl != 0 && pic.Layout == av1.LayoutI420 {
			w, h = w/2+w%2, h/2+h%2
		}
		rowBytes, stride := w*nrgba64SampleBytes, pic.Stride[pl]
		if stride < rowBytes || stride%nrgba64SampleBytes != 0 || len(pic.Data[pl]) < rowBytes || h-1 > (len(pic.Data[pl])-rowBytes)/stride {
			return nil, ErrInvalid
		}
		// Validate every displayed sample before allocating the output. Padding
		// is not image data; arbitrary padding bytes are never used as samples.
		for y := range h {
			for x := range w {
				if binary.NativeEndian.Uint16(pic.Data[pl][y*stride+x*nrgba64SampleBytes:]) >= nrgba64Samples {
					return nil, ErrInvalid
				}
			}
		}
	}

	yBias, yRange, uvRange := float32(0), float32(nrgba64Samples-1), float32(nrgba64Samples-1)
	if !pic.FullRange {
		yBias, yRange, uvRange = nrgba64LimitedYBias, nrgba64LimitedYRange, nrgba64LimitedUVRange
	}
	var tableY, tableUV [nrgba64Samples]float32
	for value := range nrgba64Samples {
		tableY[value] = (float32(value) - yBias) / yRange
		tableUV[value] = (float32(value) - nrgba64UVBias) / uvRange
	}
	// Preserve the rounded float32 operations, including kg and the derived
	// coefficient products. Compile-time exact rational folding is different.
	kr, kb := float32(.299), float32(.114)
	kg := float32(float32(1-kr) - kb)
	rCoef, bCoef := float32(2*float32(1-kr)), float32(2*float32(1-kb))
	gCr, gCb := float32(kr*float32(1-kr)), float32(kb*float32(1-kb))
	dst := image.NewNRGBA64(image.Rect(0, 0, pic.Width, pic.Height))
	for y := range pic.Height {
		for x := range pic.Width {
			yValue := tableY[binary.NativeEndian.Uint16(pic.Data[0][y*pic.Stride[0]+x*nrgba64SampleBytes:])]
			cb, cr := pictureChroma64(pic, 1, x, y, &tableUV), pictureChroma64(pic, 2, x, y, &tableUV)
			r, b := pictureFMA64(rCoef, cr, yValue), pictureFMA64(bCoef, cb, yValue)
			greenTerm := pictureFMA64(gCr, cr, float32(gCb*cb))
			g := float32(yValue - float32(float32(2*greenTerm)/kg))
			off := y*dst.Stride + x*nrgba64PixelBytes
			for channel, value := range [4]uint16{pictureChannel64(r), pictureChannel64(g), pictureChannel64(b), nrgba64Opaque} {
				binary.BigEndian.PutUint16(dst.Pix[off+channel*nrgba64SampleBytes:], value)
			}
		}
	}
	return dst, nil
}

// The policy explicitly specifies binary64 FMA followed by binary32 rounding;
// it does not rely on compiler contraction or claim all C builds are identical.
func pictureFMA64(a, b, c float32) float32 {
	return float32(math.FMA(float64(a), float64(b), float64(c)))
}
func pictureChannel64(v float32) uint16 {
	v = min(float32(1), max(float32(0), v))
	return uint16(pictureFMA64(v, nrgba64Opaque, .5))
}
func pictureChroma64(pic *av1.Picture, plane, x, y int, table *[nrgba64Samples]float32) float32 {
	sample := func(px, py int) float32 {
		return table[binary.NativeEndian.Uint16(pic.Data[plane][py*pic.Stride[plane]+px*nrgba64SampleBytes:])]
	}
	if pic.Layout == av1.LayoutI444 {
		return sample(x, y)
	}
	w, h := pic.Width/2+pic.Width%2, pic.Height/2+pic.Height%2
	x0, y0 := x/2, y/2
	x1, y1 := x0-1, y0-1
	if x%2 != 0 {
		x1 = x0 + 1
	}
	if y%2 != 0 {
		y1 = y0 + 1
	}
	x1, y1 = min(w-1, max(0, x1)), min(h-1, max(0, y1))
	// Reference order: adjacent column is multiplied first, then the near
	// sample is fused into it, followed by adjacent row and diagonal.
	v := pictureFMA64(sample(x0, y0), nrgba64Near, float32(sample(x1, y0)*nrgba64Adjacent))
	v = pictureFMA64(sample(x0, y1), nrgba64Adjacent, v)
	return pictureFMA64(sample(x1, y1), nrgba64Diagonal, v)
}
