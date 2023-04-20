package plugin

import (
	"context"
	"net"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccPublicIPAddressResolver(t *testing.T) {
	_, ok := os.LookupEnv("TF_ACC")
	if !ok {
		t.Log("skipping public ip address resolution because TF_ACC is not set")
		t.Skip()
		return
	}

	pubip := newPublicIPResolver()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	// This will test resolving multiple or singluar resolvers and ensure
	// that an address is returned. Since we can technically get multiple
	// different values we should not validate that they always match.
	for _, resolvers := range [][]ipResolver{
		defaultResolvers(),
		{withDNSResolver(
			"resolver1.opendns.com:53", "myip.opendns.com.",
			dnsClassInet, dnsTypeIPV4Host,
		)},
		{withDNSResolver(
			"ns1.google.com:53", "o-o.myaddr.l.google.com.",
			dnsClassInet, dnsTypeTXT,
		)},
		{withDNSResolver(
			"one.one.one.one:53", "whoami.cloudflare.",
			dnsClassChaos, dnsTypeTXT,
		)},
		{withHTTPSBodyResolver("https://checkip.amazonaws.com")},
	} {
		ips, err := pubip.resolve(ctx, resolvers...)
		t.Logf("ips %v", ips)
		require.NoError(t, err)
		require.NotEmpty(t, ips)
	}
}

type byIP []net.IP

func (a byIP) Len() int           { return len(a) }
func (a byIP) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byIP) Less(i, j int) bool { return a[i].String() < a[j].String() }

func TestPublicIPAddressResolver_add_ips(t *testing.T) {
	wg := sync.WaitGroup{}
	pubip := newPublicIPResolver()

	for _, ips := range [][]net.IP{
		{net.ParseIP("1.2.3.4"), net.ParseIP("1.2.3.4")},
		{net.ParseIP("3.4.5.6")},
		{net.ParseIP("1.2.3.4"), net.ParseIP("3.4.5.6")},
		{net.ParseIP("3.4.5.6"), net.ParseIP("3.4.5.6"), net.ParseIP("1.2.3.4")},
	} {
		ips := ips
		wg.Add(1)

		go func() {
			defer wg.Done()
			pubip.addIPs(ips)
		}()
	}

	expected := []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("3.4.5.6")}
	sort.Sort(byIP(expected))

	wg.Wait()

	got := pubip.ips
	sort.Sort(byIP(got))
	require.EqualValues(t, expected, got)
}
