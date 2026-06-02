package docker

// ImageFilter is a typed --filter flag for use with ListAllImages.
type ImageFilter string

// ToolFilter returns an ImageFilter matching images with the given tool label.
func ToolFilter(tool string) ImageFilter {
	return ImageFilter(labelFilter(LabelTool, tool))
}

// labelFilter builds a --filter=label=key=value Docker flag.
func labelFilter(key, value string) string {
	return arg("filter", "label="+key+"="+value)
}

// referenceFilter builds a --filter=reference=name Docker flag.
func referenceFilter(name string) string {
	return arg("filter", "reference="+name)
}
