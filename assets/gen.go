package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const outDir = "."

func main() {
	green := color.RGBA{126, 224, 184, 255}
	bgDark := color.RGBA{17, 19, 24, 255}
	borderC := color.RGBA{42, 45, 56, 255}

	for _, sz := range []int{16, 24, 32, 48, 64, 256} {
		img := image.NewRGBA(image.Rect(0, 0, sz, sz))
		drawC(img, sz, green)
		savePNG(img, fmt.Sprintf("%s/icon-c-%d.png", outDir, sz))
	}

	app := image.NewRGBA(image.Rect(0, 0, 256, 256))
	fillRounded(app, 256, 256, 48, bgDark)
	drawC(app, 256, green)
	strokeRounded(app, 256, 256, 48, 2, borderC)
	savePNG(app, outDir+"/icon-app.png")

	frameSourceIcon(outDir + "/icon-source.png")

	fmt.Println("Done: assets/icon-*.png")
}

// drawC draws a clean anti-aliased "C" letter using distance-field rendering.
func drawC(img *image.RGBA, sz int, c color.RGBA) {
	cx := float64(sz) / 2.0
	r := float64(sz) * 0.30
	hw := float64(sz) * 0.095 // half-width of the stroke
	gap := math.Pi / 5.0       // half-gap angle on the right
	blur := 1.2                 // anti-aliasing range in pixels

	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cx
			dist := math.Sqrt(dx*dx + dy*dy)
			angle := math.Atan2(dy, dx)

			// Signed distance to the C shape (positive = inside, negative = outside)
			ringDist := hw - math.Abs(dist-r) // distance from ring center, positive inside

			if ringDist <= -blur { continue } // far outside

			// Gap check: inside the right-side gap?
			gapDist := math.Abs(angle)
			if gapDist < gap {
				// Distance to nearest gap edge in radians, converted to pixel distance
				gapEdge := (gap - gapDist) * r
				if gapEdge > ringDist {
					ringDist = -gapEdge // negative = outside due to gap
				}
			}

			if ringDist <= -blur { continue }

			alpha := 1.0
			if ringDist < 0 {
				alpha = 1.0 + ringDist/blur // 0..1 transition from -blur to 0
			} else if ringDist < blur {
				alpha = 1.0 // fully opaque in the interior
			}

			if alpha <= 0 { continue }

			existing := img.At(x, y).(color.RGBA)
			img.Set(x, y, color.RGBA{
				uint8(float64(c.R)*alpha + float64(existing.R)*(1-alpha)),
				uint8(float64(c.G)*alpha + float64(existing.G)*(1-alpha)),
				uint8(float64(c.B)*alpha + float64(existing.B)*(1-alpha)),
				uint8(float64(c.A)*alpha + float64(existing.A)*(1-alpha)),
			})
		}
	}

	// Draw rounded tips at both ends of the C arc
	for _, tipAngle := range []float64{math.Pi + gap, math.Pi - gap} {
		tx := cx + r*math.Cos(tipAngle)
		ty := cx + r*math.Sin(tipAngle)
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				dx := float64(x) - tx
				dy := float64(y) - ty
				dd := math.Sqrt(dx*dx + dy*dy)
				if dd <= hw {
					img.Set(x, y, c)
				} else if dd <= hw+blur {
					alpha := 1.0 - (dd-hw)/blur
					existing := img.At(x, y).(color.RGBA)
					img.Set(x, y, color.RGBA{
						uint8(float64(c.R)*alpha + float64(existing.R)*(1-alpha)),
						uint8(float64(c.G)*alpha + float64(existing.G)*(1-alpha)),
						uint8(float64(c.B)*alpha + float64(existing.B)*(1-alpha)),
						uint8(float64(c.A)*alpha + float64(existing.A)*(1-alpha)),
					})
				}
			}
		}
	}
}

func fillRounded(img *image.RGBA, w, h int, r float64, c color.RGBA) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if inRounded(x, y, w, h, r) {
				img.Set(x, y, c)
			}
		}
	}
}

func strokeRounded(img *image.RGBA, w, h int, r float64, thick int, c color.RGBA) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !inRounded(x, y, w, h, r) { continue }
			if inRounded(x-thick, y-thick, w-2*thick, h-2*thick, r-float64(thick)) { continue }
			img.Set(x, y, c)
		}
	}
}

func inRounded(x, y, w, h int, r float64) bool {
	if x < 0 || x >= w || y < 0 || y >= h { return false }
	ri := int(r)
	if x < ri && y < ri { dx := float64(ri-x); dy := float64(ri-y); return dx*dx+dy*dy <= r*r }
	if x >= w-ri && y < ri { dx := float64(x-(w-ri)); dy := float64(ri-y); return dx*dx+dy*dy <= r*r }
	if x < ri && y >= h-ri { dx := float64(ri-x); dy := float64(y-(h-ri)); return dx*dx+dy*dy <= r*r }
	if x >= w-ri && y >= h-ri { dx := float64(x-(w-ri)); dy := float64(y-(h-ri)); return dx*dx+dy*dy <= r*r }
	return true
}

func frameSourceIcon(path string) {
	f, err := os.Open(path)
	if err != nil { return }
	src, err := png.Decode(f)
	f.Close()
	if err != nil { return }

	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	out := image.NewRGBA(image.Rect(0, 0, 256, 256))
	fillRounded(out, 256, 256, 48, color.RGBA{17, 19, 24, 255})

	pad := 30
	area := 256 - 2*pad
	scale := float64(area) / float64(sw)
	for y := 0; y < area; y++ {
		for x := 0; x < area; x++ {
			sx := int(float64(x) / scale)
			sy := int(float64(y) / scale)
			if sx < sw && sy < sh {
				r2, g2, b2, a2 := src.At(sx, sy).RGBA()
				if a2 > 0 {
					out.Set(x+pad, y+pad, color.RGBA{uint8(r2>>8), uint8(g2>>8), uint8(b2>>8), uint8(a2>>8)})
				}
			}
		}
	}
	strokeRounded(out, 256, 256, 48, 2, color.RGBA{42, 45, 56, 255})
	savePNG(out, outDir+"/icon-source-framed.png")
}

func savePNG(img *image.RGBA, path string) {
	f, _ := os.Create(path)
	png.Encode(f, img)
	f.Close()
}

