package tools

// layerPackages lists the apt packages each layer needs present in the base image.
// Each extra declares only what is not already in "base". collectPackages merges and deduplicates.
var layerPackages = map[string][]string{
	"base":   {"curl", "wget", "git", "gpg", "ca-certificates"},
	"dotnet": {"apt-transport-https"},
	"go":     {"jq"},
	"java":   {"apt-transport-https"},
}

// MergePackages appends additional to base, deduplicating while preserving declaration order.
func MergePackages(base, additional []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, pkg := range append(base, additional...) {
		if !seen[pkg] {
			seen[pkg] = true
			result = append(result, pkg)
		}
	}

	return result
}

// collectPackages merges the base packages with any extra packages declared by the given
// extra layers and any user-supplied apt packages, deduplicating while preserving declaration order.
func collectPackages(extras []string, userPkgs []string) []string {
	return MergePackages(expandPackages(extras), userPkgs)
}

// expandPackages expands layer names to their package lists.
func expandPackages(extras []string) []string {
	var all []string
	for _, name := range append([]string{"base"}, extras...) {
		all = append(all, layerPackages[name]...)
	}
	return all
}
