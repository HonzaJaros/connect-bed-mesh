package mesh

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// consolePrefix is what a console puts in front of a line the printer sent:
// OctoPrint's Recv:/Send:, Pronterface's SENDING:, arrows, a timestamp, and
// Marlin's own echo: and // in front of a line.
var consolePrefix = regexp.MustCompile(
	`^\s*(?:\[?\d{1,2}:\d{2}(?::\d{2}(?:[.,]\d+)?)?\]?\s*)?(?:(?:Recv|Send|SENDING|RECV|<<<|>>>|<|>|echo|//)\s*:?\s*)*`)

// corner is one "(x,y)" corner label of a UBL report.
var corner = regexp.MustCompile(`\(\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*\)`)

// ublRow is one row of a UBL report: its Y index, a bar, and the values.
var ublRow = regexp.MustCompile(`^\s*(\d+)\s*\|(.*)$`)

// Parse finds the bed mesh in text a printer printed.
func Parse(text string) (Mesh, error) {
	if len(text) > MaxInput {
		return Mesh{}, ErrTooLarge
	}

	lines := clean(text)
	if len(lines) == 0 {
		return Mesh{}, ErrEmpty
	}

	for _, parse := range []func([]string) (Mesh, bool){parseUBL, parseUBLCSV} {
		if mesh, ok := parse(lines); ok {
			if err := mesh.check(); err != nil {
				return Mesh{}, err
			}

			return mesh, nil
		}
	}

	return Mesh{}, ErrNoMesh
}

// clean splits the text into lines without the console's prefixes, and
// without the acknowledgements a printer sends between them.
func clean(text string) []string {
	var lines []string

	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		line = consolePrefix.ReplaceAllString(line, "")
		line = strings.TrimRight(line, " \t")

		if strings.TrimSpace(line) == "" || strings.TrimSpace(line) == "ok" {
			continue
		}

		lines = append(lines, line)
	}

	return lines
}

// parseUBL reads the human-readable Bed Topography Report.
func parseUBL(lines []string) (Mesh, bool) {
	start := indexOf(lines, func(line string) bool {
		return strings.Contains(line, "Bed Topography Report") && !strings.Contains(line, " for ")
	})
	if start < 0 {
		return Mesh{}, false
	}

	rows := map[int][]*float64{}
	top := -1
	var corners [][2]float64

	for _, line := range lines[start+1:] {
		if match := ublRow.FindStringSubmatch(line); match != nil {
			index, _ := strconv.Atoi(match[1])

			values, ok := ublValues(match[2])
			if !ok {
				break
			}

			if _, seen := rows[index]; seen {
				break // a second report starts here
			}

			rows[index] = values
			top = max(top, index)

			continue
		}

		for _, found := range corner.FindAllStringSubmatch(line, -1) {
			x, _ := strconv.ParseFloat(found[1], 64)
			y, _ := strconv.ParseFloat(found[2], 64)
			corners = append(corners, [2]float64{x, y})
		}

		if len(rows) > 0 && len(corners) >= 4 {
			break
		}
	}

	if len(rows) == 0 || len(rows) != top+1 {
		return Mesh{}, false
	}

	mesh := Mesh{Format: FormatUBL, Rows: len(rows), Cols: len(rows[0])}

	for j := 0; j <= top; j++ {
		mesh.Z = append(mesh.Z, rows[j])
	}

	if len(corners) >= 4 {
		mesh.Bounds = boundsOf(corners)
	}

	return mesh, true
}

// ublValues reads the values of one UBL row: numbers, "." for a point with no
// value, and brackets around the point the head is over.
func ublValues(text string) ([]*float64, bool) {
	text = strings.NewReplacer("[", " ", "]", " ").Replace(text)

	var values []*float64

	for _, token := range strings.Fields(text) {
		if token == "." {
			values = append(values, nil)

			continue
		}

		value, ok := number(token)
		if !ok {
			return nil, false
		}

		values = append(values, value)
	}

	return values, len(values) > 0
}

// parseUBLCSV reads the report `M420 V T1` prints.
func parseUBLCSV(lines []string) (Mesh, bool) {
	start := indexOf(lines, func(line string) bool {
		return strings.Contains(line, "Bed Topography Report for CSV")
	})
	if start < 0 {
		return Mesh{}, false
	}

	var rows [][]*float64

	for _, line := range lines[start+1:] {
		values, ok := numbers(strings.ReplaceAll(line, ",", " "))
		if !ok {
			break
		}

		rows = append(rows, values)
	}

	if len(rows) == 0 {
		return Mesh{}, false
	}

	mesh := Mesh{Format: FormatUBLCSV, Rows: len(rows), Cols: len(rows[0]), Z: rows}

	return mesh.FlipY(), true
}

// numbers reads a line made of numbers and nothing else.
func numbers(line string) ([]*float64, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, false
	}

	values := make([]*float64, 0, len(fields))

	for _, field := range fields {
		value, ok := number(field)
		if !ok {
			return nil, false
		}

		values = append(values, value)
	}

	return values, true
}

// number reads one value. NAN is a point with no value, which is a nil.
func number(token string) (*float64, bool) {
	if strings.EqualFold(token, "nan") {
		return nil, true
	}

	value, err := strconv.ParseFloat(token, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return nil, false
	}

	return &value, true
}

func indexOf(lines []string, match func(string) bool) int {
	for i, line := range lines {
		if match(line) {
			return i
		}
	}

	return -1
}

func boundsOf(corners [][2]float64) *Bounds {
	bounds := &Bounds{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}

	for _, c := range corners {
		bounds.MinX = math.Min(bounds.MinX, c[0])
		bounds.MaxX = math.Max(bounds.MaxX, c[0])
		bounds.MinY = math.Min(bounds.MinY, c[1])
		bounds.MaxY = math.Max(bounds.MaxY, c[1])
	}

	if bounds.MinX == bounds.MaxX || bounds.MinY == bounds.MaxY {
		return nil
	}

	return bounds
}

// check holds a mesh to the shape every format promises.
func (m Mesh) check() error {
	if m.Cols < MinPoints || m.Rows < MinPoints || m.Cols > MaxPoints || m.Rows > MaxPoints {
		return fmt.Errorf("%w: a mesh of %d by %d points is outside %d to %d",
			ErrNoMesh, m.Cols, m.Rows, MinPoints, MaxPoints)
	}

	for j, row := range m.Z {
		if len(row) != m.Cols {
			return fmt.Errorf("%w: row %d has %d points and the first has %d", ErrNoMesh, j, len(row), m.Cols)
		}
	}

	if m.Stats().Points == 0 {
		return fmt.Errorf("%w: no point holds a value", ErrNoMesh)
	}

	return nil
}
