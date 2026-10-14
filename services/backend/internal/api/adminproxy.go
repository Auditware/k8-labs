package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// adminProxy reverse-proxies one of the admin dashboard's linked-out
// component UIs. Grafana and ArgoCD are both configured (root_url +
// serve_from_sub_path, server.rootpath - see docs/architecture/
// environments.md) to expect their own mount prefix as part of every
// request path - same principle the ttyd terminal proxy already uses
// (internal/labs/proxy.go): get the upstream to understand its own
// prefix instead of rewriting requests to hide it, wherever the upstream
// actually supports that. Vault has no such support at all, so its proxy
// strips the prefix instead - the one exception, passed as stripPrefix.
func adminProxy(host, stripPrefix string) http.HandlerFunc {
	target := &url.URL{Scheme: "http", Host: host}
	proxy := httputil.NewSingleHostReverseProxy(target)
	return func(w http.ResponseWriter, r *http.Request) {
		if stripPrefix != "" {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, stripPrefix)
			if !strings.HasPrefix(r.URL.Path, "/") {
				r.URL.Path = "/" + r.URL.Path
			}
		}
		proxy.ServeHTTP(w, r)
	}
}
