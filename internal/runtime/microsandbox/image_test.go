package microsandbox

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_EnsureImage(t *testing.T) {
	t.Run("a missing image is an error", func(t *testing.T) {
		// Arrange
		store := &fakeImageStore{}
		src := &fakeImageSource{imageID: func(string) (string, bool, error) { return "", false, nil }}

		// Act
		_, err := store.client().EnsureImage(src, "agentic-claude")

		// Assert
		require.EqualError(t, err, "image agentic-claude not found")
		assert.Empty(t, store.calls)
	})

	t.Run("ensures the image is loaded under its ID ref and returns the ref", func(t *testing.T) {
		// Arrange
		store := &fakeImageStore{held: true}

		// Act
		ref, err := store.client().EnsureImage(&fakeImageSource{}, "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude:abc123", ref)
		assert.Equal(t, []string{"inspect agentic-claude:abc123"}, store.calls)
	})

	t.Run("a failed load returns no ref", func(t *testing.T) {
		// Arrange
		store := &fakeImageStore{inspectErr: errors.New("msb down")}

		// Act
		ref, err := store.client().EnsureImage(&fakeImageSource{}, "agentic-claude")

		// Assert
		require.EqualError(t, err, "inspect msb image agentic-claude:abc123: msb down")
		assert.Empty(t, ref)
	})
}

func TestClient_Ref(t *testing.T) {
	for _, tc := range []struct {
		name      string
		imageID   func(string) (string, bool, error)
		wantRef   string
		wantFound bool
		wantErr   string
	}{
		{
			name:      "tags the image with its ID",
			wantRef:   "agentic-claude:abc123",
			wantFound: true,
		},
		{
			name:    "a missing image is not found",
			imageID: func(string) (string, bool, error) { return "", false, nil },
		},
		{
			name:    "inspect error is wrapped",
			imageID: func(string) (string, bool, error) { return "", false, errors.New("daemon down") },
			wantErr: "inspect agentic-claude: daemon down",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			store := &fakeImageStore{}

			// Act
			ref, found, err := store.client().Ref(&fakeImageSource{imageID: tc.imageID}, "agentic-claude")

			// Assert
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantRef, ref)
			assert.Equal(t, tc.wantFound, found)
			assert.Empty(t, store.calls)
		})
	}
}

func Test_Client_ensureLoaded(t *testing.T) {
	t.Run("skips the load when msb already holds the ref", func(t *testing.T) {
		// Arrange
		store := &fakeImageStore{held: true}
		saved := false
		src := &fakeImageSource{saveImage: func(string, io.Writer) error {
			saved = true
			return nil
		}}

		// Act
		err := store.client().ensureLoaded(src, "agentic-claude", "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"inspect agentic-claude:abc123"}, store.calls)
		assert.False(t, saved)
	})

	t.Run("check error returns before loading", func(t *testing.T) {
		// Arrange
		store := &fakeImageStore{inspectErr: errors.New("msb down")}

		// Act
		err := store.client().ensureLoaded(&fakeImageSource{}, "agentic-claude", "agentic-claude:abc123")

		// Assert
		require.EqualError(t, err, "inspect msb image agentic-claude:abc123: msb down")
		assert.Equal(t, []string{"inspect agentic-claude:abc123"}, store.calls)
	})

	t.Run("failed load keeps the old refs", func(t *testing.T) {
		// Arrange
		store := &fakeImageStore{loadErr: errors.New("load failed"), refs: []string{"agentic-claude:old111"}}

		// Act
		err := store.client().ensureLoaded(&fakeImageSource{}, "agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "load failed")
		assert.Equal(t, []string{"inspect agentic-claude:abc123", "load agentic-claude:abc123"}, store.calls)
	})

	t.Run("loads the image under the ref when msb lacks it, then removes its stale refs", func(t *testing.T) {
		// Arrange
		store := &fakeImageStore{refs: []string{"agentic-claude:old111", "agentic-claude:abc123"}}

		// Act
		err := store.client().ensureLoaded(&fakeImageSource{}, "agentic-claude", "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{
			"inspect agentic-claude:abc123",
			"load agentic-claude:abc123",
			"images",
			"rmi agentic-claude:old111",
		}, store.calls)
	})
}

func Test_Client_loadFrom(t *testing.T) {
	t.Run("streams the saved image into msb under the ref", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		var gotInput string
		client := newTestClient(&fakeRunner{runWithStdin: func(r io.Reader, args ...string) (string, error) {
			gotArgs = args
			b, err := io.ReadAll(r)
			gotInput = string(b)
			return "", err
		}})

		// Act
		err := client.loadFrom(&fakeImageSource{}, "agentic-claude", "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"load", "--tag=agentic-claude:abc123"}, gotArgs)
		assert.Equal(t, "tar:agentic-claude:latest", gotInput)
	})

	for _, tc := range []struct {
		name string
		load func(io.Reader, ...string) (string, error)
	}{
		{
			name: "a failed save is reported over the load error",
			load: func(io.Reader, ...string) (string, error) { return "", errors.New("truncated archive") },
		},
		{
			name: "a failed save fails a load that read to the end",
			load: func(r io.Reader, _ ...string) (string, error) {
				_, err := io.ReadAll(r)
				return "", err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			client := newTestClient(&fakeRunner{runWithStdin: tc.load})
			src := &fakeImageSource{saveImage: func(string, io.Writer) error { return errors.New("save failed") }}

			// Act
			err := client.loadFrom(src, "agentic-claude", "agentic-claude:abc123")

			// Assert
			assert.EqualError(t, err, "save failed")
		})
	}
}

func Test_Client_removeStaleRefs(t *testing.T) {
	t.Run("removes only the image's other ID refs", func(t *testing.T) {
		// Arrange
		store := &fakeImageStore{refs: []string{
			"agentic-claude:old111",
			"docker.io/library/agentic-claude:old222",
			"agentic-claude:abc123",
			"agentic-claude:latest",
			"agentic-claude",
			"agentic-copilot:old333",
			"python:3.13",
		}}

		// Act
		store.client().removeStaleRefs("agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.Equal(t, []string{
			"images",
			"rmi agentic-claude:old111",
			"rmi docker.io/library/agentic-claude:old222",
		}, store.calls)
	})

	t.Run("a ref msb can't remove is skipped", func(t *testing.T) {
		// Arrange
		logs := iostreamstest.StubErr(t)
		store := &fakeImageStore{
			refs:    []string{"agentic-claude:old111", "agentic-claude:old222"},
			rmiErrs: map[string]error{"agentic-claude:old111": errors.New("in use")},
		}

		// Act
		store.client().removeStaleRefs("agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.Equal(t, []string{"images", "rmi agentic-claude:old111", "rmi agentic-claude:old222"}, store.calls)
		assert.Contains(t, logs.String(), "kept old microsandbox image agentic-claude:old111: remove msb image agentic-claude:old111: in use")
	})

	t.Run("a listing error only warns", func(t *testing.T) {
		// Arrange
		logs := iostreamstest.StubErr(t)
		store := &fakeImageStore{refsErr: errors.New("msb down")}

		// Act
		store.client().removeStaleRefs("agentic-claude", "agentic-claude:abc123")

		// Assert
		assert.Equal(t, []string{"images"}, store.calls)
		assert.Contains(t, logs.String(), "kept old microsandbox images of agentic-claude: list msb images: msb down")
	})
}

func Test_Client_loadImage(t *testing.T) {
	t.Run("pipes the archive to msb load with the tag", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		var gotInput string
		client := newTestClient(&fakeRunner{runWithStdin: func(r io.Reader, args ...string) (string, error) {
			gotArgs = args
			b, err := io.ReadAll(r)
			gotInput = string(b)
			return "", err
		}})

		// Act
		err := client.loadImage(strings.NewReader("tar-bytes"), "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"load", "--tag=agentic-claude:abc123"}, gotArgs)
		assert.Equal(t, "tar-bytes", gotInput)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{runWithStdin: func(io.Reader, ...string) (string, error) {
			return "", errors.New("boom")
		}})

		// Act
		err := client.loadImage(strings.NewReader("tar-bytes"), "agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "load image into msb: boom")
	})
}

func Test_Client_hasImage(t *testing.T) {
	notFound := exec.Command("false").Run()
	for _, tc := range []struct {
		name    string
		err     error
		want    bool
		wantErr string
	}{
		{name: "msb resolves the ref", want: true},
		{name: "msb exiting non-zero means missing", err: fmt.Errorf("msb inspect: %w", notFound)},
		{name: "msb failing to start is an error", err: errors.New("boom"), wantErr: "inspect msb image agentic-claude:abc123: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			client, args := stubRun("", tc.err)

			// Act
			have, err := client.hasImage("agentic-claude:abc123")

			// Assert
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, have)
			assert.Equal(t, []string{"image", "inspect", "agentic-claude:abc123"}, *args)
		})
	}
}

