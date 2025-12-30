package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/clarateach/backend/internal/orchestrator"
	"github.com/clarateach/backend/internal/store"
)

type DynamicProxy struct {
	store        store.Store
	orchestrator orchestrator.Provider
	baseDomain   string
}

func NewDynamicProxy(s store.Store, o orchestrator.Provider, baseDomain string) *DynamicProxy {
	return &DynamicProxy{
		store:        s,
		orchestrator: o,
		baseDomain:   baseDomain,
	}
}

// ServeHTTP implements the reverse proxy logic
func (p *DynamicProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Identify Target from Hostname
	host := r.Host
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Check if subdomain matches workshop pattern
	if !strings.HasSuffix(host, p.baseDomain) {
		http.Error(w, "Invalid domain", http.StatusNotFound)
		return
	}

	prefix := strings.TrimSuffix(host, "."+p.baseDomain)
	// format: ws-<id> (or just <id>?)
	workshopID := prefix

	// 2. Identify Seat from Path (e.g. /vm/1/...)
	path := r.URL.Path
	var seatID int
	var targetPath string

	if strings.HasPrefix(path, "/vm/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 3 {
			// parts[0] = ""
			// parts[1] = "vm"
			// parts[2] = "1" (seat)
			fmt.Sscanf(parts[2], "%d", &seatID)
			targetPath = "/" + strings.Join(parts[3:], "/")
		}
	}

	if seatID == 0 {
		http.Error(w, "Seat not specified", http.StatusBadRequest)
		return
	}

	// 3. Get instance details (includes host ports for local dev)
	instance, err := p.orchestrator.GetInstance(r.Context(), workshopID, seatID)
	if err != nil {
		http.Error(w, "Workspace not ready", http.StatusServiceUnavailable)
		return
	}

	// 4. Determine Service Type & Port
	// The proxy extracts /vm/{seat}/terminal/... or /vm/{seat}/files/...
	// After routing, we need to pass the FULL path to the workspace server
	// because the workspace server routes are /vm/:seat/files and /vm/:seat/terminal
	var targetHost string
	var port int

	// For local development, use localhost with host-mapped ports
	// For production (when IP is routable), use container IP
	useHostPorts := instance.HostTerminalPort > 0 && p.baseDomain == "localhost"

	// Determine which service to route to, but KEEP the path as /vm/{seat}/{service}/...
	// because that's what the workspace server expects
	if strings.HasPrefix(targetPath, "/browser") {
		// Route to Neko Sidecar
		targetPath = strings.TrimPrefix(targetPath, "/browser")
		if useHostPorts {
			targetHost = "127.0.0.1"
			port = instance.HostBrowserPort
		} else {
			browserIP, _ := p.orchestrator.GetBrowserIP(r.Context(), workshopID, seatID)
			targetHost = browserIP
			port = 3003
		}
	} else {
		// Route to Main Workspace - keep the full /vm/{seat}/... path for the workspace server
		if useHostPorts {
			targetHost = "127.0.0.1"
			if strings.HasPrefix(targetPath, "/terminal") {
				port = instance.HostTerminalPort
			} else if strings.HasPrefix(targetPath, "/files") {
				port = instance.HostFilesPort
			} else {
				port = instance.HostFilesPort // Default to files
			}
		} else {
			targetHost = instance.IP
			if strings.HasPrefix(targetPath, "/terminal") {
				port = 3001
			} else if strings.HasPrefix(targetPath, "/files") {
				port = 3002
			} else {
				port = 3002 // Default to files
			}
		}
		// Reconstruct path as /vm/{seat}/{rest of path}
		// targetPath is already like /files or /terminal/xxx
		targetPath = fmt.Sprintf("/vm/%d%s", seatID, targetPath)
	}

	if targetHost == "" || port == 0 {
		http.Error(w, "Workspace not found", http.StatusNotFound)
		return
	}

	// 5. Proxy Request
	targetStr := fmt.Sprintf("http://%s:%d", targetHost, port)
	target, _ := url.Parse(targetStr)

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Update path for the target (strip the prefix)
	r.URL.Path = targetPath

	proxy.ServeHTTP(w, r)
}
