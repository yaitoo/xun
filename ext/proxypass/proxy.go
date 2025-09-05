package proxypass

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Proxy struct {
	Target *url.URL
	proxy  *httputil.ReverseProxy
}

func (s *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.proxy.ServeHTTP(rw, req)
}

func create(target *url.URL) *Proxy {
	return &Proxy{
		Target: target,
		proxy:  httputil.NewSingleHostReverseProxy(target),
	}
}

func up(domain string, s *Proxy) {
	mu.Lock()
	defer mu.Unlock()
	items := servers.Load().(map[string]*Proxy)
	items[domain] = s
	servers.Store(items)
}

func down(domain string) {
	mu.Lock()
	defer mu.Unlock()
	items := servers.Load().(map[string]*Proxy)
	delete(items, domain)
	servers.Store(items)
}

func get(domain, fallback string) *Proxy {
	items := servers.Load().(map[string]*Proxy)
	s, ok := items[domain]
	if ok {
		return s
	}

	s, ok = items[fallback]
	if ok {
		return s
	}

	return nil
}

// func rewriteRequestURL(req *http.Request, target *url.URL) {
// 	targetQuery := target.RawQuery
// 	req.URL.Scheme = target.Scheme
// 	req.URL.Host = target.Host
// 	req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
// 	if targetQuery == "" || req.URL.RawQuery == "" {
// 		req.URL.RawQuery = targetQuery + req.URL.RawQuery
// 	} else {
// 		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
// 	}
// }
