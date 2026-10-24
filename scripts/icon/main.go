package main

import (
	"bytes"
	"encoding/binary"
	"github.com/tc-hib/winres"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

type point struct{ x, y float64 }

func inside(poly []point, x, y float64) bool {
	hit := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a.y > y) != (b.y > y) && x < (b.x-a.x)*(y-a.y)/(b.y-a.y)+a.x {
			hit = !hit
		}
	}
	return hit
}
func icon(n int) []byte {
	im := image.NewNRGBA(image.Rect(0, 0, n, n))
	poly := []point{{.23, .73}, {.23, .27}, {.33, .27}, {.50, .54}, {.67, .27}, {.77, .27}, {.77, .73}, {.67, .73}, {.67, .45}, {.50, .70}, {.33, .45}, {.33, .73}}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			r, g, b, a := 0, 0, 0, 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					fx := (float64(x) + (float64(sx)+.5)/4) / float64(n)
					fy := (float64(y) + (float64(sy)+.5)/4) / float64(n)
					dx := max(.10-fx, fx-.90, 0)
					dy := max(.10-fy, fy-.90, 0)
					if fx >= .05 && fx <= .95 && fy >= .05 && fy <= .95 && dx*dx+dy*dy <= .0025 {
						a += 255
						if inside(poly, fx, fy) {
							r += 38
							g += 38
							b += 38
						} else {
							r += 250
							g += 250
							b += 250
						}
					}
				}
			}
			im.SetNRGBA(x, y, color.NRGBA{uint8(r / 16), uint8(g / 16), uint8(b / 16), uint8(a / 16)})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, im)
	return buf.Bytes()
}
func main() {
	os.MkdirAll("build/windows", 0755)
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint16(0))
	binary.Write(&out, binary.LittleEndian, uint16(1))
	binary.Write(&out, binary.LittleEndian, uint16(len(sizes)))
	images := [][]byte{}
	offset := 6 + 16*len(sizes)
	for _, size := range sizes {
		b := icon(size)
		images = append(images, b)
		out.Write([]byte{byte(size % 256), byte(size % 256), 0, 0})
		binary.Write(&out, binary.LittleEndian, uint16(1))
		binary.Write(&out, binary.LittleEndian, uint16(32))
		binary.Write(&out, binary.LittleEndian, uint32(len(b)))
		binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(b)
	}
	for _, b := range images {
		out.Write(b)
	}
	os.WriteFile(filepath.Join("build", "windows", "icon.ico"), out.Bytes(), 0644)
	os.WriteFile(filepath.Join("build", "appicon.png"), icon(256), 0644)
	ico, e := winres.LoadICO(bytes.NewReader(out.Bytes()))
	if e != nil {
		panic(e)
	}
	rs := winres.ResourceSet{}
	if e = rs.SetIcon(winres.ID(32512), ico); e != nil {
		panic(e)
	}
	manifest, e := os.ReadFile("build/windows/app.manifest")
	if e != nil {
		panic(e)
	}
	if e = rs.Set(winres.RT_MANIFEST, winres.ID(1), 0, manifest); e != nil {
		panic(e)
	}
	f, e := os.Create("app_windows_amd64.syso")
	if e != nil {
		panic(e)
	}
	defer f.Close()
	if e = rs.WriteObject(f, winres.ArchAMD64); e != nil {
		panic(e)
	}
}
