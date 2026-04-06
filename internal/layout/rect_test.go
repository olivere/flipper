package layout

import (
	"testing"
)

func TestRectInsetAll(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 800, H: 480}
	got := r.InsetAll(10)
	want := Rect{X: 10, Y: 10, W: 780, H: 460}
	if got != want {
		t.Errorf("InsetAll(10) = %v, want %v", got, want)
	}
}

func TestRectInset(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 800, H: 480}
	got := r.Inset(10, 20, 30, 40)
	want := Rect{X: 40, Y: 10, W: 740, H: 440}
	if got != want {
		t.Errorf("Inset(10,20,30,40) = %v, want %v", got, want)
	}
}

func TestRectSplitH(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 800, H: 480}
	top, bottom := r.SplitH(100)

	if top.H != 100 {
		t.Errorf("top height = %d, want 100", top.H)
	}
	if bottom.Y != 100 {
		t.Errorf("bottom Y = %d, want 100", bottom.Y)
	}
	if bottom.H != 380 {
		t.Errorf("bottom height = %d, want 380", bottom.H)
	}
}

func TestRectSplitV(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 800, H: 480}
	left, right := r.SplitV(300)

	if left.W != 300 {
		t.Errorf("left width = %d, want 300", left.W)
	}
	if right.X != 300 {
		t.Errorf("right X = %d, want 300", right.X)
	}
	if right.W != 500 {
		t.Errorf("right width = %d, want 500", right.W)
	}
}

func TestRectColumns(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 800, H: 480}
	cols := r.Columns(3, 10)

	if len(cols) != 3 {
		t.Fatalf("got %d columns, want 3", len(cols))
	}

	// Each column: (800 - 20) / 3 = 260
	for i, col := range cols {
		if col.W != 260 {
			t.Errorf("column %d width = %d, want 260", i, col.W)
		}
		if col.H != 480 {
			t.Errorf("column %d height = %d, want 480", i, col.H)
		}
	}

	// Check gaps
	if cols[1].X != cols[0].X+260+10 {
		t.Errorf("column 1 X = %d, want %d", cols[1].X, cols[0].X+260+10)
	}
}

func TestRectRows(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 800, H: 480}
	rows := r.Rows(2, 10)

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	// Each row: (480 - 10) / 2 = 235
	for i, row := range rows {
		if row.H != 235 {
			t.Errorf("row %d height = %d, want 235", i, row.H)
		}
	}

	if rows[1].Y != rows[0].Y+235+10 {
		t.Errorf("row 1 Y = %d, want %d", rows[1].Y, rows[0].Y+235+10)
	}
}

func TestRectColumnsZero(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 800, H: 480}
	cols := r.Columns(0, 10)
	if cols != nil {
		t.Errorf("Columns(0) should return nil, got %v", cols)
	}
}

func TestRectInsetWithOffset(t *testing.T) {
	r := Rect{X: 100, Y: 50, W: 600, H: 400}
	got := r.InsetAll(20)
	want := Rect{X: 120, Y: 70, W: 560, H: 360}
	if got != want {
		t.Errorf("InsetAll on offset rect = %v, want %v", got, want)
	}
}
