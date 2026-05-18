package mount

import "strings"

// TmpfsOptions configures a tmpfs mount.
type TmpfsOptions struct {
	Exec bool
	Size string
}

// TmpfsMount builds a Docker tmpfs spec: path[:options]
func TmpfsMount(path string, opts TmpfsOptions) string {
	var parts []string
	if opts.Exec {
		parts = append(parts, "exec")
	}

	if opts.Size != "" {
		parts = append(parts, "size="+opts.Size)
	}

	if len(parts) == 0 {
		return path
	}

	return path + ":" + strings.Join(parts, ",")
}
