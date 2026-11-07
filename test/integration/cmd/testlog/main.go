//go:build integration

// Command testlog turns go test -json on stdin into a line per finished test with its subtests indented, and the failures' output at the end.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// event is one go test -json line; build-output events name their package in ImportPath.
type event struct {
	Action     string
	Package    string
	ImportPath string
	Test       string
	Elapsed    float64
	Output     string
}

// slowest is how many tests the summary lists by duration.
const slowest = 10

// colors are the ANSI codes per result.
var colors = map[string]string{
	"pass": "\x1b[32m",
	"fail": "\x1b[31m",
	"skip": "\x1b[33m",
}

// options are the command's flags plus what main detects about stdout.
type options struct {
	groups bool
	color  bool
}

// formatter prints each top-level test with its subtests indented below it once it finishes and keeps only the output the summary or a CI group needs.
type formatter struct {
	out      io.Writer
	opts     options
	outputs  map[string]*strings.Builder
	logs     map[string]*strings.Builder
	failed   []string
	packages int
	tests    int
	skipped  int
	timings  []timing
	children map[string][]string
	lines    map[string]string
}

// timing is how long one finished test took.
type timing struct {
	key     string
	elapsed float64
}

func newFormatter(out io.Writer, opts options) *formatter {
	return &formatter{
		out:      out,
		opts:     opts,
		outputs:  map[string]*strings.Builder{},
		logs:     map[string]*strings.Builder{},
		children: map[string][]string{},
		lines:    map[string]string{},
	}
}

func (f *formatter) handle(e event) {
	if e.Action == "build-output" {
		e.Action = "output"
		e.Package, _, _ = strings.Cut(e.ImportPath, " ")
	}
	k := key(e.Package, e.Test)
	switch e.Action {
	case "run":
		if top, _, sub := strings.Cut(e.Test, "/"); sub {
			parent := key(e.Package, top)
			f.children[parent] = append(f.children[parent], k)
		}
	case "output":
		buffer(f.outputs, k).WriteString(e.Output)
		if f.opts.groups {
			buffer(f.logs, e.Package).WriteString(e.Output)
		}
	case "pass", "skip", "fail":
		f.finish(e, k)
	}
}

func (f *formatter) finish(e event, k string) {
	if e.Test == "" && e.Action == "skip" {
		// A package without test files (cmd/*, platform) has nothing to report.
		delete(f.outputs, k)
		delete(f.logs, e.Package)
		return
	}
	if e.Test == "" {
		f.packages++
	} else {
		f.tests++
		f.timings = append(f.timings, timing{key: k, elapsed: e.Elapsed})
	}
	if e.Action == "skip" && e.Test != "" {
		f.skipped++
	}
	if e.Action == "fail" {
		f.failed = append(f.failed, k)
	} else {
		delete(f.outputs, k)
	}
	f.printResult(e, k)
	if e.Test == "" && f.opts.groups {
		f.printGroup(e.Package)
	}
}

// printResult holds a subtest's line until its top-level test finishes, then prints that test with its subtests indented
// in the order they ran, so parallel packages and tests can't interleave a tree.
func (f *formatter) printResult(e event, k string) {
	status := f.paint(e.Action, fmt.Sprintf("%-4s", strings.ToUpper(e.Action)))
	if depth := strings.Count(e.Test, "/"); depth > 0 {
		name := e.Test[strings.LastIndex(e.Test, "/")+1:]
		f.lines[k] = fmt.Sprintf("%s%s %s (%.1fs)", strings.Repeat("  ", depth), status, name, e.Elapsed)
		return
	}

	fmt.Fprintf(f.out, "%s %s (%.1fs)\n", status, k, e.Elapsed)
	for _, child := range f.children[k] {
		if line, ok := f.lines[child]; ok {
			fmt.Fprintln(f.out, line)
		}
		delete(f.lines, child)
	}
	delete(f.children, k)
}

func (f *formatter) printGroup(pkg string) {
	fmt.Fprintf(f.out, "::group::%s log\n", path.Base(pkg))
	fmt.Fprint(f.out, buffer(f.logs, pkg).String())
	fmt.Fprintln(f.out, "::endgroup::")
	delete(f.logs, pkg)
}

