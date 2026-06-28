package main

import (
	"encoding/binary"
	"fmt"
	"image/png"
	"os"
)

func main() {
	sizes := []int{16, 32, 48, 256}
	var dibData [][]byte

	for _, sz := range sizes {
		f, err := os.Open(fmt.Sprintf("E:/CodeSeek/assets/icon-c-%d.png", sz))
		if err != nil {
			panic(err)
		}
		img, _ := png.Decode(f)
		f.Close()

		b := img.Bounds()
		w, h := b.Dx(), b.Dy()

		// BITMAPINFOHEADER (40 bytes)
		header := make([]byte, 40)
		binary.LittleEndian.PutUint32(header[0:], 40)   // biSize
		binary.LittleEndian.PutUint32(header[4:], uint32(w)) // biWidth
		// Height is doubled because ICO stores XOR+AND mask
		binary.LittleEndian.PutUint32(header[8:], uint32(h*2)) // biHeight
		binary.LittleEndian.PutUint16(header[12:], 1)    // biPlanes
		binary.LittleEndian.PutUint16(header[14:], 32)   // biBitCount
		// Rest is zero (BI_RGB, no compression)

		// XOR mask: BGRA pixels (bottom-up)
		xorMask := make([]byte, w*h*4)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b2, a := img.At(x, y).RGBA()
				off := ((h-1-y)*w + x) * 4 // bottom-up
				xorMask[off] = byte(b2 >> 8)
				xorMask[off+1] = byte(g >> 8)
				xorMask[off+2] = byte(r >> 8)
				xorMask[off+3] = byte(a >> 8)
			}
		}

		// AND mask: 1 bit per pixel, each row padded to 4 bytes
		andRowBytes := ((w + 31) / 32) * 4
		andMask := make([]byte, andRowBytes*h)

		data := make([]byte, 0, 40+len(xorMask)+len(andMask))
		data = append(data, header...)
		data = append(data, xorMask...)
		data = append(data, andMask...)
		dibData = append(dibData, data)
	}

	out, _ := os.Create("E:/CodeSeek/cmd/codeseek-gui/codeseek.ico")
	defer out.Close()

	// ICONDIR
	binary.Write(out, binary.LittleEndian, uint16(0))      // reserved
	binary.Write(out, binary.LittleEndian, uint16(1))      // ICO type
	binary.Write(out, binary.LittleEndian, uint16(len(sizes)))

	offset := uint32(6 + len(sizes)*16)
	for i, sz := range sizes {
		var w, h uint8
		if sz >= 256 { w, h = 0, 0 } else { w, h = uint8(sz), uint8(sz) }
		binary.Write(out, binary.LittleEndian, w)
		binary.Write(out, binary.LittleEndian, h)
		binary.Write(out, binary.LittleEndian, uint8(0))  // color count
		binary.Write(out, binary.LittleEndian, uint8(0))  // reserved
		binary.Write(out, binary.LittleEndian, uint16(1)) // planes
		binary.Write(out, binary.LittleEndian, uint16(32)) // bpp
		binary.Write(out, binary.LittleEndian, uint32(len(dibData[i])))
		binary.Write(out, binary.LittleEndian, offset)
		offset += uint32(len(dibData[i]))
	}

	for _, d := range dibData {
		out.Write(d)
	}

	fmt.Println("Proper ICO created:", len(sizes), "sizes")
}
