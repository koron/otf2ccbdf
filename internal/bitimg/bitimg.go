// Package bitimg provides 1-bit monochronme image.
package bitimg

import (
	"image"
	"image/color"
	"image/draw"
)

type Bit bool

var BitModel = color.ModelFunc(func(c color.Color) color.Color {
	return toBit(c)
})

func toBit(c color.Color) Bit {
	switch v := c.(type) {
	case Bit:
		return v
	default:
		g := color.GrayModel.Convert(c).(color.Gray)
		return g.Y > 127
	}
}

func (b Bit) RGBA() (uint32, uint32, uint32, uint32) {
	if b {
		return 65535, 65535, 65535, 65535
	}
	return 0, 0, 0, 65535
}

type Image struct {
	buf  []byte
	xn   int
	rect image.Rectangle
}

func New(r image.Rectangle) *Image {
	w, h := r.Dx(), r.Dy()
	xn := (w + 7) / 8
	buf := make([]byte, xn*h)
	return &Image{
		buf:  buf,
		xn:   xn,
		rect: r,
	}
}

func (img *Image) Xn() int { return img.xn }

func (img *Image) Bytes() []byte { return img.buf }

func (img *Image) Clear() {
	for i := range img.buf {
		img.buf[i] = 0
	}
}

var (
	_ image.Image = (*Image)(nil)
	_ draw.Image  = (*Image)(nil)
)

func (img *Image) ColorModel() color.Model {
	return BitModel
}

func (img *Image) Bounds() image.Rectangle {
	return img.rect
}

func (img *Image) address(x, y int) (index, shift int) {
	x -= img.rect.Min.X
	y -= img.rect.Min.Y
	return y*img.xn + x/8, x % 8
}

func (img *Image) At(x, y int) color.Color {
	idx, shift := img.address(x, y)
	mask := byte(0x80) >> shift
	if img.buf[idx]&mask != 0 {
		return color.White
	}
	return color.Black
}

func (img *Image) Set(x, y int, c color.Color) {
	idx, shift := img.address(x, y)
	if idx >= len(img.buf) {
		return
	}
	if toBit(c) {
		img.buf[idx] |= byte(0x80) >> shift
		return
	}
	img.buf[idx] &= ^(byte(0x80) >> shift)
}
