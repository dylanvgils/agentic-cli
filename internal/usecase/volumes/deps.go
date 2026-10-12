package volumes

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	ListVolumeNames() ([]string, error)
	RemoveVolume(name string) error
}

// Microsandbox is the subset of *microsandbox.Client this package uses.
type Microsandbox interface {
	Available() error
	ListVolumeNames() ([]string, error)
	RemoveVolume(name string) error
}
