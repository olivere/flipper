package layout

import "strconv"

// Template is a named set of regions that partition a rectangular area.
type Template struct {
	Regions []Region
}

// Region is a named rectangular area within a template.
type Region struct {
	Name string
	Rect Rect
}

// Region returns the Rect for the named region, or an empty Rect
// if the name is not found.
func (t Template) Region(name string) Rect {
	for _, r := range t.Regions {
		if r.Name == name {
			return r.Rect
		}
	}
	return Rect{}
}

const defaultGap = 4

// Full returns a single region covering the entire bounds.
//
//	┌──────────────┐
//	│    content    │
//	└──────────────┘
func Full(bounds Rect) Template {
	return Template{
		Regions: []Region{
			{Name: "content", Rect: bounds},
		},
	}
}

// HalfVertical splits the bounds into two equal-width columns.
//
//	┌──────┬───────┐
//	│ left │ right │
//	└──────┴───────┘
func HalfVertical(bounds Rect) Template {
	cols := bounds.Columns(2, defaultGap)
	return Template{
		Regions: []Region{
			{Name: "left", Rect: cols[0]},
			{Name: "right", Rect: cols[1]},
		},
	}
}

// HalfHorizontal splits the bounds into two equal-height rows.
//
//	┌──────────────┐
//	│     top      │
//	├──────────────┤
//	│    bottom    │
//	└──────────────┘
func HalfHorizontal(bounds Rect) Template {
	rows := bounds.Rows(2, defaultGap)
	return Template{
		Regions: []Region{
			{Name: "top", Rect: rows[0]},
			{Name: "bottom", Rect: rows[1]},
		},
	}
}

// Rows3 splits the bounds into three equal-height rows.
//
//	┌──────────────┐
//	│     top      │
//	├──────────────┤
//	│    middle    │
//	├──────────────┤
//	│    bottom    │
//	└──────────────┘
func Rows3(bounds Rect) Template {
	rows := bounds.Rows(3, defaultGap)
	return Template{
		Regions: []Region{
			{Name: "top", Rect: rows[0]},
			{Name: "middle", Rect: rows[1]},
			{Name: "bottom", Rect: rows[2]},
		},
	}
}

// Quadrant splits the bounds into four equal quadrants.
//
//	┌──────┬───────┐
//	│  tl  │  tr   │
//	├──────┼───────┤
//	│  bl  │  br   │
//	└──────┴───────┘
func Quadrant(bounds Rect) Template {
	rows := bounds.Rows(2, defaultGap)
	topCols := rows[0].Columns(2, defaultGap)
	botCols := rows[1].Columns(2, defaultGap)
	return Template{
		Regions: []Region{
			{Name: "top-left", Rect: topCols[0]},
			{Name: "top-right", Rect: topCols[1]},
			{Name: "bottom-left", Rect: botCols[0]},
			{Name: "bottom-right", Rect: botCols[1]},
		},
	}
}

// LeftStack splits the bounds into a left column and two stacked
// rows on the right.
//
//	┌──────┬───────┐
//	│      │  tr   │
//	│ left ├───────┤
//	│      │  br   │
//	└──────┴───────┘
func LeftStack(bounds Rect) Template {
	cols := bounds.Columns(2, defaultGap)
	rightRows := cols[1].Rows(2, defaultGap)
	return Template{
		Regions: []Region{
			{Name: "left", Rect: cols[0]},
			{Name: "top-right", Rect: rightRows[0]},
			{Name: "bottom-right", Rect: rightRows[1]},
		},
	}
}

// TopSplit splits the bounds into two columns on top and a full-width
// row on the bottom.
//
//	┌──────┬───────┐
//	│  tl  │  tr   │
//	├──────┴───────┤
//	│    bottom    │
//	└──────────────┘
func TopSplit(bounds Rect) Template {
	rows := bounds.Rows(2, defaultGap)
	topCols := rows[0].Columns(2, defaultGap)
	return Template{
		Regions: []Region{
			{Name: "top-left", Rect: topCols[0]},
			{Name: "top-right", Rect: topCols[1]},
			{Name: "bottom", Rect: rows[1]},
		},
	}
}

// RightStack splits the bounds into two stacked rows on the left
// and a single right column.
//
//	┌──────┬───────┐
//	│  tl  │       │
//	├──────┤ right │
//	│  bl  │       │
//	└──────┴───────┘
func RightStack(bounds Rect) Template {
	cols := bounds.Columns(2, defaultGap)
	leftRows := cols[0].Rows(2, defaultGap)
	return Template{
		Regions: []Region{
			{Name: "top-left", Rect: leftRows[0]},
			{Name: "bottom-left", Rect: leftRows[1]},
			{Name: "right", Rect: cols[1]},
		},
	}
}

// BottomSplit splits the bounds into a full-width top row and two
// columns on the bottom.
//
//	┌──────────────┐
//	│     top      │
//	├──────┬───────┤
//	│  bl  │  br   │
//	└──────┴───────┘
func BottomSplit(bounds Rect) Template {
	rows := bounds.Rows(2, defaultGap)
	botCols := rows[1].Columns(2, defaultGap)
	return Template{
		Regions: []Region{
			{Name: "top", Rect: rows[0]},
			{Name: "bottom-left", Rect: botCols[0]},
			{Name: "bottom-right", Rect: botCols[1]},
		},
	}
}

// Grid2x3 splits the bounds into a 2-row by 3-column grid.
// Regions are named "r0c0" through "r1c2".
//
//	┌────┬────┬────┐
//	│0,0 │0,1 │0,2 │
//	├────┼────┼────┤
//	│1,0 │1,1 │1,2 │
//	└────┴────┴────┘
func Grid2x3(bounds Rect) Template {
	rows := bounds.Rows(2, defaultGap)
	var regions []Region
	for r, row := range rows {
		cols := row.Columns(3, defaultGap)
		for col, rect := range cols {
			name := "r" + itoa(r) + "c" + itoa(col)
			regions = append(regions, Region{Name: name, Rect: rect})
		}
	}
	return Template{Regions: regions}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
