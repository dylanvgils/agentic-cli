package tools

// layerPackages lists the apt packages each layer needs present in the base image.
// Each extra declares only what is not already in "base". collectPackages merges and deduplicates.
var layerPackages = map[string][]string{
	"base":   {"curl", "wget", "git", "gpg", "ca-certificates"},
	"dotnet": {"apt-transport-https"},
	"go":     {"jq"},
	"java":   {"apt-transport-https"},
}

// collectPackages merges the base packages with any extra packages declared by the given
// extra layers, deduplicating while preserving declaration order.
func collectPackages(extras []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, name := range append([]string{"base"}, extras...) {
		for _, pkg := range layerPackages[name] {
			if !seen[pkg] {
				seen[pkg] = true
				result = append(result, pkg)
			}
		}
	}

	return result
}
