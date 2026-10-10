package proxy

// isBlocked is a seam over blockedAddr, so tests can tunnel to loopback upstreams.
var isBlocked = blockedAddr
