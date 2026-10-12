package volumes

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	listVolumeNames func() ([]string, error)
	removeVolume    func(string) error
}

func (f *fakeDocker) ListVolumeNames() ([]string, error) {
	if f.listVolumeNames == nil {
		return nil, nil
	}
	return f.listVolumeNames()
}

func (f *fakeDocker) RemoveVolume(name string) error {
	if f.removeVolume == nil {
		return nil
	}
	return f.removeVolume(name)
}

// fakeMsb implements Microsandbox; a nil field succeeds with a zero value.
type fakeMsb struct {
	available       func() error
	listVolumeNames func() ([]string, error)
	removeVolume    func(string) error
}

func (f *fakeMsb) Available() error {
	if f.available == nil {
		return nil
	}
	return f.available()
}

func (f *fakeMsb) ListVolumeNames() ([]string, error) {
	if f.listVolumeNames == nil {
		return nil, nil
	}
	return f.listVolumeNames()
}

func (f *fakeMsb) RemoveVolume(name string) error {
	if f.removeVolume == nil {
		return nil
	}
	return f.removeVolume(name)
}

// namesFunc returns a list func that returns names.
func namesFunc(names ...string) func() ([]string, error) {
	return func() ([]string, error) { return names, nil }
}