// summary prints the output of each failure, skipping parents whose subtests failed, and reports success.
func (f *formatter) summary(took time.Duration) bool {
	// Two blank lines set the summary apart from the results streamed during the run
	fmt.Fprintln(f.out)
	if len(f.failed) > 0 {
		fmt.Fprintln(f.out)
		fmt.Fprintln(f.out, "=== Failures")
	}
	for _, k := range f.failed {
		if hasFailedChild(f.failed, k) {
			continue
		}
		fmt.Fprintf(f.out, "%s %s\n", f.paint("fail", "--- FAIL"), k)
		fmt.Fprint(f.out, withoutMarkers(buffer(f.outputs, k).String()))
	}
	f.printSlowest()
	fmt.Fprintln(f.out)
	ok := len(f.failed) == 0 && f.packages > 0
	result := "fail"
	if ok {
		result = "pass"
	}
	done := fmt.Sprintf("DONE %d packages, %d tests, %d skipped, %d failed in %s",
		f.packages, f.tests, f.skipped, len(f.failed), took.Round(time.Second))
	fmt.Fprintln(f.out, f.paint(result, done))
	return ok
}

// printSlowest lists the slowest tests without subtests, as a parent's time only sums its subtests.
func (f *formatter) printSlowest() {
	var leaves []timing
	for _, t := range f.timings {
		if !hasChild(f.timings, t.key) {
			leaves = append(leaves, t)
		}
	}
	sort.SliceStable(leaves, func(i, j int) bool { return leaves[i].elapsed > leaves[j].elapsed })
	if len(leaves) > slowest {
		leaves = leaves[:slowest]
	}
	if len(leaves) == 0 {
		return
	}
	fmt.Fprintln(f.out)
	fmt.Fprintln(f.out, "=== Slowest")
	for _, t := range leaves {
		fmt.Fprintf(f.out, "%7.1fs %s\n", t.elapsed, t.key)
	}
}

// paint colors text by result when color is on.
func (f *formatter) paint(result, text string) string {
	if !f.opts.color {
		return text
	}
	return colors[result] + text + "\x1b[0m"
}

func main() {
	groups := flag.Bool("groups", false, "fold each package's full log into a GitHub Actions group")
	flag.Parse()

	opts := options{
		groups: *groups,
		color:  useColor(),
	}
	if !run(os.Stdin, os.Stdout, opts) {
		os.Exit(1)
	}
}

// run formats go test -json from in; lines that aren't JSON events (go test's own errors) pass through as is.
func run(in io.Reader, out io.Writer, opts options) bool {
	start := time.Now()
	f := newFormatter(out, opts)
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var e event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil || e.Action == "" {
			fmt.Fprintln(out, scanner.Text())
			continue
		}
		f.handle(e)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(out, "testlog: %v\n", err)
		return false
	}
	return f.summary(time.Since(start))
}

// useColor colors a terminal only, never on CI or with NO_COLOR.
func useColor() bool {
	if os.Getenv("CI") != "" {
		return false
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func key(pkg, test string) string {
	return strings.TrimSpace(path.Base(pkg) + " " + test)
}

func buffer(m map[string]*strings.Builder, k string) *strings.Builder {
	if m[k] == nil {
		m[k] = &strings.Builder{}
	}
	return m[k]
}

func hasChild(timings []timing, k string) bool {
	for _, t := range timings {
		if strings.HasPrefix(t.key, k+"/") {
			return true
		}
	}
	return false
}

func hasFailedChild(failed []string, k string) bool {
	for _, f := range failed {
		if strings.HasPrefix(f, k+"/") {
			return true
		}
	}
	return false
}

// withoutMarkers drops go test's === RUN/PAUSE/CONT/NAME lines, which only repeat the test name.
func withoutMarkers(output string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(output, "\n") {
		if !strings.HasPrefix(line, "=== ") {
			b.WriteString(line)
		}
	}
	return b.String()
}
