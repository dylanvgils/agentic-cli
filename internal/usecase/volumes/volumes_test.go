package volumes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_List(t *testing.T) {
	t.Run("returns docker volumes then msb volumes", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{listVolumeNames: namesFunc("maven")}, &fakeMsb{listVolumeNames: namesFunc("gradle")})

		// Act
		volumes, err := svc.List()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Volume{{Name: "maven", Runtime: RuntimeDocker}, {Name: "gradle", Runtime: RuntimeMicrosandbox}}, volumes)
	})

	t.Run("skips msb when it isn't installed", func(t *testing.T) {
		// Arrange
		msb := &fakeMsb{
			available:       func() error { return errors.New("msb not found") },
			listVolumeNames: func() ([]string, error) { return nil, errors.New("unexpected") },
		}
		svc := New(&fakeDocker{listVolumeNames: namesFunc("maven")}, msb)

		// Act
		volumes, err := svc.List()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Volume{{Name: "maven", Runtime: RuntimeDocker}}, volumes)
	})

	t.Run("returns a docker listing error", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{listVolumeNames: func() ([]string, error) { return nil, errors.New("daemon down") }}, &fakeMsb{})

		// Act
		_, err := svc.List()

		// Assert
		assert.ErrorContains(t, err, "daemon down")
	})

	t.Run("returns an msb listing error", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{}, &fakeMsb{listVolumeNames: func() ([]string, error) { return nil, errors.New("boom") }})

		// Act
		_, err := svc.List()

		// Assert
		assert.ErrorContains(t, err, "boom")
	})
}

func Test_Service_Remove(t *testing.T) {
	t.Run("removes a docker volume from docker", func(t *testing.T) {
		// Arrange
		var got string
		svc := New(&fakeDocker{removeVolume: func(n string) error { got = n; return nil }}, &fakeMsb{})

		// Act
		err := svc.Remove(Volume{Name: "maven", Runtime: RuntimeDocker})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "maven", got)
	})

	t.Run("removes an msb volume from msb", func(t *testing.T) {
		// Arrange
		var got string
		svc := New(&fakeDocker{}, &fakeMsb{removeVolume: func(n string) error { got = n; return nil }})

		// Act
		err := svc.Remove(Volume{Name: "maven", Runtime: RuntimeMicrosandbox})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "maven", got)
	})
}

func Test_Service_RemoveNamed(t *testing.T) {
	t.Run("removes the volume from every runtime that holds it", func(t *testing.T) {
		// Arrange
		var removed []string
		d := &fakeDocker{
			listVolumeNames: namesFunc("maven", "gradle"),
			removeVolume:    func(n string) error { removed = append(removed, "docker "+n); return nil },
		}
		m := &fakeMsb{
			listVolumeNames: namesFunc("maven"),
			removeVolume:    func(n string) error { removed = append(removed, "msb "+n); return nil },
		}

		// Act
		err := New(d, m).RemoveNamed("maven")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"docker maven", "msb maven"}, removed)
	})

	t.Run("removes a volume only msb holds", func(t *testing.T) {
		// Arrange
		var removed []string
		m := &fakeMsb{
			listVolumeNames: namesFunc("maven"),
			removeVolume:    func(n string) error { removed = append(removed, n); return nil },
		}

		// Act
		err := New(&fakeDocker{}, m).RemoveNamed("maven")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"maven"}, removed)
	})

	t.Run("returns an error when no runtime holds it", func(t *testing.T) {
		// Act
		err := New(&fakeDocker{}, &fakeMsb{}).RemoveNamed("maven")

		// Assert
		assert.EqualError(t, err, "'maven' is not an agentic-managed volume")
	})

	t.Run("returns a remove error", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			listVolumeNames: namesFunc("maven"),
			removeVolume:    func(string) error { return errors.New("in use") },
		}

		// Act
		err := New(d, &fakeMsb{}).RemoveNamed("maven")

		// Assert
		assert.ErrorContains(t, err, "in use")
	})
}
