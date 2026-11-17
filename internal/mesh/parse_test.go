package mesh

import (
	"errors"
	"strings"
	"testing"
)

// ublReport is what Buddy firmware prints for `M420 V` on a 3 by 3 grid,
// spaced the way ubl.cpp spaces it, with the head over the middle of the back
// row and one point that holds no value.
const ublReport = `
echo:Bed Topography Report:

    (  2,206)              (242,206)
        0      1      2
 2 | +0.100  -0.050 [ 0.000]
   |
 1 |   .     +0.020  -0.010
   |
 0 | -0.200  +0.000  +0.030
        0      1      2
    (  2, -4)              (242, -4)

echo:Bed Leveling ON
echo:Fade Height 10.00
ok
`

func TestAUBLReportReadsFrontRowFirst(t *testing.T) {
	mesh, err := Parse(ublReport)
	if err != nil {
		t.Fatal(err)
	}

	if mesh.Format != FormatUBL || mesh.Cols != 3 || mesh.Rows != 3 {
		t.Fatalf("got %s %dx%d", mesh.Format, mesh.Cols, mesh.Rows)
	}

	want := [][]string{
		{"-0.2", "0", "0.03"},
		{"nil", "0.02", "-0.01"},
		{"0.1", "-0.05", "0"},
	}
	assertGrid(t, mesh, want)

	if mesh.Bounds == nil || *mesh.Bounds != (Bounds{MinX: 2, MinY: -4, MaxX: 242, MaxY: 206}) {
		t.Fatalf("bounds %+v", mesh.Bounds)
	}
}

func TestAConsolesPrefixesAreIgnored(t *testing.T) {
	var prefixed strings.Builder

	for _, line := range strings.Split(ublReport, "\n") {
		prefixed.WriteString("Recv: " + line + "\n")
	}

	mesh, err := Parse(prefixed.String())
	if err != nil {
		t.Fatal(err)
	}

	if mesh.Format != FormatUBL || mesh.Rows != 3 {
		t.Fatalf("got %s with %d rows", mesh.Format, mesh.Rows)
	}

	timestamped := strings.ReplaceAll(ublReport, "\n ", "\n12:01:02.345 > ")

	if _, err = Parse(timestamped); err != nil {
		t.Fatalf("a timestamped log: %v", err)
	}
}

func TestOnlyTheFirstOfTwoReportsIsRead(t *testing.T) {
	second := strings.ReplaceAll(ublReport, "+0.100", "+0.900")

	mesh, err := Parse(ublReport + second)
	if err != nil {
		t.Fatal(err)
	}

	if got := *mesh.Z[2][0]; got != 0.1 {
		t.Fatalf("back-left is %v, want the first report's 0.1", got)
	}
}

func TestACSVReportReadsFrontRowFirst(t *testing.T) {
	mesh, err := Parse("Bed Topography Report for CSV:\n\n0.100\t-0.050\t0.000\nNAN\t0.020\t-0.010\n-0.200\t0.000\t0.030\n")
	if err != nil {
		t.Fatal(err)
	}

	if mesh.Format != FormatUBLCSV {
		t.Fatalf("format %s", mesh.Format)
	}

	assertGrid(t, mesh, [][]string{
		{"-0.2", "0", "0.03"},
		{"nil", "0.02", "-0.01"},
		{"0.1", "-0.05", "0"},
	})
}

func TestStats(t *testing.T) {
	mesh, err := Parse(ublReport)
	if err != nil {
		t.Fatal(err)
	}

	stats := mesh.Stats()
	if stats.Points != 8 || stats.Missing != 1 || stats.Min != -0.2 || stats.Max != 0.1 || stats.Range != 0.3 {
		t.Fatalf("stats %+v", stats)
	}
}

func TestRefusals(t *testing.T) {
	for name, test := range map[string]struct {
		text string
		want error
	}{
		"empty":            {"  \n ok \n", ErrEmpty},
		"words":            {"echo:Invalid mesh.\nok\n", ErrNoMesh},
		"numbers alone":    {"0.1 0.2 0.3\n0.1 0.2 0.3\n", ErrNoMesh},
		"every point NAN":  {"Bed Topography Report for CSV:\nNAN\tNAN\nNAN\tNAN\n", ErrNoMesh},
		"ragged ubl":       {"Bed Topography Report:\n 1 | +0.1 +0.2\n 0 | +0.1\n", ErrNoMesh},
		"too long a paste": {strings.Repeat("0.1 ", MaxInput), ErrTooLarge},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(test.text); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func assertGrid(t *testing.T, mesh Mesh, want [][]string) {
	t.Helper()

	for j, row := range want {
		for i, cell := range row {
			got := "nil"
			if z := mesh.Z[j][i]; z != nil {
				got = strings.TrimSuffix(strings.TrimRight(formatFloat(*z), "0"), ".")
			}

			if got != cell && !(cell == "0" && got == "") {
				t.Errorf("Z[%d][%d] = %s, want %s", j, i, got, cell)
			}
		}
	}
}
