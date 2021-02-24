package plugin

import (
	"context"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/ssh"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

// embeddedTransportV1 represents the embedded transport state for all
// resources and data source. It is intended to be used as ouput from the
// transport data source and used as the transport input for all resources.
type embeddedTransportV1 struct {
	SSH *embeddedTransportSSHv1
}

type embeddedTransportSSHv1 struct {
	User           string
	Host           string
	PrivateKey     string
	PrivateKeyPath string
	Passphrase     string
	PassphrasePath string

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
	// schema.
	Values map[string]tftypes.Value
}

func newEmbeddedTransport() *embeddedTransportV1 {
	return &embeddedTransportV1{
		SSH: &embeddedTransportSSHv1{
			Values: map[string]tftypes.Value{},
		},
	}
}

// SchemaAttributeTransport is our transport schema configuration attribute.
// Resources that embed a transport should use this as transport schema.
func (em *embeddedTransportV1) SchemaAttributeTransport() *tfprotov5.SchemaAttribute {
	return &tfprotov5.SchemaAttribute{
		Name:     "transport",
		Type:     em.Terraform5Type(),
		Optional: true, // We'll handle our own schema validation
	}
}

// SchemaAttributeOut is our transport schema output object. The transport data
// source can use this to output the user configuration as an object.
func (em *embeddedTransportV1) SchemaAttributeOut() *tfprotov5.SchemaAttribute {
	return &tfprotov5.SchemaAttribute{
		Name:     "out",
		Type:     em.Terraform5Type(),
		Computed: true,
	}
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Value with As().
func (em *embeddedTransportV1) FromTerraform5Value(val tftypes.Value) error {
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return err
	}

	if !vals["ssh"].IsKnown() || vals["ssh"].IsNull() {
		return nil
	}

	em.SSH.Values, err = mapAttributesTo(vals["ssh"], map[string]interface{}{
		"user":             &em.SSH.User,
		"host":             &em.SSH.Host,
		"private_key":      &em.SSH.PrivateKey,
		"private_key_path": &em.SSH.PrivateKeyPath,
		"passphrase":       &em.SSH.Passphrase,
		"passphrase_path":  &em.SSH.PassphrasePath,
	})
	if err != nil {
		return wrapErrWithDiagnostics(err, "invalid configuration syntax",
			"unable to marshal transport SSH values", "transport", "ssh",
		)
	}

	return nil
}

// Terraform5Type is the tftypes.Type
func (em *embeddedTransportV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"ssh": tftypes.DynamicPseudoType,
		},
	}
}

// Terraform5Type is the tftypes.Value
func (em *embeddedTransportV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(em.Terraform5Type(), map[string]tftypes.Value{
		"ssh": em.Terraform5ValueSSH(),
	})
}

// Terraform5TypeSSH is the dynamically generated SSH tftypes.Type. It must
// always match the schema that is passed in as user configuration.
func (em *embeddedTransportV1) Terraform5TypeSSH() tftypes.Type {
	defaultTypes := map[string]tftypes.Type{
		"user":             tftypes.String,
		"host":             tftypes.String,
		"private_key":      tftypes.String,
		"private_key_path": tftypes.String,
		"passphrase":       tftypes.String,
		"passphrase_path":  tftypes.String,
	}

	// The SSH Values are set if the struct is built dynamically from user
	// config. If it's not set then return all of our defaults.
	if len(em.SSH.Values) == 0 {
		return tftypes.Object{AttributeTypes: defaultTypes}
	}

	newTypes := map[string]tftypes.Type{}
	for name := range em.SSH.Values {
		newTypes[name] = defaultTypes[name]
	}

	return tftypes.Object{AttributeTypes: newTypes}
}

