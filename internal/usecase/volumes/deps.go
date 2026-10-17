package volumes

// Store is the subset of a runtime client this package uses to list and remove its named volumes.
type Store interface {
	ListVolumeNames() ([]string, error)
	RemoveVolume(name string) error
}

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	Store
}
