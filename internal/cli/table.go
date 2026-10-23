package cli

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
)

// writeTable prints rows in aligned columns, one per string field tagged `table:"HEADER"`, with a dash for each empty
// cell.
func writeTable[T any](out io.Writer, rows []T) error {
	typ := reflect.TypeFor[T]()

	var header []string
	var columns []int
	for i := range typ.NumField() {
		if name := typ.Field(i).Tag.Get("table"); name != "" {
			header = append(header, name)
			columns = append(columns, i)
		}
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, strings.Join(header, "\t")); err != nil {
		return err
	}

	for _, row := range rows {
		v := reflect.ValueOf(row)
		cells := make([]string, len(columns))
		for i, column := range columns {
			cells[i] = orDash(v.Field(column).String())
		}
		if _, err := fmt.Fprintln(w, strings.Join(cells, "\t")); err != nil {
			return err
		}
	}

	return w.Flush()
}

// orDash returns s, or "-" when it is empty.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
