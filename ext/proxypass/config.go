package proxypass

import (
	"net"
	"net/http"
	"net/http/httputil"
	"sync/atomic"

	"github.com/yaitoo/xun"
)

var (
	v *atomic.Value
)

func init() {
	v = &atomic.Value{}
	v.Store(make(map[string]*httputil.ReverseProxy))
}

func getReverseProxy(c *xun.Context, options *Options) *httputil.ReverseProxy {
	host, _, err := net.SplitHostPort(c.Request.URL.Host)
	if err != nil || host == "" {
		return nil
	}

	items := v.Load().(map[string]*httputil.ReverseProxy)

	it, ok := items[host]
	if ok {
		return it
	}

	it = httputil.NewSingleHostReverseProxy(c.Request.URL)

	// 1. 请求阶段：修正Host头确保后端识别
	it.Director = func(req *http.Request) {
		req.Host = c.Request.Host // 使用提取的域名
		req.URL.Host = c.Request.URL.Host
		req.URL.Scheme = c.Request.URL.Scheme

		ip, country := options.GetVisitor(c)

		req.Header.Set("X-Visitor-Ip", ip)
		req.Header.Set("X-Visitor-Country", country)
	}

	it.ModifyResponse = func(resp *http.Response) error {

		return nil
	}

	it.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("TargetURL is unavailable")) //nolint: errcheck
	}

	items[host] = it
	v.Store(items)
	return it
}
