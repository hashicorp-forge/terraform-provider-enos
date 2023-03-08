package plugin

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/hashicorp/enos-provider/internal/log"
	"github.com/hashicorp/enos-provider/internal/remoteflight/systemd"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/ssh"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type sshTransportBuilder func(state *embeddedTransportSSHv1, ctx context.Context) (it.Transport, error)

var defaultSSHTransportBuilder = func(state *embeddedTransportSSHv1, ctx context.Context) (it.Transport, error) {
	sshOpts := []ssh.Opt{
		ssh.WithContext(ctx),
		ssh.WithUser(state.User.Value()),
		ssh.WithHost(state.Host.Value()),
	}

	if key, ok := state.PrivateKey.Get(); ok {
		sshOpts = append(sshOpts, ssh.WithKey(key))
	}

	if keyPath, ok := state.PrivateKeyPath.Get(); ok {
		sshOpts = append(sshOpts, ssh.WithKeyPath(keyPath))
	}

	if pass, ok := state.Passphrase.Get(); ok {
		sshOpts = append(sshOpts, ssh.WithPassphrase(pass))
	}

	if passPath, ok := state.PassphrasePath.Get(); ok {
		sshOpts = append(sshOpts, ssh.WithPassphrasePath(passPath))
	}

	return ssh.New(sshOpts...)
}

var sshAttributes = []string{"user", "host", "private_key", "private_key_path", "passphrase", "passphrase_path"}

var sshTransportTmpl = template.Must(template.New("ssh_transport").Parse(`
    ssh = {
      {{range $key, $val := .}}
      {{if $val.Value}} 
      {{if eq $key "private_key"}}
      {{$key}} = <<EOF
{{$val}}
EOF
      {{else}}
      {{$key}} = "{{$val.Value}}"
      {{end}}
      {{end}}
      {{end}}
    }`))

type embeddedTransportSSHv1 struct {
	sshTransportBuilder  sshTransportBuilder // added in order to support testing
	systemdClientFactory func(transport it.Transport, logger log.Logger) systemd.Client

	User           *tfString
	Host           *tfString
	PrivateKey     *tfString
	PrivateKeyPath *tfString
	Passphrase     *tfString
	PassphrasePath *tfString

	// We have two requirements for the embedded transport: users are able to
	// specify any combination of configuration keys and their associated values,
	// and those values are exportable as an object so that we can easily pass
	// the entire transport output around. To enable the dynamic input and object
	// output we use a DynamicPseudoType for the transport schema instead of
	// concrete nested schemas. This gives us the input and output features we
	// need but also requires us to dynamically generate the object schema
	// depeding on the user input.
	//
	// To generate that schema, we need to keep track of the raw values that are
	// passed over the wire from the user configuration. We'll set these Values
	// when we're unmarshaled and use them later when constructing our marshal
	// schema. Thr marshalled values must contain the same attributes as the unmarshalled values
	// otherwise Terraform will blow up with an error.
	Values map[string]tftypes.Value
}

var _ transportState = (*embeddedTransportSSHv1)(nil)

func newEmbeddedTransportSSH() *embeddedTransportSSHv1 {
	return &embeddedTransportSSHv1{
		sshTransportBuilder:  defaultSSHTransportBuilder,
		systemdClientFactory: systemd.NewClient,
		User:                 newTfString(),
		Host:                 newTfString(),
		PrivateKey:           newTfString(),
		PrivateKeyPath:       newTfString(),
		Passphrase:           newTfString(),
		PassphrasePath:       newTfString(),
		Values:               map[string]tftypes.Value{},
	}
}

func (em *embeddedTransportSSHv1) Terraform5Type() tftypes.Type {
	return terraform5Type(em.Values)
}

func (em *embeddedTransportSSHv1) Terraform5Value() tftypes.Value {
	return terraform5Value(em.Values)
}

func (em *embeddedTransportSSHv1) ApplyDefaults(defaults map[string]TFType) error {
	return applyDefaults(defaults, em.Attributes())
}

func (em *embeddedTransportSSHv1) CopyValues() map[string]tftypes.Value {
	return copyValues(em.Values)
}

func (em *embeddedTransportSSHv1) IsConfigured() bool {
	return isTransportConfigured(em)
}

func (em *embeddedTransportSSHv1) FromTerraform5Value(val tftypes.Value) (err error) {
	em.Values, err = mapAttributesTo(val, map[string]interface{}{
		"user":             em.User,
		"host":             em.Host,
		"private_key":      em.PrivateKey,
		"private_key_path": em.PrivateKeyPath,
		"passphrase":       em.Passphrase,
		"passphrase_path":  em.PassphrasePath,
	})
	if err != nil {
		return AttributePathError(
			fmt.Errorf("failed to convert terraform value to 'SSH' transport config, due to: %w", err),
			"transport", "ssh",
		)
	}
	return verifyConfiguration(sshAttributes, em.Values, "ssh")
}

func (em *embeddedTransportSSHv1) Validate(ctx context.Context) error {
	if _, ok := em.User.Get(); !ok {
		return ValidationError("you must provide the transport SSH user", "transport", "ssh", "user")
	}

	if _, ok := em.Host.Get(); !ok {
		return ValidationError("you must provide the transport SSH host", "transport", "ssh", "host")
	}

	_, okpk := em.PrivateKey.Get()
	_, okpkp := em.PrivateKeyPath.Get()
	if !okpk && !okpkp {
		return ValidationError("you must provide either the private_key or private_key_path", "transport", "ssh", "private_key")
	}

	return nil
}

func (em *embeddedTransportSSHv1) Client(ctx context.Context) (it.Transport, error) {
	return em.sshTransportBuilder(em, ctx)
}

func (em *embeddedTransportSSHv1) Attributes() map[string]TFType {
	return map[string]TFType{
		"user":             em.User,
		"host":             em.Host,
		"private_key":      em.PrivateKey,
		"private_key_path": em.PrivateKeyPath,
		"passphrase":       em.Passphrase,
		"passphrase_path":  em.PassphrasePath,
	}
}

func (em *embeddedTransportSSHv1) GetAttributesForReplace() []string {
	if _, ok := em.Values["host"]; ok {
		return []string{"host"}
	}
	return []string{}
}

func (em *embeddedTransportSSHv1) Type() TransportType {
	return SSH
}

// render renders the transport to terraform
func (em *embeddedTransportSSHv1) render() (string, error) {
	buf := bytes.Buffer{}
	if err := sshTransportTmpl.Execute(&buf, em.Attributes()); err != nil {
		return "", fmt.Errorf("failed to render ssh transport config, due to: %w", err)
	}

	return buf.String(), nil
}

func (em *embeddedTransportSSHv1) debug() string {
	maxWidth := 0
	attributes := em.Attributes()
	for name := range attributes {
		if len(name) > maxWidth {
			maxWidth = len(name)
		}
	}

	var vals []string
	for _, name := range sshAttributes {
		val := "null"
		if value, ok := attributes[name]; ok && !value.TFValue().IsNull() {
			if name == "passphrase" {
				val = "[redacted]"
			} else {
				val = value.String()
			}
		}
		vals = append(vals, fmt.Sprintf("%*s : %s", maxWidth, name, val))
	}

	return fmt.Sprintf("SSH Transport Config:\n%s", strings.Join(vals, "\n"))
}

// systemdClient creates a new systemd client for this transport
func (em *embeddedTransportSSHv1) systemdClient(ctx context.Context, logger log.Logger) (systemd.Client, error) {
	client, err := em.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh client, due to: %w", err)
	}

	return em.systemdClientFactory(client, logger), nil
}
