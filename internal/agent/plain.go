package agent

import "strings"

// plainAdapter preserves the historical stdout behavior for CLIs that do not
// expose a structured event stream.
type plainAdapter struct{ raw strings.Builder }

func (a *plainAdapter) line(line string) []logEntry {
	a.raw.WriteString(line)
	a.raw.WriteByte('\n')
	return []logEntry{{Text: line}}
}

func (a *plainAdapter) usage() Usage   { return Usage{} }
func (a *plainAdapter) limits() Limits { return Limits{} }
func (a *plainAdapter) report() string { return a.raw.String() }
func (a *plainAdapter) err() error     { return nil }