func Test_Client_imageRefs(t *testing.T) {
	t.Run("returns one ref per non-empty line", func(t *testing.T) {
		// Arrange
		client, args := stubRun("agentic-claude:abc123\n\n  python:latest  \n", nil)

		// Act
		refs, err := client.imageRefs()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"images", "--quiet"}, *args)
		assert.Equal(t, []string{"agentic-claude:abc123", "python:latest"}, refs)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client, _ := stubRun("", errors.New("boom"))

		// Act
		_, err := client.imageRefs()

		// Assert
		assert.ErrorContains(t, err, "list msb images: boom")
	})
}

func Test_Client_removeImage(t *testing.T) {
	t.Run("runs msb rmi for the ref", func(t *testing.T) {
		// Arrange
		client, args := stubRun("", nil)

		// Act
		err := client.removeImage("agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"rmi", "agentic-claude:abc123"}, *args)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client, _ := stubRun("", errors.New("in use"))

		// Act
		err := client.removeImage("agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "remove msb image agentic-claude:abc123: in use")
	})
}

func TestSplitRef(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ref     string
		wantTag string
	}{
		{name: "name and tag", ref: "agentic-claude:abc123", wantTag: "abc123"},
		{name: "drops a registry and namespace prefix", ref: "registry.example.test:5000/library/agentic-claude:abc123", wantTag: "abc123"},
		{name: "no tag means latest", ref: "agentic-claude", wantTag: "latest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			repo, tag := SplitRef(tc.ref)

			// Assert
			assert.Equal(t, "agentic-claude", repo)
			assert.Equal(t, tc.wantTag, tag)
		})
	}
}

func Test_sameRef(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    string
		want bool
	}{
		{name: "ignores a registry prefix", a: "docker.io/library/agentic-claude:abc123", want: true},
		{name: "different tag differs", a: "agentic-claude:old111"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			same := sameRef(tc.a, "agentic-claude:abc123")

			// Assert
			assert.Equal(t, tc.want, same)
		})
	}
}

func Test_withTag(t *testing.T) {
	for _, tc := range []struct {
		name  string
		image string
		want  string
	}{
		{name: "adds latest to an untagged name", image: "agentic-claude", want: "agentic-claude:latest"},
		{name: "keeps a tag", image: "agentic-claude:abc123", want: "agentic-claude:abc123"},
		{name: "a registry port is no tag", image: "registry.example.test:5000/agentic-claude", want: "registry.example.test:5000/agentic-claude:latest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := withTag(tc.image)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}
