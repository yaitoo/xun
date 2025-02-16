package reqlog

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yaitoo/xun"
)

// Format is a function type that takes a Context pointer, an Options pointer, and a time.Time as arguments.
// It is used to format log messages.
type Format func(c *xun.Context, options *Options, starts time.Time)

// Combined log request with Combined Log Format (XLF/ELF)
func Combined(c *xun.Context, options *Options, starts time.Time) {
	requestLine := fmt.Sprintf(`"%s %s %s"`, c.Request.Method, c.Request.URL.Path, c.Request.Proto)
	remoteAddr, _, _ := net.SplitHostPort(c.Request.RemoteAddr)

	//COMBINED: remote、visitor、user、datetime、request line、status、body_bytes_sent、referer、user-agent
	options.Logger.Printf("%s %s %s %s %s %d %d \"%s\" \"%s\"\n",
		remoteAddr,
		Escape(options.GetVisitor(c)),
		Escape(options.GetUser(c)),
		starts.Format("[02/Jan/2006:15:04:05 -0700]"),
		requestLine,
		c.Response.StatusCode(),
		c.Response.BodyBytesSent(),
		c.Request.Referer(),
		c.Request.UserAgent(),
	)
}

// VCombined log request with Combined Log Format (XLF/ELF) with virtual host
func VCombined(c *xun.Context, options *Options, starts time.Time) {
	requestLine := fmt.Sprintf(`"%s %s %s"`, c.Request.Method, c.Request.URL.Path, c.Request.Proto)
	remoteAddr, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	host := c.Request.Host

	if host == "" {
		host = c.Request.URL.Host
	}

	if !strings.Contains(host, ":") {
		if IsHTTPs(c.Request) {
			host += ":" + "443"
		} else {
			host += ":" + "80"
		}
	}

	//VCombined: host、remote、visitor、user、datetime、request line、status、body_bytes_sent、referer、user-agent
	options.Logger.Printf("%s %s %s %s %s %s %d %d %s %s\n",
		host,
		remoteAddr,
		Escape(options.GetVisitor(c)),
		Escape(options.GetUser(c)),
		starts.Format("[02/Jan/2006:15:04:05 -0700]"),
		requestLine,
		c.Response.StatusCode(),
		c.Response.BodyBytesSent(),
		Escape(c.Request.Referer()),
		Escape(c.Request.UserAgent()),
	)
}

// Common log request with Common Log Format (CLF)
func Common(c *xun.Context, options *Options, starts time.Time) {
	requestLine := fmt.Sprintf(`"%s %s %s"`, c.Request.Method, c.Request.URL.Path, c.Request.Proto)
	remoteAddr, _, _ := net.SplitHostPort(c.Request.RemoteAddr)

	//Common: remote、visitor、user、datetime、request line、status、body_bytes_sent
	options.Logger.Printf("%s %s %s %s %s %d %d\n",
		remoteAddr,
		Escape(options.GetVisitor(c)),
		Escape(options.GetUser(c)),
		starts.Format("[02/Jan/2006:15:04:05 -0700]"),
		requestLine,
		c.Response.StatusCode(),
		c.Response.BodyBytesSent(),
	)
}

func IsHTTPs(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func Escape(s string) string {
	if s == "-" {
		return s
	}

	return "\"" + strings.ReplaceAll(s, `"`, `\"`) + "\""
}
