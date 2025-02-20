// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type (
	dnsQtype  uint16
	dnsQClass uint16
)

func (t dnsQtype) String() string {
	switch t {
	case dnsTypeIPV6Host:
		return "AAAA"
	case dnsTypeIPV4Host:
		return "A"
	case dnsTypeTXT:
		return "TXT"
	default:
		return ""
	}
}

func (c dnsQClass) String() string {
	switch c {
	case dnsClassChaos:
		return "CHAOSNET"
	case dnsClassInet:
		return "Inet"
	default:
		return ""
	}
}

const (
	dnsTypeIPV4Host dnsQtype  = dnsQtype(dns.TypeA)
	dnsTypeIPV6Host dnsQtype  = dnsQtype(dns.TypeAAAA)
	dnsTypeTXT      dnsQtype  = dnsQtype(dns.TypeTXT)
	dnsClassInet    dnsQClass = dnsQClass(dns.ClassINET)
	dnsClassChaos   dnsQClass = dnsQClass(dns.ClassCHAOS)
)

type ipResolver func(context.Context) ([]net.IP, error)

type publicIPResolver struct {
	v4Ips map[string]net.IP
	v6Ips map[string]net.IP
	m     sync.Mutex
}

func newPublicIPResolver() *publicIPResolver {
	return &publicIPResolver{
		v4Ips: map[string]net.IP{},
		v6Ips: map[string]net.IP{},
		m:     sync.Mutex{},
	}
}

func defaultResolvers() []ipResolver {
	return []ipResolver{
		withDNSResolver(
			"resolver1.opendns.com:53", "myip.opendns.com.",
			dnsClassInet, dnsTypeIPV4Host,
		),
		withDNSResolver(
			"resolver1.opendns.com:53", "myip.opendns.com.",
			dnsClassInet, dnsTypeIPV6Host,
		),
		withDNSResolver(
			"ns1.google.com:53", "o-o.myaddr.l.google.com.",
			dnsClassInet, dnsTypeTXT,
		),
		withDNSResolver(
			"one.one.one.one:53", "whoami.cloudflare.",
			dnsClassChaos, dnsTypeTXT,
		),
		withHTTPSBodyResolver("https://checkip.amazonaws.com"),
	}
}

func withDNSResolver(
	nameserver string,
	host string,
	qClass dnsQClass,
	qType dnsQtype,
) ipResolver {
	return func(ctx context.Context) ([]net.IP, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msg := &dns.Msg{
			MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery},
			Question: []dns.Question{{
				Name:   host,
				Qtype:  uint16(qType),
				Qclass: uint16(qClass),
			}},
		}

		baseErr := fmt.Sprintf("%s@%s (%s/%s)",
			host, nameserver, qType.String(), qClass.String(),
		)

		client := dns.Client{}
		res, _, err := client.ExchangeContext(ctx, msg, nameserver)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", baseErr, err)
		}

		if len(res.Answer) < 1 {
			return nil, fmt.Errorf("%s: no records found", baseErr)
		}

		ips := []net.IP{}
		for _, ans := range res.Answer {
			switch dnsQtype(ans.Header().Rrtype) {
			case dnsTypeIPV4Host:
				a, ok := ans.(*dns.A)
				if !ok {
					return ips, fmt.Errorf("%s: unable to convert answer to correct type. Expected A got %T", baseErr, ans)
				}
				ips = append(ips, a.A)
			case dnsTypeIPV6Host:
				a, ok := ans.(*dns.AAAA)
				if !ok {
					return ips, fmt.Errorf("%s: unable to convert answer to correct type. Expected AAAA got %T", baseErr, ans)
				}
				ips = append(ips, a.AAAA)
			case dnsTypeTXT:
				t, ok := ans.(*dns.TXT)
				if !ok {
					return ips, fmt.Errorf("%s: unable to convert answer to correct type. Expected A got %T", baseErr, ans)
				}

				if len(t.Txt) < 1 {
					return ips, fmt.Errorf("%s: no ips found in txt record", baseErr)
				}

				for _, txt := range t.Txt {
					ips = append(ips, net.ParseIP(txt))
				}
			default:
				return ips, fmt.Errorf("%s: unsupported answer type. got %T", baseErr, ans)
			}
		}

		return ips, nil
	}
}

