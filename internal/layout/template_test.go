package layout

import (
	"testing"
)

var ogBounds = Rect{X: 0, Y: 0, W: 800, H: 480}

func TestFullTemplate(t *testing.T) {
	tmpl := Full(ogBounds)
	if len(tmpl.Regions) != 1 {
		t.Fatalf("Full: got %d regions, want 1", len(tmpl.Regions))
	}
	if tmpl.Region("content") != ogBounds {
		t.Errorf("Full content region = %v, want %v", tmpl.Region("content"), ogBounds)
	}
}

func TestHalfVerticalTemplate(t *testing.T) {
	tmpl := HalfVertical(ogBounds)
	if len(tmpl.Regions) != 2 {
		t.Fatalf("HalfVertical: got %d regions, want 2", len(tmpl.Regions))
	}

	left := tmpl.Region("left")
	right := tmpl.Region("right")

	if left.X != 0 {
		t.Errorf("left X = %d, want 0", left.X)
	}
	if right.X <= left.X+left.W-1 {
		t.Errorf("right should be to the right of left: right.X=%d, left end=%d", right.X, left.X+left.W)
	}
	// Both should span the full height
	if left.H != ogBounds.H || right.H != ogBounds.H {
		t.Errorf("columns should span full height: left.H=%d, right.H=%d, bounds.H=%d", left.H, right.H, ogBounds.H)
	}
}

func TestHalfHorizontalTemplate(t *testing.T) {
	tmpl := HalfHorizontal(ogBounds)
	if len(tmpl.Regions) != 2 {
		t.Fatalf("HalfHorizontal: got %d regions, want 2", len(tmpl.Regions))
	}

	top := tmpl.Region("top")
	bottom := tmpl.Region("bottom")

	if top.W != ogBounds.W || bottom.W != ogBounds.W {
		t.Errorf("rows should span full width: top.W=%d, bottom.W=%d", top.W, bottom.W)
	}
	if bottom.Y <= top.Y {
		t.Errorf("bottom should be below top: bottom.Y=%d, top.Y=%d", bottom.Y, top.Y)
	}
}

func TestQuadrantTemplate(t *testing.T) {
	tmpl := Quadrant(ogBounds)
	if len(tmpl.Regions) != 4 {
		t.Fatalf("Quadrant: got %d regions, want 4", len(tmpl.Regions))
	}

	names := []string{"top-left", "top-right", "bottom-left", "bottom-right"}
	for _, name := range names {
		r := tmpl.Region(name)
		if r.W == 0 || r.H == 0 {
			t.Errorf("Quadrant region %q has zero dimension: %v", name, r)
		}
	}
}

func TestLeftStackTemplate(t *testing.T) {
	tmpl := LeftStack(ogBounds)
	if len(tmpl.Regions) != 3 {
		t.Fatalf("LeftStack: got %d regions, want 3", len(tmpl.Regions))
	}

	left := tmpl.Region("left")
	tr := tmpl.Region("top-right")
	br := tmpl.Region("bottom-right")

	// Left spans full height
	if left.H != ogBounds.H {
		t.Errorf("left should span full height: got %d, want %d", left.H, ogBounds.H)
	}
	// Right regions are stacked
	if br.Y <= tr.Y {
		t.Errorf("bottom-right should be below top-right")
	}
}

func TestTopSplitTemplate(t *testing.T) {
	tmpl := TopSplit(ogBounds)
	if len(tmpl.Regions) != 3 {
		t.Fatalf("TopSplit: got %d regions, want 3", len(tmpl.Regions))
	}

	bottom := tmpl.Region("bottom")
	if bottom.W != ogBounds.W {
		t.Errorf("bottom should span full width: got %d, want %d", bottom.W, ogBounds.W)
	}
}

func TestGrid2x3Template(t *testing.T) {
	tmpl := Grid2x3(ogBounds)
	if len(tmpl.Regions) != 6 {
		t.Fatalf("Grid2x3: got %d regions, want 6", len(tmpl.Regions))
	}

	// Check all cells exist
	for r := range 2 {
		for col := range 3 {
			name := "r" + string(rune('0'+r)) + "c" + string(rune('0'+col))
			rect := tmpl.Region(name)
			if rect.W == 0 || rect.H == 0 {
				t.Errorf("Grid2x3 region %q has zero dimension", name)
			}
		}
	}
}

func TestRightStackTemplate(t *testing.T) {
	tmpl := RightStack(ogBounds)
	if len(tmpl.Regions) != 3 {
		t.Fatalf("RightStack: got %d regions, want 3", len(tmpl.Regions))
	}

	right := tmpl.Region("right")
	tl := tmpl.Region("top-left")
	bl := tmpl.Region("bottom-left")

	// Right spans full height
	if right.H != ogBounds.H {
		t.Errorf("right should span full height: got %d, want %d", right.H, ogBounds.H)
	}
	// Left regions are stacked
	if bl.Y <= tl.Y {
		t.Errorf("bottom-left should be below top-left")
	}
}

func TestBottomSplitTemplate(t *testing.T) {
	tmpl := BottomSplit(ogBounds)
	if len(tmpl.Regions) != 3 {
		t.Fatalf("BottomSplit: got %d regions, want 3", len(tmpl.Regions))
	}

	top := tmpl.Region("top")
	if top.W != ogBounds.W {
		t.Errorf("top should span full width: got %d, want %d", top.W, ogBounds.W)
	}

	bl := tmpl.Region("bottom-left")
	br := tmpl.Region("bottom-right")
	if br.X <= bl.X {
		t.Errorf("bottom-right should be to the right of bottom-left")
	}
}

func TestRegionNotFound(t *testing.T) {
	tmpl := Full(ogBounds)
	r := tmpl.Region("nonexistent")
	if r != (Rect{}) {
		t.Errorf("Region for unknown name should be zero Rect, got %v", r)
	}
}

func TestRows3Template(t *testing.T) {
	tmpl := Rows3(ogBounds)
	if len(tmpl.Regions) != 3 {
		t.Fatalf("Rows3: got %d regions, want 3", len(tmpl.Regions))
	}

	top := tmpl.Region("top")
	middle := tmpl.Region("middle")
	bottom := tmpl.Region("bottom")

	if middle.Y <= top.Y {
		t.Error("middle should be below top")
	}
	if bottom.Y <= middle.Y {
		t.Error("bottom should be below middle")
	}
}
