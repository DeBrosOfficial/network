package namespace

import "fmt"

// localRQLiteDSN is the DSN a process on this node uses to reach rqlited.
// rqlited binds the WireGuard advertise address, not 0.0.0.0 or loopback, so
// localhost is the wrong host once wrap is on.
func localRQLiteDSN(host string, port int) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}
