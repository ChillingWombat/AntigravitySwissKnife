package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
)

// resizeBilinear resizes an image to target dimensions with bilinear filtering for smooth edges
func resizeBilinear(src image.Image, targetW, targetH int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	for y := 0; y < targetH; y++ {
		sy := float64(y) * float64(srcH-1) / float64(targetH-1)
		y0 := int(sy)
		y1 := y0 + 1
		if y1 >= srcH {
			y1 = y0
		}
		yf := sy - float64(y0)

		for x := 0; x < targetW; x++ {
			sx := float64(x) * float64(srcW-1) / float64(targetW-1)
			x0 := int(sx)
			x1 := x0 + 1
			if x1 >= srcW {
				x1 = x0
			}
			xf := sx - float64(x0)

			r00, g00, b00, a00 := src.At(bounds.Min.X+x0, bounds.Min.Y+y0).RGBA()
			r10, g10, b10, a10 := src.At(bounds.Min.X+x1, bounds.Min.Y+y0).RGBA()
			r01, g01, b01, a01 := src.At(bounds.Min.X+x0, bounds.Min.Y+y1).RGBA()
			r11, g11, b11, a11 := src.At(bounds.Min.X+x1, bounds.Min.Y+y1).RGBA()

			interp := func(v00, v10, v01, v11 uint32) uint8 {
				top := float64(v00)*(1.0-xf) + float64(v10)*xf
				bot := float64(v01)*(1.0-xf) + float64(v11)*xf
				val := top*(1.0-yf) + bot*yf
				return uint8(val / 257)
			}

			dst.SetRGBA(x, y, color.RGBA{
				R: interp(r00, r10, r01, r11),
				G: interp(g00, g10, g01, g11),
				B: interp(b00, b10, b01, b11),
				A: interp(a00, a10, a01, a11),
			})
		}
	}
	return dst
}

func main() {
	f, err := os.Open("assets/icons/icon_1024x1024.png")
	if err != nil {
		log.Fatalf("failed to open source icon: %v", err)
	}
	defer f.Close()

	src, err := png.Decode(f)
	if err != nil {
		log.Fatalf("failed to decode png: %v", err)
	}

	// Apple HIG specifies an 824x824 squircle centered in a 1024x1024 grid
	// Inset: 100px on all sides (824 / 1024 = 80.5% grid allocation)
	canvas1024 := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	scaled824 := resizeBilinear(src, 824, 824)
	draw.Draw(canvas1024, image.Rect(100, 100, 924, 924), scaled824, image.Point{}, draw.Over)

	out1024, err := os.Create("assets/icons/icon_mac.png")
	if err != nil {
		log.Fatalf("failed to create icon_mac.png: %v", err)
	}
	defer out1024.Close()
	if err := png.Encode(out1024, canvas1024); err != nil {
		log.Fatalf("failed to encode icon_mac.png: %v", err)
	}

	// 256x256 version for getWindowIcon() on macOS (206x206 centered in 256x256)
	canvas256 := image.NewRGBA(image.Rect(0, 0, 256, 256))
	scaled206 := resizeBilinear(src, 206, 206)
	draw.Draw(canvas256, image.Rect(25, 25, 231, 231), scaled206, image.Point{}, draw.Over)

	out256, err := os.Create("assets/icons/icon_mac_256.png")
	if err != nil {
		log.Fatalf("failed to create icon_mac_256.png: %v", err)
	}
	defer out256.Close()
	if err := png.Encode(out256, canvas256); err != nil {
		log.Fatalf("failed to encode icon_mac_256.png: %v", err)
	}

	log.Println("Generated macOS padded icons successfully.")
}
