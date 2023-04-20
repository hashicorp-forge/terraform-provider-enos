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

const (
	dnsTypeIPV4Host dnsQtype  = dnsQtype(dns.TypeA)
	dnsTypeTXT      dnsQtype  = dnsQtype(dns.TypeTXT)
	dnsClassInet    dnsQClass = dnsQClass(dns.ClassINET)
	dnsClassChaos   dnsQClass = dnsQClass(dns.ClassCHAOS)
)

type ipResolver func(context.Context) ([]net.IP, error)

type publicIPResolver struct {
	ips []net.IP
	m   sync.Mutex
}

func newPublicIPResolver() *publicIPResolver {
	return &publicIPResolver{
		ips: []net.IP{},
		m:   sync.Mutex{},
	}
}

func defaultResolvers() []ipResolver {
	return []ipResolver{
		withDNSResolver(
			"resolver1.opendns.com:53", "myip.opendns.com.",
			dnsClassInet, dnsTypeIPV4Host,
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

		client := dns.Client{}
		res, _, err := client.ExchangeContext(ctx, msg, nameserver)
		if err != nil {
			return nil, err
		}

		if len(res.Answer) < 1 {
			return nil, fmt.Errorf("no records found")
		}

		ips := []net.IP{}
		for _, ans := range res.Answer {
			switch dnsQtype(ans.Header().Rrtype) {
			case dnsTypeIPV4Host:
				a, ok := ans.(*dns.A)
				if !ok {
					return ips, fmt.Errorf("unable to convert answer to correct type. Expected A got %T", ans)
				}
				ips = append(ips, a.A)
			case dnsTypeTXT:
				t, ok := ans.(*dns.TXT)
				if !ok {
					return ips, fmt.Errorf("unable to convert answer to correct type. Expected TXT got %T", ans)
				}

				if len(t.Txt) < 1 {
					return ips, fmt.Errorf("no ips found in txt record")
				}

				for _, txt := range t.Txt {
					ips = append(ips, net.ParseIP(txt))
				}
			default:
				return ips, fmt.Errorf("unsupported answer type. Expected A got %T", ans)
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

		req, err := http.NewRequest("GET", host, nil)
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		return []net.IP{net.ParseIP(strings.TrimSpace(string(body)))}, nil
	}
}

// addIPs takes a slice of IPs and adds them to the ip set
func (r *publicIPResolver) addIPs(ips []net.IP) {
	r.m.Lock()
	defer r.m.Unlock()

	updatedIPs := map[string]net.IP{}
	for _, ip := range append(r.ips, ips...) {
		if ip == nil || ip.String() == "" {
			continue
		}
		updatedIPs[ip.String()] = ip
	}

	r.ips = []net.IP{}
	for _, v := range updatedIPs {
		r.ips = append(r.ips, v)
	}
}

func (r *publicIPResolver) resolve(ctx context.Context, resolvers ...ipResolver) ([]net.IP, error) {
	var err error

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	switch len(resolvers) {
	case 0:
		return nil, fmt.Errorf("no resolvers have been provided")
	case 1:
		ips, err := resolvers[0](ctx)
		if err != nil {
			r.addIPs(ips)
		}
		return r.ips, err
	default:
	}

	wg := sync.WaitGroup{}
	ipCtx, ipCancel := context.WithDeadline(ctx, time.Now().Add(time.Millisecond*500))

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
		i := i
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

	if len(r.ips) > 0 {
		return r.ips, nil
	}

	return r.ips, err
}
