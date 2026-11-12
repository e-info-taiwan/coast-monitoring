package report

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"coast-monitoring/internal/service"
)

func TestReportFileModificationTimes(t *testing.T) {
	before := time.Now().UTC().Truncate(time.Second)
	for _, format := range []string{"", "headquarters", "taibif", "internal"} {
		raw, err := FormatArchive([]service.ReefDataDetail{templateFixture()}, format)
		if err != nil {
			t.Fatal(err)
		}
		archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range archive.File {
			if file.Modified.Before(before) || file.Modified.After(time.Now()) {
				t.Fatalf("%s: wrong modification time %v", file.Name, file.Modified)
			}
			if !strings.HasSuffix(file.Name, ".xlsx") {
				continue
			}
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			workbook, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			parts, err := zip.NewReader(bytes.NewReader(workbook), int64(len(workbook)))
			if err != nil {
				t.Fatal(err)
			}
			for _, part := range parts.File {
				if part.Modified.Before(before) || part.Modified.After(time.Now()) {
					t.Fatalf("%s/%s: wrong modification time %v", file.Name, part.Name, part.Modified)
				}
			}
		}
	}
}
