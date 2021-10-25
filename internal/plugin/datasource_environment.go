package plugin

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type environment struct {
	providerConfig *config
}

var _ datarouter.DataSource = (*environment)(nil)

type environmentStateV1 struct {
	ID              *tfString
	PublicIPAddress *tfString
}

type publicIPResolver struct{}

var _ State = (*environmentStateV1)(nil)

func newEnvironment() *environment {
	return &environment{
		providerConfig: newProviderConfig(),
	}
}

func newEnvironmentStateV1() *environmentStateV1 {
	return &environmentStateV1{
		ID:              newTfString(),
		PublicIPAddress: newTfString(),
	}
}

func newPublicIPResolver() *publicIPResolver {
	return &publicIPResolver{}
}

func (d *environment) Name() string {
	return "enos_environment"
}

func (d *environment) Schema() *tfprotov6.Schema {
	return newEnvironmentStateV1().Schema()
}

func (d *environment) SetProviderConfig(meta tftypes.Value) error {
	return d.providerConfig.FromTerraform5Value(meta)
}

// ValidateDataResourceConfig is the request Terraform sends when it wants to
// validate the data source's configuration.
func (d *environment) ValidateDataResourceConfig(ctx context.Context, req *tfprotov6.ValidateDataResourceConfigRequest) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	res := &tfprotov6.ValidateDataResourceConfigResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	// unmarshal it to our known type to ensure whatever was passed in matches
	// the correct schema.
	newConfig := newEnvironmentStateV1()
	err := unmarshal(newConfig, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
	}

	return res, err
}

// ReadDataSource is the request Terraform sends when it wants to get the latest
// state for the data source.
func (d *environment) ReadDataSource(ctx context.Context, req *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	res := &tfprotov6.ReadDataSourceResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	newState := newEnvironmentStateV1()

	// unmarshal and re-marshal the state to add default fields
	err := unmarshal(newState, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
	newState.ID.Set("static")

	resolver := newPublicIPResolver()
	ip, err := resolver.Resolve(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(wrapErrWithDiagnostics(
			err, "ip address", "failed to resolve public IP address",
		)))
		return res, err
	}
	newState.PublicIPAddress.Set(ip.String())

	res.State, err = marshal(newState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, nil
}

// Schema is the file states Terraform schema.
func (s *environmentStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "public_ip_address",
					Type:     tftypes.String,
					Computed: true,
				},
			},
		},
	}
}

// Validate validates the configuration.
func (s *environmentStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *environmentStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"id":                s.ID,
		"public_ip_address": s.PublicIPAddress,
	})

	return err
}

// Terraform5Type is the file state tftypes.Type.
func (s *environmentStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":                s.ID.TFType(),
		"public_ip_address": s.PublicIPAddress.TFType(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *environmentStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":                s.ID.TFValue(),
		"public_ip_address": s.PublicIPAddress.TFValue(),
	})
}

func (r *publicIPResolver) Resolve(ctx context.Context) (net.IP, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try all of our resolvers one-by-one until we successfully resolve the
	// public IP address. Start with the DNS resolvers as they have far less
	// overhead and will try both UDP and TCP. If they both fail for some reason
	// fall back to the HTTPS AWS resolver.
	ip, err := r.resolveOpenDNS(ctx)
	if err == nil {
		return ip, err
	}
	merr := &multierror.Error{}
	merr = multierror.Append(merr, err)

	ip, err = r.resolveGoogle(ctx)
	if err == nil {
		return ip, err
	}
	merr = multierror.Append(merr, err)

	ip, err = r.resolveAWS(ctx)
	if err == nil {
		return ip, err
	}
	merr = multierror.Append(merr, err)

	return nil, merr.ErrorOrNil()
}

func (r *publicIPResolver) resolverFor(addr string) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp4", addr)
		},
	}
}

func (r *publicIPResolver) resolveOpenDNS(ctx context.Context) (net.IP, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	openDNS := r.resolverFor("resolver1.opendns.com:53")
	ips, err := openDNS.LookupHost(ctx, "myip.opendns.com")
	if err != nil {
		return nil, err
	}

	return net.ParseIP(ips[0]), nil
}

func (r *publicIPResolver) resolveGoogle(ctx context.Context) (net.IP, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	google := r.resolverFor("ns1.google.com:53")
	ips, err := google.LookupTXT(ctx, "o-o.myaddr.l.google.com")
	if err != nil {
		return nil, err
	}

	return net.ParseIP(ips[0]), nil
}

func (r *publicIPResolver) resolveAWS(ctx context.Context) (net.IP, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	req, err := http.NewRequest("GET", "https://checkip.amazonaws.com", nil)
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

	return net.ParseIP(strings.TrimSpace(string(body))), nil
}
