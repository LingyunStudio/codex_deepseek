// +build ignore

package main

import (
	"encoding/binary"
	"image/png"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		panic("usage: go run png2ico.go <input.png> <output.ico>")
	}
	inF, _ := os.Open(os.Args[1])
	img, _ := png.Decode(inF)
	inF.Close()

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			off := (y*w + x) * 4
			pixels[off] = byte(b >> 8)
			pixels[off+1] = byte(g >> 8)
			pixels[off+2] = byte(r >> 8)
			pixels[off+3] = byte(a >> 8)
		}
	}

	out, _ := os.Create(os.Args[2])
	defer out.Close()
	binary.Write(out, binary.LittleEndian, uint16(0))
	binary.Write(out, binary.LittleEndian, uint16(1))
	binary.Write(out, binary.LittleEndian, uint16(1))
	binary.Write(out, binary.LittleEndian, uint8(min(w, 256)))
	binary.Write(out, binary.LittleEndian, uint8(min(h, 256)))
	binary.Write(out, binary.LittleEndian, uint8(0))
	binary.Write(out, binary.LittleEndian, uint8(0))
	binary.Write(out, binary.LittleEndian, uint16(1))
	binary.Write(out, binary.LittleEndian, uint16(32))
	binary.Write(out, binary.LittleEndian, uint32(len(pixels)))
	binary.Write(out, binary.LittleEndian, uint32(22))
	out.Write(pixels)
}
