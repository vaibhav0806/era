package main

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

// newProxyHandler returns the forward-proxy HTTP handler. It supports both
// CONNECT (HTTPS tunneling) and direct HTTP forwarding.
func newProxyHandler(a *allowlist) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			handleConnect(w, r, a)
			return
		}
		// HTTP requests come with absolute URI. r.URL.Host is the target.
		target := r.URL.Host
		if target == "" {
			target = r.Host
		}
		if !a.allowed(target) {
			http.Error(w, "host not in allowlist: "+hostnameOnly(target), http.StatusForbidden)
			return
		}
		forwardHTTP(w, r)
	})
}

func handleConnect(w http.ResponseWriter, r *http.Request, a *allowlist) {
	if !a.allowed(r.Host) {
		http.Error(w, "host not in allowlist: "+hostnameOnly(r.Host), http.StatusForbidden)
		return
	}
	dst, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "connect: "+err.Error(), http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		dst.Close()
		return
	}
	src, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "hijack: "+err.Error(), http.StatusInternalServerError)
		dst.Close()
		return
	}
	src.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	go func() { defer dst.Close(); io.Copy(dst, src) }()
	go func() { defer src.Close(); io.Copy(src, dst) }()
}

func forwardHTTP(w http.ResponseWriter, r *http.Request) {
	out, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for k, vv := range r.Header {
		for _, v := range vv {
			out.Header.Add(k, v)
		}
	}
	out.Header.Del("Proxy-Connection")
	out.Header.Del("Connection")

	resp, err := http.DefaultTransport.RoundTrip(out)
	if err != nil {
		slog.Error("proxy forward", "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func hostnameOnly(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}

// newTestPermitHandler returns a handler that lets tests permit a host
// dynamically. Only registered when PI_SIDECAR_TEST_HOOKS=1.
//
// SECURITY NOTE: this endpoint is gated by env var, not build tag. If
// PI_SIDECAR_TEST_HOOKS=1 leaks into production, any in-container process
// could permit arbitrary hosts. For M2 we accept the risk because the sidecar
// is loopback-only. A later hardening step is to gate via `//go:build testonly`
// so this code is physically absent from production binaries.
func newTestPermitHandler(a *allowlist) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.URL.Query().Get("host")
		ttlMs, _ := strconv.Atoi(r.URL.Query().Get("ttl_ms"))
		if ttlMs <= 0 {
			ttlMs = 30000
		}
		a.permit(host, time.Duration(ttlMs)*time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}
}
