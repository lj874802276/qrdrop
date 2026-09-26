// Package netutil helps QRDrop present a scannable address: it picks the
// machine's primary LAN IPv4 so the QR code points at something a phone can
// actually reach, and opens the default browser on startup for zero-friction use.
package netutil

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

// PrimaryLanIP returns the machine's most likely LAN-reachable IPv4 address.
// It skips loopback and virtual adapters (VMware, VirtualBox, WSL, Hyper-V,
// Docker bridges) and prefers RFC1918 private ranges. An empty string means
// no suitable address was found — callers should fall back to localhost.
func PrimaryLanIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var candidates []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isVirtualInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil || !ip.IsGlobalUnicast() {
				continue
			}
			candidates = append(candidates, ip.String())
		}
	}
	for _, c := range candidates {
		if ip := net.ParseIP(c); ip != nil && ip.IsPrivate() {
			return c
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

// isVirtualInterface reports whether an adapter name looks like a VM/container
// bridge that a phone could never route to.
func isVirtualInterface(name string) bool {
	n := strings.ToLower(name)
	for _, probe := range []string{
		"vmware", "virtualbox", "vboxnet",
		"vethernet", "wsl", "hyper-v", "hyperv",
		"docker", "br-", "lo",
	} {
		if strings.Contains(n, probe) {
			return true
		}
	}
	return false
}

// OpenBrowser opens url in the system default browser. It is best-effort:
// any failure is returned, never fatal, so headless/server runs stay quiet.
func OpenBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", "", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // linux, freebsd, etc.
		cmd = "xdg-open"
		args = []string{url}
	}
	if err := exec.Command(cmd, args...).Start(); err != nil {
		return fmt.Errorf("open browser (%s): %w", cmd, err)
	}
	return nil
}
