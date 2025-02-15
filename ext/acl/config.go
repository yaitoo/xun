package acl

import (
	"bufio"
	"io"
	"io/fs"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Section int

const (
	SectionNA Section = iota // none
	SectionAH                // allow_hosts
	SectionAN                // allow_ipnets
	SectionDN                // deny_ipnets
	SectionAC                // allow_countries
	SectionDC                // deny_countries
)

var openFile = func(file string) (fs.File, error) {
	return os.OpenFile(file, os.O_RDONLY, 0600)
}

func loadOptions(file string, o *Options) bool {
	f, err := openFile(file)
	if err != nil {
		Logger.Println("acl: can't read file", file, err)
		return false
	}

	// nolint: errcheck
	defer f.Close() // skipcq: GO-S2307

	s := bufio.NewScanner(f)
	var section Section
	for {
		l, err := loadLine(s)
		if err != nil {
			break
		}

		if l == "[host_redirect]" {
			section = SectionNA
			l = loadHostRedirect(s, o)
		}

		switch l {
		case "[allow_hosts]":
			section = SectionAH
			continue
		case "[allow_ipnets]":
			section = SectionAN
			continue
		case "[deny_ipnets]":
			section = SectionDN
			continue
		case "[allow_countries]":
			section = SectionAC
			continue
		case "[deny_countries]":
			section = SectionDC
			continue
		}

		switch section {
		case SectionAH:
			AllowHosts(l)(o)
		case SectionAN:
			AllowIPNets(l)(o)
		case SectionDN:
			DenyIPNets(l)(o)
		case SectionAC:
			AllowCountries(l)(o)
		case SectionDC:
			DenyCountries(l)(o)
		}

	}

	Logger.Printf("acl: allow_hosts=%v %v %s\n", len(o.AllowHosts), o.HostRedirectStatusCode, o.HostRedirectURL)
	Logger.Printf("acl: allow_ipnets=%v deny_ipnets=%v\n", len(o.AllowIPNets), len(o.DenyIPNets))
	Logger.Printf("acl: allow_countries=%v %v deny_countries=%v %v\n", o.AllowCountries.HasAny, len(o.AllowCountries.Items), o.DenyCountries.HasAny, len(o.DenyCountries.Items))
	return true
}

func loadLine(s *bufio.Scanner) (string, error) {
	for s.Scan() {
		l := strings.ReplaceAll(s.Text(), " ", "")
		if l == "" {
			continue
		}

		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, ";") {
			continue
		}

		return l, nil
	}

	return "", io.EOF
}

func loadHostRedirect(s *bufio.Scanner, o *Options) string {
	for {
		l, err := loadLine(s)
		if err != nil {
			return ""
		}

		if strings.HasPrefix(l, "url=") {
			u, err := url.Parse(l[len("url="):])
			if err == nil {
				o.HostRedirectURL = u.String()
			}
		} else if strings.HasPrefix(l, "status_code=") {
			o.HostRedirectStatusCode, _ = strconv.Atoi(l[len("status_code="):])
		} else if strings.HasPrefix(l, "[") { // other section starts
			return l
		}
	}
}
