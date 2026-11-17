package mesh

import (
	"os"
	"testing"
)

// TestTheSampleInTheDocsParses holds docs/sample-m420v-mk4.txt, the 21 by 21
// report the manual offers for trying the extension without a printer, to the
// parser.
func TestTheSampleInTheDocsParses(t *testing.T) {
	text, err := os.ReadFile("../../docs/sample-m420v-mk4.txt")
	if err != nil {
		t.Fatal(err)
	}

	mesh, err := Parse(string(text))
	if err != nil {
		t.Fatal(err)
	}

	if mesh.Format != FormatUBL || mesh.Cols != 21 || mesh.Rows != 21 || mesh.Bounds == nil || mesh.Stats().Missing != 0 {
		t.Fatalf("parsed %s %dx%d bounds %+v", mesh.Format, mesh.Cols, mesh.Rows, mesh.Bounds)
	}
}