// Terraform5ValueSSH is the dynamically generated SSH tftypes.Value. It must
// always match the schema that is passed in as user configuration.
func (em *embeddedTransportV1) Terraform5ValueSSH() tftypes.Value {
	defaultValues := map[string]tftypes.Value{
		"user":             stringValue(em.SSH.User),
		"host":             stringValue(em.SSH.Host),
		"private_key":      stringValue(em.SSH.PrivateKey),
		"private_key_path": stringValue(em.SSH.PrivateKeyPath),

		"passphrase":      stringValue(em.SSH.Passphrase),
		"passphrase_path": stringValue(em.SSH.PassphrasePath),
	}

	// The SSH Values are set if the struct is built dynamically from user
	// config. If it's not set then return all of our defaults.
	if len(em.SSH.Values) == 0 {
		return tftypes.NewValue(em.Terraform5TypeSSH(), defaultValues)
	}

	newValues := map[string]tftypes.Value{}
	for name, val := range em.SSH.Values {
		setVal, ok := defaultValues[name]
		if ok {
			newValues[name] = setVal
			continue
		}

		newValues[name] = val
	}

	return tftypes.NewValue(em.Terraform5TypeSSH(), newValues)
}

// Validate validates that transport can use the given configuration as a
// transport. Be warned that this will read and path based configuration and
// attempt to parse any keys.
func (em *embeddedTransportV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Assume SSH since it's the only allowed transport
	if em.SSH.User == "" {
		return newErrWithDiagnostics("Invalid configuration", "you must provide the transport SSH user", "ssh", "user")
	}

	if em.SSH.Host == "" {
		return newErrWithDiagnostics("Invalid configuration", "you must provide the transport SSH host", "ssh", "host")
	}

	if em.SSH.PrivateKey == "" && em.SSH.PrivateKeyPath == "" {
		return newErrWithDiagnostics("Invalid configuration", "you must provide either the private_key or private_key_path", "ssh", "private_key")
	}

	// Create a new SSH client with out options. This doesn't initialize a new
	// session but it will attempt to read and parse any keys or passphrases.
	sshOpts := []ssh.Opt{
		ssh.WithContext(ctx),
		ssh.WithUser(em.SSH.User),
		ssh.WithHost(em.SSH.Host),
	}

	if em.SSH.PrivateKey != "" {
		sshOpts = append(sshOpts, ssh.WithKey(em.SSH.PrivateKey))
	}

	if em.SSH.PrivateKeyPath != "" {
		sshOpts = append(sshOpts, ssh.WithKeyPath(em.SSH.PrivateKeyPath))
	}

	if em.SSH.Passphrase != "" {
		sshOpts = append(sshOpts, ssh.WithPassphrase(em.SSH.Passphrase))
	}

	if em.SSH.PassphrasePath != "" {
		sshOpts = append(sshOpts, ssh.WithPassphrasePath(em.SSH.PassphrasePath))
	}

	_, err := ssh.New(sshOpts...)
	if err != nil {
		return wrapErrWithDiagnostics(err,
			"Invalid configuration",
			"Unable to create SSH client from transport configuration",
			"transport", "ssh",
		)
	}

	return nil
}

// Client returns a Transport client that be used to perform actions against
// the target that has been configured.
func (em *embeddedTransportV1) Client(ctx context.Context) (it.Transport, error) {
	sshOpts := []ssh.Opt{
		ssh.WithContext(ctx),
		ssh.WithUser(em.SSH.User),
		ssh.WithHost(em.SSH.Host),
	}

	if key := em.SSH.PrivateKey; key != "" {
		sshOpts = append(sshOpts, ssh.WithKey(key))
	}

	if keyPath := em.SSH.PrivateKeyPath; keyPath != "" {
		sshOpts = append(sshOpts, ssh.WithKeyPath(keyPath))
	}

	if pass := em.SSH.Passphrase; pass != "" {
		sshOpts = append(sshOpts, ssh.WithPassphrase(pass))
	}

	if passPath := em.SSH.PassphrasePath; passPath != "" {
		sshOpts = append(sshOpts, ssh.WithPassphrasePath(passPath))
	}

	return ssh.New(sshOpts...)
}
