package docker

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// runGuard tears a run down on release, holding off interrupt/terminate signals until then.
type runGuard struct {
	client   *Client
	toolHome string
	id       sandbox.RunID

	// signals fires on a guarded signal, so a startup in progress can stop; nil without signals
	signals <-chan os.Signal
	stop    func()
}

// guardRun guards run id under toolHome, catching signals only with catchSignals.
func (c *Client) guardRun(toolHome string, id sandbox.RunID, catchSignals bool) *runGuard {
	g := &runGuard{
		client:   c,
		toolHome: toolHome,
		id:       id,
		stop:     noCleanup,
	}
	if catchSignals {
		g.signals, g.stop = guardSignals()
	}
	return g
}

// release tears the run down, then uninstalls the signal guard, so a second Ctrl-C can't cut the teardown short; a nil
// guard, from a dry run, does nothing.
func (g *runGuard) release() {
	if g == nil {
		return
	}
	warnOnTeardownError(g.client.Teardown(g.toolHome, g.id))
	g.stop()
}

// guardSignals routes interrupt/terminate signals to a channel so deferred cleanup still runs; stop uninstalls it.
func guardSignals() (signals <-chan os.Signal, stop func()) {
	ch := make(chan os.Signal, 1)
	// SIGHUP covers a closed terminal
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return ch, func() { signal.Stop(ch) }
}

// noCleanup is the stop func for paths that set nothing up.
func noCleanup() {
	// Nothing to undo
}
