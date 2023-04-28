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
	t.Parallel()

	_, ok := os.LookupEnv("TF_ACC")
	if !ok {
		t.Log("skipping public ip address resolution because TF_ACC is not set")
		t.Skip()

		return
	}

	pubip := newPublicIPResolver()
	// Test that we can resolve something out of all of our providers
	t.Run("defaultResolvers_all", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := pubip.resolve(ctx, defaultResolvers()...)
		require.NoError(t, err)
		t.Logf("v4 ips %v", pubip.v4Strings())
		t.Logf("v6 ips %v", pubip.v6Strings())
		require.NotEmpty(t, pubip.ips())
	})

	// Since ISPs run things differently not all of these will work for everyone.
	// Only warn if they can't resolve an IP address
	t.Run("defaultResolvers_each", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, resolver := range defaultResolvers() {
			err := pubip.resolve(ctx, resolver)
			if err != nil {
				t.Logf("Warning, a DNS resolver failed to resolve a public IP addr. %v", err)
			}
		}
	})
}

type byIP []net.IP

func (a byIP) Len() int           { return len(a) }
func (a byIP) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byIP) Less(i, j int) bool { return a[i].String() < a[j].String() }

func TestPublicIPAddressResolver_add_ips(t *testing.T) {
	t.Parallel()
	wg := sync.WaitGroup{}
	pubip := newPublicIPResolver()

	for _, ips := range [][]net.IP{
		{net.ParseIP("2605:56c8:517b:cb10:dc78:62b9:61ce:8243")},
		{net.ParseIP("2603:6b11:2c99:cda1:e12b:aef1:b3d2:8241")},
		{net.ParseIP("2603:6b11:2c99:cda1:e12b:aef1:b3d2:8241"), net.ParseIP("2605:56c8:517b:cb10:dc78:62b9:61ce:8243")},
		{net.ParseIP("1.2.3.4"), net.ParseIP("1.2.3.4")},
		{net.ParseIP("3.4.5.6")},
		{net.ParseIP("1.2.3.4"), net.ParseIP("3.4.5.6")},
		{net.ParseIP("3.4.5.6"), net.ParseIP("3.4.5.6"), net.ParseIP("1.2.3.4")},
		{
			net.ParseIP("2603:6b11:2c99:cda1:e12b:aef1:b3d2:8241"),
			net.ParseIP("3.4.5.6"),
			net.ParseIP("1.2.3.4"),
			net.ParseIP("2605:56c8:517b:cb10:dc78:62b9:61ce:8243"),
			net.ParseIP("3.4.5.6"),
		},
	} {
		ips := ips
		wg.Add(1)

		go func() {
			defer wg.Done()
			pubip.addIPs(ips)
		}()
	}

	expectedV4 := []net.IP{
		net.ParseIP("1.2.3.4").To4(),
		net.ParseIP("3.4.5.6").To4(),
	}
	expectedV6 := []net.IP{
		net.ParseIP("2603:6b11:2c99:cda1:e12b:aef1:b3d2:8241"),
		net.ParseIP("2605:56c8:517b:cb10:dc78:62b9:61ce:8243"),
	}
	expectedAll := []net.IP{
		net.ParseIP("1.2.3.4").To4(),
		net.ParseIP("3.4.5.6").To4(),
		net.ParseIP("2603:6b11:2c99:cda1:e12b:aef1:b3d2:8241"),
		net.ParseIP("2605:56c8:517b:cb10:dc78:62b9:61ce:8243"),
	}

	sort.Sort(byIP(expectedV4))
	sort.Sort(byIP(expectedV6))
	sort.Sort(byIP(expectedAll))

	wg.Wait()

	gotV4 := pubip.v4()
	gotV6 := pubip.v6()
	gotAll := pubip.ips()
	sort.Sort(byIP(gotV4))
	sort.Sort(byIP(gotV6))
	sort.Sort(byIP(gotAll))

	require.EqualValues(t, expectedV4, gotV4)
	require.EqualValues(t, expectedV6, gotV6)
	require.EqualValues(t, expectedAll, gotAll)
}
