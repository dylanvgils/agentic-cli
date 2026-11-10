package platform

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// StartDetached starts name in its own session, with stdin and stderr on the null device, and waits for its first stdout
// line.
func StartDetached(name string, args []string, readyWithin time.Duration) error {
	ready, readyWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create ready pipe: %w", err)
	}
	defer func() { _ = ready.Close() }()

	cmd := exec.Command(name, args...) // NOSONAR - callers pass agentic's own executable
	cmd.Stdout = readyWriter
	cmd.SysProcAttr = detachedAttr()
	startErr := cmd.Start()
	_ = readyWriter.Close()
	if startErr != nil {
		return startErr
	}

	// Reap it if it exits first
	go func() { _ = cmd.Wait() }()
	return awaitLine(ready, readyWithin)
}

// awaitLine waits up to timeout for a line on r.
func awaitLine(r *os.File, timeout time.Duration) error {
	got := make(chan error, 1)
	go func() {
		_, err := bufio.NewReader(r).ReadString('\n')
		got <- err
	}()

	select {
	case err := <-got:
		if err != nil {
			return errors.New("exited before it was ready")
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("not ready within %s", timeout)
	}
}
