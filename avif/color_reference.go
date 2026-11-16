package avif

import "math"

// referencePixel fixes the float32 multiply-add rounding used by the reviewed
// libavif 1.4.2 arm64 reference. An intermediate float32 multiply rounds too
// early at exact half-channel boundaries (V05T: red 49 instead of 50).
// Use explicit float64 FMA followed by float32 rounding, as verified against
// the independent corpus. This does not rely on compiler contraction or
// architecture-specific SIMD; other C builds need their own color validation.
func referencePixel(dst []byte, yy, cb, cr float32, alpha byte, c *rowConsts) {
	r := float32(math.FMA(float64(c.crCoef), float64(cr), float64(yy)))
	b := float32(math.FMA(float64(c.cbCoef), float64(cb), float64(yy)))
	g := yy - (2*(float32(c.gcr*cr)+float32(c.gcb*cb)))/c.kg
	dst[0] = uint8(0.5 + float32(clampF(r)*c.outMax))
	dst[1] = uint8(0.5 + float32(clampF(g)*c.outMax))
	dst[2] = uint8(0.5 + float32(clampF(b)*c.outMax))
	dst[3] = alpha
}

func convertRowReference444(dst []uint8, yf, u0, u1, v0, v1 []float32, a []uint8, n int, c *rowConsts) {
	for x := range n {
		referencePixel(dst[4*x:4*x+4], yf[x], u0[x+1], v0[x+1], a[x], c)
	}
}

func convertRowReference420(dst []uint8, yf, u0, u1, v0, v1 []float32, a []uint8, n int, c *rowConsts) {
	for x := range n {
		k := (x >> 1) + 1
		j := k - 1
		if x&1 != 0 {
			j = k + 1
		}
		cb := float32(u0[k]*c.w0) + float32(u0[j]*c.w1) + float32(u1[k]*c.w1) + float32(u1[j]*c.w2)
		cr := float32(v0[k]*c.w0) + float32(v0[j]*c.w1) + float32(v1[k]*c.w1) + float32(v1[j]*c.w2)
		referencePixel(dst[4*x:4*x+4], yf[x], cb, cr, a[x], c)
	}
}
