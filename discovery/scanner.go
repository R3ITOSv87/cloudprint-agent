package discovery

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// Printer represents a discovered network printer.
type Printer struct {
	Name     string
	IPPURI   string
	Model    string
	Location string
	IP       string
	Port     int
}

// ScanMDNS discovers IPP printers via mDNS/Bonjour.
func ScanMDNS(ctx context.Context, timeout time.Duration) ([]Printer, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("mdns resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var printers []Printer
	var mu sync.Mutex

	go func() {
		for entry := range entries {
			if len(entry.AddrIPv4) == 0 {
				continue
			}
			ip := entry.AddrIPv4[0].String()
			port := entry.Port
			if port == 0 {
				port = 631
			}
			ippURI := fmt.Sprintf("ipp://%s:%d/ipp/print", ip, port)

			// Try to get model from TXT records
			model := ""
			for _, txt := range entry.Text {
				if len(txt) > 3 && txt[:3] == "ty=" {
					model = txt[3:]
				}
				if len(txt) > 4 && txt[:4] == "mdl=" {
					model = txt[4:]
				}
			}

			mu.Lock()
			printers = append(printers, Printer{
				Name:   entry.ServiceInstanceName(),
				IPPURI: ippURI,
				Model:  model,
				IP:     ip,
				Port:   port,
			})
			mu.Unlock()
		}
	}()

	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	services := []string{"_ipp._tcp", "_ipps._tcp", "_printer._tcp"}
	for _, svc := range services {
		if err := resolver.Browse(scanCtx, svc, "local.", entries); err != nil {
			// non-fatal, try next service type
			continue
		}
	}

	<-scanCtx.Done()
	close(entries)
	return printers, nil
}

// ScanSubnet probes port 631 (IPP) across a subnet range.
// subnet example: "192.168.1.0/24"
func ScanSubnet(subnet string, timeout time.Duration) []Printer {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		// Try to derive subnet from local interface
		ipNet = getLocalSubnet()
		if ipNet == nil {
			return nil
		}
	}

	hosts := expand(ipNet)
	results := make(chan Printer, len(hosts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // max 50 concurrent probes

	for _, host := range hosts {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			addr := fmt.Sprintf("%s:631", ip)
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err != nil {
				return
			}
			conn.Close()

			// Port is open — probe IPP
			ippURI := fmt.Sprintf("ipp://%s:631/ipp/print", ip)
			results <- Printer{
				Name:   fmt.Sprintf("Printer @ %s", ip),
				IPPURI: ippURI,
				IP:     ip,
				Port:   631,
			}
		}(host)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var printers []Printer
	for p := range results {
		printers = append(printers, p)
	}
	return printers
}

// ScanCUPS lists printers configured in the local CUPS installation (Linux).
func ScanCUPS() []Printer {
	out, err := exec.Command("lpstat", "-p", "-d").CombinedOutput()
	if err != nil {
		return nil
	}
	var printers []Printer
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "printer ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[1]
				printers = append(printers, Printer{
					Name:   name,
					IPPURI: fmt.Sprintf("ipp://localhost:631/printers/%s", name),
					IP:     "localhost",
					Port:   631,
				})
			}
		}
	}
	return printers
}

func expand(ipNet *net.IPNet) []string {
	var ips []string
	for ip := cloneIP(ipNet.IP.Mask(ipNet.Mask)); ipNet.Contains(ip); incrementIP(ip) {
		// skip network address and broadcast
		if ip[len(ip)-1] == 0 || ip[len(ip)-1] == 255 {
			continue
		}
		ips = append(ips, ip.String())
	}
	return ips
}

func cloneIP(ip net.IP) net.IP {
	clone := make(net.IP, len(ip))
	copy(clone, ip)
	return clone
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func getLocalSubnet() *net.IPNet {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				return ipNet
			}
		}
	}
	return nil
}
