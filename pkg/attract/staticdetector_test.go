package attract

import "testing"

// pixelAt's word reads must give the same bytes as reading each one, at
// every alignment, up to the end of the memory.
func TestPixelAt(t *testing.T) {
	m := make([]byte, 64)
	for i := range m {
		m[i] = byte(i*37 + 11)
	}
	for i := 0; i+2 < len(m); i++ {
		r, g, b := pixelAt(m, i)
		if r != m[i] || g != m[i+1] || b != m[i+2] {
			t.Fatalf("at %d: got %d %d %d, want %d %d %d", i, r, g, b, m[i], m[i+1], m[i+2])
		}
	}
}

// Cells built from columnCells and a row part must match working out the
// whole cell for each pixel.
func TestColumnCells(t *testing.T) {
	var c columnCells
	for _, size := range [][2]int{{320, 240}, {1508, 286}, {256, 224}, {720, 576}} {
		w, h := size[0], size[1]
		step := sampleStep(w, h)
		for _, off := range sampleOffsets {
			ox, oy := scaleOffset(off[0], step), scaleOffset(off[1], step)
			cols := c.get(w, step, ox)
			for y := oy; y < h; y += step {
				rowCell := uint16((y * gridRows / h) * gridCols)
				for xi, x := 0, ox; x < w; xi, x = xi+1, x+step {
					want := uint16((y*gridRows/h)*gridCols + x*gridCols/w)
					if got := rowCell + cols[xi]; got != want {
						t.Fatalf("%dx%d at %d,%d: cell %d, want %d", w, h, x, y, got, want)
					}
				}
			}
		}
	}
}