func withHTTPSBodyResolver(host string) ipResolver {
	return func(ctx context.Context) ([]net.IP, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		baseErr := fmt.Sprintf("host(%s)", host)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, host, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", baseErr, err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", baseErr, err)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", baseErr, err)
		}

		return []net.IP{net.ParseIP(strings.TrimSpace(string(body)))}, nil
	}
}

// addIPs takes a slice of IPs and adds them to the ip set.
func (r *publicIPResolver) addIPs(ips []net.IP) {
	r.m.Lock()
	defer r.m.Unlock()

	for _, ip := range ips {
		// Make sure it's a valid IP address. To16() handles 4 and 16 byte addresses
		if ip == nil || ip.To16() == nil {
			continue
		}

		// See if it's a v4 address
		if ipv4 := ip.To4(); ipv4 != nil {
			r.v4Ips[ipv4.String()] = ipv4
			continue
		}

		// Must be v6
		r.v6Ips[ip.String()] = ip
	}
}

func (r *publicIPResolver) resolve(ctx context.Context, resolvers ...ipResolver) error {
	var err error

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	switch len(resolvers) {
	case 0:
		return errors.New("no resolvers have been provided")
	case 1:
		ips, err := resolvers[0](ctx)
		if err != nil {
			r.addIPs(ips)
		}

		return err
	default:
	}

	wg := sync.WaitGroup{}
	ipCtx, ipCancel := context.WithDeadline(ctx, time.Now().Add(time.Second*1))

	defer ipCancel()
	errC := make(chan error)
	errCtx, errCancel := context.WithCancel(ctx)
	defer errCancel()

	// Fire off our error collector
	go func() {
		for {
			select {
			case <-errCtx.Done():
				return
			default:
			}

			select {
			case <-errCtx.Done():
				return
			case err1 := <-errC:
				err = errors.Join(err, err1)
			}
		}
	}()

	// Fire off all of our resolvers and collect the IP addresses resolved by
	// all of them. Only return an error if all resolvers are unable to get an
	// ip address. Return a slice of all unique resolved IP addresses.
	for i := range resolvers {
		wg.Add(1)

		go func() {
			defer wg.Done()
			ips, err := resolvers[i](ipCtx)
			if err != nil {
				select {
				case errC <- err:
				default:
				}

				return
			}

			r.addIPs(ips)
		}()
	}

	wg.Wait()

	if len(r.ips()) == 0 {
		return err
	}

	return nil
}

func (r *publicIPResolver) v4() []net.IP {
	r.m.Lock()
	defer r.m.Unlock()

	if len(r.v4Ips) < 1 {
		return nil
	}

	ips := []net.IP{}
	for _, ip := range r.v4Ips {
		ips = append(ips, ip)
	}

	return ips
}

func (r *publicIPResolver) v4Strings() []string {
	v4s := r.v4()
	ips := []string{}

	for _, ip := range v4s {
		ips = append(ips, ip.String())
	}

	return ips
}

func (r *publicIPResolver) v6() []net.IP {
	r.m.Lock()
	defer r.m.Unlock()

	if len(r.v6Ips) < 1 {
		return nil
	}

	ips := []net.IP{}
	for _, ip := range r.v6Ips {
		ips = append(ips, ip)
	}

	return ips
}

func (r *publicIPResolver) v6Strings() []string {
	v6s := r.v6()
	ips := []string{}

	for _, ip := range v6s {
		ips = append(ips, ip.String())
	}

	return ips
}

func (r *publicIPResolver) ips() []net.IP {
	return append(r.v4(), r.v6()...)
}

func (r *publicIPResolver) ipsStrings() []string {
	return append(r.v4Strings(), r.v6Strings()...)
}
