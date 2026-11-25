#!/bin/sh

# msb passes the init its own args, so start dockerd without them
dockerd &
pid=$!

# Pass any shutdown signal msb may send (e.g. SIGRTMIN+4, which dockerd ignores) on as SIGTERM
trap 'kill "$pid"' TERM INT PWR $(seq 34 64)

# wait returns on each signal and reaps orphans too
while kill -0 "$pid" 2>/dev/null; do wait; done

# msb waits for the guest to power off; PID 1 exiting would panic the kernel instead
exec busybox poweroff -f
