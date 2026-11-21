package microsandbox

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
)

// ImageSource is where msb gets the tool images it runs.
type ImageSource interface {
	// ImageID returns image's ID; found is false when image is missing.
	ImageID(image string) (id string, found bool, err error)
	// SaveImage streams image as a `docker save` tar archive to w.
	SaveImage(image string, w io.Writer) error
}

// EnsureImage returns image's ref in msb, loading it from src unless msb holds it, then removing older refs.
func (c *Client) EnsureImage(src ImageSource, image string) (string, error) {
	ref, found, err := c.Ref(src, image)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("image %s not found", image)
	}

	have, err := c.hasImage(ref)
	if err != nil {
		return "", err
	}
	if have {
		return ref, nil
	}

	if err := c.loadFrom(src, image, ref); err != nil {
		return "", err
	}

	refs, err := c.imageRefs()
	if err != nil {
		logging.Warnf("kept old microsandbox images of %s: %v", image, err)
		return ref, nil
	}
	c.removeStaleRefs(refs, image, ref)
	return ref, nil
}

// Ref returns image tagged with its ID in src, pinning the run to this build, without loading it; found is false when
// src lacks image.
func (c *Client) Ref(src ImageSource, image string) (ref string, found bool, err error) {
	id, found, err := src.ImageID(image)
	if err != nil {
		return "", false, fmt.Errorf("inspect %s: %w", image, err)
	}
	if !found {
		return "", false, nil
	}
	return image + ":" + id, true, nil
}

// loadFrom streams image saved from src into `msb load`, tagged as ref.
func (c *Client) loadFrom(src ImageSource, image, ref string) error {
	logging.Infof("loading %s into microsandbox", image)

	// An untagged name would save every tag of the repo
	if !strings.Contains(image[strings.LastIndex(image, "/")+1:], ":") {
		image += ":latest"
	}

	pr, pw := io.Pipe()
	saved := make(chan error, 1)
	go func() {
		err := src.SaveImage(image, pw)
		_ = pw.CloseWithError(err)
		saved <- err
	}()

	loadErr := c.loadImage(pr, ref)
	_ = pr.CloseWithError(loadErr)
	if saveErr := <-saved; saveErr != nil {
		return saveErr
	}
	return loadErr
}

// removeStaleRefs removes image's refs other than keep and latest, skipping ones in use.
func (c *Client) removeStaleRefs(refs []string, image, keep string) {
	repo, _ := SplitRef(image)
	for _, ref := range refs {
		refRepo, tag := SplitRef(ref)
		if refRepo != repo || tag == "latest" || sameRef(ref, keep) {
			continue
		}
		if err := c.removeImage(ref); err != nil {
			logging.Warnf("kept old microsandbox image %s: %v", ref, err)
		}
	}
}

// loadImage loads a `docker save` tar archive read from r into msb's image store, adding tag as a local reference.
func (c *Client) loadImage(r io.Reader, tag string) error {
	if _, err := c.runner.RunWithStdin(r, "load", arg("tag", tag)); err != nil {
		return fmt.Errorf("load image into msb: %w", err)
	}
	return nil
}

// hasImage reports whether msb's image store resolves ref, the lookup `msb run` uses; msb exiting non-zero means it
// doesn't, so a misread costs only a reload.
func (c *Client) hasImage(ref string) (bool, error) {
	_, err := c.runner.Run("image", "inspect", ref)
	if exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect msb image %s: %w", ref, err)
	}
	return true, nil
}

// imageRefs lists the references of the images in msb's image store.
func (c *Client) imageRefs() ([]string, error) {
	out, err := c.runner.Run("images", arg("quiet"))
	if err != nil {
		return nil, fmt.Errorf("list msb images: %w", err)
	}

	var refs []string
	for line := range strings.Lines(out) {
		if ref := strings.TrimSpace(line); ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

// removeImage removes ref from msb's image store; msb keeps the image while a sandbox uses it.
func (c *Client) removeImage(ref string) error {
	if _, err := c.runner.Run("rmi", ref); err != nil {
		return fmt.Errorf("remove msb image %s: %w", ref, err)
	}
	return nil
}

// SplitRef returns ref's repository name without any registry or namespace prefix, and its tag ("latest" when unset).
func SplitRef(ref string) (repo, tag string) {
	name := ref[strings.LastIndex(ref, "/")+1:]
	repo, tag, ok := strings.Cut(name, ":")
	if !ok {
		tag = "latest"
	}
	return repo, tag
}

// sameRef reports whether a and b name the same repository and tag, ignoring a registry or namespace prefix msb may add.
func sameRef(a, b string) bool {
	aRepo, aTag := SplitRef(a)
	bRepo, bTag := SplitRef(b)
	return aRepo == bRepo && aTag == bTag
}
