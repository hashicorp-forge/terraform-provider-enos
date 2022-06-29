package plugin

import (
	"context"

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

type embeddedTransportSSHv1 struct {
	sshTransportBuilder sshTransportBuilder // added in order to support testing

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
		sshTransportBuilder: defaultSSHTransportBuilder,
		User:                newTfString(),
		Host:                newTfString(),
		PrivateKey:          newTfString(),
		PrivateKeyPath:      newTfString(),
		Passphrase:          newTfString(),
		PassphrasePath:      newTfString(),
		Values:              map[string]tftypes.Value{},
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
		return wrapErrWithDiagnostics(err, "invalid configuration syntax",
			"unable to marshal transport SSH values", "transport", "ssh",
		)
	}
	return verifyConfiguration(sshAttributes, em.Values, "ssh")
}

func (em *embeddedTransportSSHv1) Validate(ctx context.Context) error {
	if _, ok := em.User.Get(); !ok {
		return newErrWithDiagnostics("Invalid Transport Configuration", "you must provide the transport SSH user", "transport", "ssh", "user")
	}

	if _, ok := em.Host.Get(); !ok {
		return newErrWithDiagnostics("Invalid Transport Configuration", "you must provide the transport SSH host", "transport", "ssh", "host")
	}

	_, okpk := em.PrivateKey.Get()
	_, okpkp := em.PrivateKeyPath.Get()
	if !okpk && !okpkp {
		return newErrWithDiagnostics("Invalid Transport Configuration", "you must provide either the private_key or private_key_path", "transport", "ssh", "private_key")
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
