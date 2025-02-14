package acl

import (
	"bufio"
	"io"
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
	SectionHR                // host_redirect
)

func loadOptions(s *bufio.Scanner) *Options {
	o := NewOptions()

	for {
		var section Section
		l, err := loadLine(s)
		if err != nil {
			break
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
		case "[host_redirect]":
			loadHostRedirect(s, o)
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

	return o
}

func loadLine(s *bufio.Scanner) (string, error) {
	for s.Scan() {
		l := strings.TrimSpace(s.Text())
		if l == "" {
			continue
		}

		if strings.HasPrefix(l, "#") {
			continue
		}

		return l, nil
	}

	return "", io.EOF
}

func loadHostRedirect(s *bufio.Scanner, o *Options) {
	var url string
	var statusCode int

	var done int

	for {
		l, err := loadLine(s)
		if err != nil {
			return
		}

		if strings.HasPrefix(l, "url=") {
			url = strings.TrimLeft(l, "url=")
			done++
		} else if strings.HasPrefix(l, "status_code=") {
			statusCode, _ = strconv.Atoi(strings.TrimLeft(l, "status_code="))
			done++
		}

		if done == 2 {
			break
		}
	}

	o.HostRedirectURL = url
	if statusCode > 0 {
		o.HostRedirectStatusCode = statusCode
	}

}
