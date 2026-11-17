// Package mesh reads a bed mesh out of text a printer printed on its serial
// console, and describes it.
//
// The text is whatever a person copied out of a console after asking the
// printer for its mesh. It may carry the console's own prefixes (Recv:,
// echo:, timestamps), the lines before and after the map, and the `ok`
// acknowledgements. Parse finds the map in it.
//
// # Formats
//
//   - ubl: the human-readable "Bed Topography Report" Marlin's Unified Bed
//     Leveling prints for `M420 V` and `G29 T`. Prusa's Buddy firmware (MK4,
//     MK3.5, MK3.9, Core One, XL, MINI) prints this one. Rows carry their Y
//     index, the back row comes first, "." is a point that holds no value, and
//     the corners carry the bed coordinates of the mesh.
//   - ubl-csv: the same report as `M420 V T1` prints it, tab separated, back
//     row first, NAN for a point with no value.
//
// Only printers with Buddy (xBuddy) firmware connect to Prusa Connect Local,
// so these two are the only formats read.
//
// # Orientation
//
// A Mesh always holds its rows front to back: Z[0] is the row at the lowest Y,
// and Z[j][0] is the point at the lowest X of that row.
package mesh

import (
	"errors"
	"math"
)

// The limits of a mesh this package accepts. The largest Prusa grid is the
// XL's 36 by 36. Anything well past that is not a bed mesh.
const (
	MinPoints = 2
	MaxPoints = 64
	// MaxInput is the longest text Parse reads, in bytes.
	MaxInput = 256 << 10
)

// The reasons Parse refuses a text. Callers inspect them with errors.Is.
var (
	ErrEmpty    = errors.New("mesh: the text is empty")
	ErrTooLarge = errors.New("mesh: the text is longer than the parser reads")
	ErrNoMesh   = errors.New("mesh: the text holds no mesh")
)

// Format names the console output a mesh was read from.
type Format string

// The formats Parse recognises. The package documentation describes each.
const (
	FormatUBL      Format = "ubl"
	FormatUBLCSV   Format = "ubl-csv"
)

// Bounds is the area of the bed the mesh covers, in millimetres, when the
// printer printed it.
type Bounds struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
}

// Mesh is one measured bed.
type Mesh struct {
	Format Format `json:"format"`
	// Cols is the number of points along X, Rows along Y.
	Cols int `json:"cols"`
	Rows int `json:"rows"`
	// Z holds the heights in millimetres, front row first. A nil is a point
	// the printer holds no value for.
	Z [][]*float64 `json:"z"`
	// Bounds is present when the printer printed the corners of the mesh.
	Bounds *Bounds `json:"bounds,omitempty"`
}

// Stats describes the heights of a mesh.
type Stats struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	// Range is Max minus Min: how far the highest point is above the lowest.
	Range float64 `json:"range"`
	Mean  float64 `json:"mean"`
	// StdDev is the population standard deviation of the heights.
	StdDev float64 `json:"std_dev"`
	// Points is how many points hold a value; Missing how many do not.
	Points  int `json:"points"`
	Missing int `json:"missing"`
}

// Stats computes the statistics of the points that hold a value.
func (m Mesh) Stats() Stats {
	stats := Stats{Min: math.Inf(1), Max: math.Inf(-1)}

	sum := 0.0

	for _, row := range m.Z {
		for _, z := range row {
			if z == nil {
				stats.Missing++

				continue
			}

			stats.Points++
			sum += *z
			stats.Min = math.Min(stats.Min, *z)
			stats.Max = math.Max(stats.Max, *z)
		}
	}

	if stats.Points == 0 {
		return Stats{Missing: stats.Missing}
	}

	stats.Mean = sum / float64(stats.Points)
	stats.Range = stats.Max - stats.Min

	squares := 0.0

	for _, row := range m.Z {
		for _, z := range row {
			if z != nil {
				squares += (*z - stats.Mean) * (*z - stats.Mean)
			}
		}
	}

	stats.StdDev = math.Sqrt(squares / float64(stats.Points))

	for _, value := range []*float64{&stats.Min, &stats.Max, &stats.Range, &stats.Mean, &stats.StdDev} {
		*value = round(*value, 4)
	}

	return stats
}

// FlipY returns the mesh with its rows in the other order. Both reports print
// the back row first.
func (m Mesh) FlipY() Mesh {
	flipped := m
	flipped.Z = make([][]*float64, len(m.Z))

	for j, row := range m.Z {
		flipped.Z[len(m.Z)-1-j] = row
	}

	return flipped
}

func round(value float64, places int) float64 {
	scale := math.Pow(10, float64(places))

	return math.Round(value*scale) / scale
}
