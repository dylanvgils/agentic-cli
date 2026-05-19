package cmd

import "strings"

var (
	version       = "dev"
	commit        = ""
	buildDate     = ""
	installMethod = ""
)

func buildVersion() string {
	var meta []string

	if commit != "" {
		meta = append(meta, commit)
	}
	if buildDate != "" {
		meta = append(meta, buildDate)
	}
	if installMethod != "" {
		meta = append(meta, installMethod)
	}

	if len(meta) == 0 {
		return version
	}
	return version + " (" + strings.Join(meta, ", ") + ")"
}
