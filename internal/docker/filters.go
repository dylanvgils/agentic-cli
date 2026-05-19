package docker

// labelFilter builds a --filter=label=key=value Docker flag.
func labelFilter(key, value string) string {
	return arg("filter", "label="+key+"="+value)
}

// referenceFilter builds a --filter=reference=name Docker flag.
func referenceFilter(name string) string {
	return arg("filter", "reference="+name)
}
