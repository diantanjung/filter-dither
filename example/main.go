package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"

	"github.com/diantanjung/filter-dither"
)

func main()  {
	reader, err := os.Open("lenna.png")
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	src, _, err := image.Decode(reader)
	if err != nil {
		log.Fatal(err)
	}

	dst := image.NewPaletted(src.Bounds(), color.Palette{color.Black, color.White})

	floydSteinberg := dithering.NewDither(dithering.FloydSteinberg)
	floydSteinberg.Draw(dst, dst.Bounds(), src)

	file, err := os.Create("result.png")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	if err = png.Encode(file, dst); err != nil {
		log.Fatal(err)
	}
}