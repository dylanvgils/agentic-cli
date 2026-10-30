//go:build integration

package network

// gatewayScript is reachScript for the run network's gateway address, the first one in the tool's subnet.
func gatewayScript(port string) string {
	return `gw=$(hostname -i | awk '{print $1}' | sed 's/\.[0-9]*$/.1/'); echo "gateway=$gw"; ` + reachScript("$gw", port)
}

// reachScript prints reach=yes when a TCP connection to ip:port succeeds, reach=no otherwise.
func reachScript(ip, port string) string {
	return "nc -z -w 3 " + ip + " " + port + " && echo reach=yes || echo reach=no"
}
