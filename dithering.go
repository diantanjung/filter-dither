// Package dithering provides a customizable image ditherer
package dithering

import (
	"image"
	"image/color"
	"image/draw"
)

var (
	// FloydSteinberg is the Floyd Steinberg matrix
	FloydSteinberg = [][]float32{{0, 0, 7.0 / 16.0}, {3.0 / 16.0, 5.0 / 16.0, 1.0 / 16.0}}
	// JarvisJudiceNinke is the JarvisJudiceNinke matrix
	JarvisJudiceNinke = [][]float32{{0, 0, 0, 7.0 / 48.0, 5.0 / 48.0}, {3.0 / 48.0, 5.0 / 48.0, 7.0 / 48.0, 5.0 / 48.0, 3.0 / 48.0}, {1.0 / 48.0, 3.0 / 48.0, 5.0 / 48.0, 3.0 / 48.0, 1.0 / 48.0}}
	// Stucki is the Stucki matrix
	Stucki = [][]float32{{0, 0, 0, 8.0 / 42.0, 4.0 / 42.0}, {2.0 / 42.0, 4.0 / 42.0, 8.0 / 42.0, 4.0 / 42.0, 2.0 / 42.0}, {1.0 / 42.0, 2.0 / 42.0, 4.0 / 42.0, 2.0 / 42.0, 1.0 / 42.0}}
	// Atkinson is the Atkinson matrix
	Atkinson = [][]float32{{0, 0, 1.0 / 8.0, 1.0 / 8.0}, {1.0 / 8.0, 1.0 / 8.0, 1.0 / 8.0, 0}, {0, 1.0 / 8.0, 0, 0}}
	// Burkes is the Burkes matrix
	Burkes = [][]float32{{0, 0, 0, 8.0 / 32.0, 4.0 / 32.0}, {2.0 / 32.0, 4.0 / 32.0, 8.0 / 32.0, 4.0 / 32.0, 2.0 / 32.0}}
	// Sierra is the Sierra matrix
	Sierra = [][]float32{{0, 0, 0, 5.0 / 32.0, 3.0 / 32.0}, {2.0 / 32.0, 4.0 / 32.0, 5.0 / 32.0, 4.0 / 32.0, 2.0 / 32.0}, {0, 2.0 / 32.0, 3.0 / 32.0, 2.0 / 32.0, 0}}
	// TwoRowSierra is a variant of the Sierrra matrix
	TwoRowSierra = [][]float32{{0, 0, 0, 4.0 / 16.0, 3.0 / 16.0}, {1.0 / 32.0, 2.0 / 32.0, 3.0 / 32.0, 2.0 / 32.0, 1.0 / 32.0}}
	// SierraLite is a variant of the Sierra matrix
	SierraLite = [][]float32{{0, 0, 2.0 / 4.0}, {1.0 / 4.0, 1.0 / 4.0, 0}}
)

// Dither represent dithering algorithm implementation
type Dither struct {
	// Matrix is the error diffusion matrix
	Matrix    [][]float32
	animation chan draw.Image
	nbFrames  int
}

// NewDither prepares a dithering algorithm
func NewDither(matrix [][]float32) Dither {
	return Dither{matrix, make(chan draw.Image), 1}
}

// NewDitherAnimation prepares a dithering algorithm and animation
//
// you can retrieve every generated frames thanks to RetrieveFrame
// Note: frames are shared using an unbuffered channel
func NewDitherAnimation(matrix [][]float32, nbFrames int) Dither {
	return Dither{matrix, make(chan draw.Image), nbFrames}
}

// abs gives the absolute value of a signed integer
func abs(x int16) uint16 {
	if x < 0 {
		return uint16(-x)
	}
	return uint16(x)
}

// findColor determines the closest color in a palette given the pixel color and the error
//
// It returns the closest color, the updated error and the distance between the error and the color
func findColor(err color.Color, pix color.Color, pal color.Palette) (color.RGBA, PixelError, uint16) {
	var errR, errG, errB,
		pixR, pixG, pixB,
		colR, colG, colB int16
	_errR, _errG, _errB, _ := err.RGBA()
	_pixR, _pixG, _pixB, _ := pix.RGBA()

	// Low-pass filter
	errR = int16(float32(int16(_errR)) * 0.75)
	errG = int16(float32(int16(_errG)) * 0.75)
	errB = int16(float32(int16(_errB)) * 0.75)

	pixR = int16(uint8(_pixR)) + errR
	pixG = int16(uint8(_pixG)) + errG
	pixB = int16(uint8(_pixB)) + errB

	var index int
	var minDiff uint16 = 1<<16 - 1

	for i, col := range pal {
		_colR, _colG, _colB, _ := col.RGBA()

		colR = int16(uint8(_colR))
		colG = int16(uint8(_colG))
		colB = int16(uint8(_colB))
		var distance = abs(pixR-colR) + abs(pixG-colG) + abs(pixB-colB)

		if distance < minDiff {
			index = i
			minDiff = distance
		}
	}

	_colR, _colG, _colB, _ := pal[index].RGBA()

	colR = int16(uint8(_colR))
	colG = int16(uint8(_colG))
	colB = int16(uint8(_colB))

	return color.RGBA{uint8(colR), uint8(colG), uint8(colB), 255},
		PixelError{float32(pixR - colR),
			float32(pixG - colG),
			float32(pixB - colB),
			1<<16 - 1},
		minDiff
}

func findShift(matrix [][]float32) int {
	for _, v1 := range matrix {
		for j, v2 := range v1 {
			if v2 > 0.0 {
				return -j + 1
			}
		}
	}
	return 0
}

// Draw applies an error diffusion algorithm to the src image
func (dit Dither) Draw(dst draw.Image, rect image.Rectangle, src image.Image) {
	if _, ok := dst.(*image.Paletted); !ok {
		return
	}
	p := dst.(*image.Paletted).Palette

	err := NewErrorImage(rect)
	shift := findShift(dit.Matrix)

	pixPerFrame := (rect.Dx() * rect.Dy()) / dit.nbFrames

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// using the closest color
			r, e, _ := findColor(err.PixelErrorAt(x, y), src.At(x, y), p)
			dst.Set(x, y, r)
			err.SetPixelError(x, y, e)

			if (y != 0 && x != 0) && (((y*rect.Dy())+x)%pixPerFrame == 0) {
				dit.animation <- dst
			}

			// diffusing the error using the diffusion matrix
			for i, v1 := range dit.Matrix {
				for j, v2 := range v1 {
					err.SetPixelError(x+j+shift, y+i,
						err.PixelErrorAt(x+j+shift, y+i).Add(err.PixelErrorAt(x, y).Mul(v2)))
				}
			}
		}
	}
}


