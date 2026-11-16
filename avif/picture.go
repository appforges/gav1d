package avif

import (
	"image"

	"github.com/appforges/gav1d/av1"
)

// PictureToNRGBA synchronously converts a live decoder Picture to an independently
// owned opaque NRGBA, using bilinear chroma and the reviewed reference's explicit
// multiply-add rounding. Ordinary Decode retains upstream arithmetic.
// pic is required, borrowed read-only, and must remain live throughout the call.
// The caller owns its lifetime: this function does not decode, retain, Release,
// or Flush it. Do not pass fabricated pictures or race the call with Release.
//
// This narrow entry accepts only 8-bit I420/I444 SDR pictures with CICP 1/13/6
// (BT.709 primaries, sRGB transfer, BT.601 matrix), full or limited range.
// It does not apply container metadata, ICC profiles, cropping or orientation.
// Invalid planes return ErrInvalid; unsupported formats return ErrUnsupported.
func PictureToNRGBA(pic *av1.Picture) (*image.NRGBA, error) {
	if pic == nil || pic.Width <= 0 || pic.Height <= 0 {
		return nil, ErrInvalid
	}
	if pic.BitDepth != 8 || (pic.Layout != av1.LayoutI420 && pic.Layout != av1.LayoutI444) ||
		pic.ColorPrimaries != 1 || pic.Transfer != 13 || pic.MatrixCoeffs != 6 {
		return nil, ErrUnsupported
	}
	// Four bytes per pixel in NRGBA. Divide before multiplying to reject
	// hostile dimensions without integer overflow or an output allocation.
	const nrgbaBytesPerPixel = 4
	maxPixels := int(^uint(0)>>1) / nrgbaBytesPerPixel
	if pic.Width > maxPixels/pic.Height {
		return nil, ErrInvalid
	}
	for plane := range 3 {
		w, h := pic.Width, pic.Height
		if plane != 0 && pic.Layout == av1.LayoutI420 {
			w, h = w/2+w%2, h/2+h%2
		}
		stride := pic.Stride[plane]
		if stride < w || len(pic.Data[plane]) < w || (h-1) > (len(pic.Data[plane])-w)/stride {
			return nil, ErrInvalid
		}
	}
	img, err := pictureToImagePolicy(pic, nil, ColorInfo{
		Primaries: uint16(pic.ColorPrimaries), Transfer: uint16(pic.Transfer),
		Matrix: uint16(pic.MatrixCoeffs), FullRange: pic.FullRange,
	}, true)
	if err != nil {
		return nil, err
	}
	return img.(*image.NRGBA), nil // The checked 8-bit input selects this exact type.
}
