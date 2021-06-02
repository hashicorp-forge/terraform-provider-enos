package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/ssh"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// embeddedTransportV1 represents the embedded transport state for all
// resources and data source. It is intended to be used as ouput from the
// transport data source and used as the transport input for all resources.
type embeddedTransportV1 struct {
	mu  sync.Mutex
	SSH *embeddedTransportSSHv1 `json:"ssh"`
}

type embeddedTransportSSHv1 struct {
	User           string `json:"user,omitempty"`
	Host           string `json:"host,omitempty"`
	PrivateKey     string `json:"private_key,omitempty"`
	PrivateKeyPath string `json:"private_key_path,omitempty"`
	Passphrase     string `json:"passphrase,omitempty"`
	PassphrasePath string `json:"passphrase_path,omitempty"`

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

// embeddedTransportPrivate is out private state
type embeddedTransportPrivate struct {
	Host string `json:"host,omitempty"`
}

func newEmbeddedTransport() *embeddedTransportV1 {
	return &embeddedTransportV1{
		mu: sync.Mutex{},
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
	em.mu.Lock()
	defer em.mu.Unlock()

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

	// Because the SSH type is a dynamic psuedo type we have to manually ensure
	// that the user hasn't set any unknown attributes.
	knownSSHAttr := func(attr string) error {
		for known := range em.defaultSSHTypes() {
			if attr == known {
				return nil
			}
		}

		return newErrWithDiagnostics("Unsupported argument",
			fmt.Sprintf(`An argument named "%s" is not expected here.`, attr), "transport", "ssh", attr,
		)
	}

	for sshAttr := range em.SSH.Values {
		err := knownSSHAttr(sshAttr)
		if err != nil {
			return err
		}
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
	if len(em.SSH.Values) == 0 {
		return tftypes.NewValue(em.Terraform5Type(), nil)
	}

	return tftypes.NewValue(em.Terraform5Type(), map[string]tftypes.Value{
		"ssh": em.Terraform5ValueSSH(),
	})
}

// Terraform5TypeSSH is the dynamically generated SSH tftypes.Type. It must
// always match the schema that is passed in as user configuration.
func (em *embeddedTransportV1) Terraform5TypeSSH() tftypes.Type {
	newTypes := map[string]tftypes.Type{}
	defaultTypes := em.defaultSSHTypes()
	for name := range em.SSH.Values {
		newTypes[name] = defaultTypes[name]
	}

	return tftypes.Object{AttributeTypes: newTypes}
}

// Terraform5ValueSSH is the dynamically generated SSH tftypes.Value. It must
// always match the schema that is passed in as user configuration.
func (em *embeddedTransportV1) Terraform5ValueSSH() tftypes.Value {
	newValues := map[string]tftypes.Value{}
	defaultValues := em.defaultSSHValues()
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

func (em *embeddedTransportV1) defaultSSHTypes() map[string]tftypes.Type {
	return map[string]tftypes.Type{
		"user":             tftypes.String,
		"host":             tftypes.String,
		"private_key":      tftypes.String,
		"private_key_path": tftypes.String,
		"passphrase":       tftypes.String,
		"passphrase_path":  tftypes.String,
	}
}

func (em *embeddedTransportV1) defaultSSHValues() map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"user":             tfMarshalStringValue(em.SSH.User),
		"host":             tfMarshalStringValue(em.SSH.Host),
		"private_key":      tfMarshalStringValue(em.SSH.PrivateKey),
		"private_key_path": tfMarshalStringValue(em.SSH.PrivateKeyPath),
		"passphrase":       tfMarshalStringValue(em.SSH.Passphrase),
		"passphrase_path":  tfMarshalStringValue(em.SSH.PassphrasePath),
	}
}

func (em *embeddedTransportV1) Copy() (*embeddedTransportV1, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	newCopy := newEmbeddedTransport()

	self, err := json.Marshal(em)
	if err != nil {
		return newCopy, err
	}

	err = json.Unmarshal(self, newCopy)
	if err != nil {
		return newCopy, err
	}

	for k, v := range em.SSH.Values {
		newCopy.SSH.Values[k] = v
	}

	return newCopy, nil
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

func (em *embeddedTransportV1) FromEnvironment() {
	em.mu.Lock()
	defer em.mu.Unlock()

	for _, key := range []struct {
		name string
		env  string
		dst  *string
	}{
		{"user", "ENOS_TRANSPORT_USER", &em.SSH.User},
		{"host", "ENOS_TRANSPORT_HOST", &em.SSH.Host},
		{"private_key", "ENOS_TRANSPORT_PRIVATE_KEY", &em.SSH.PrivateKey},
		{"private_key_path", "ENOS_TRANSPORT_PRIVATE_KEY_PATH", &em.SSH.PrivateKeyPath},
		{"passphrase", "ENOS_TRANSPORT_PASSPHRASE", &em.SSH.Passphrase},
		{"passphrase_path", "ENOS_TRANSPORT_PASSPHRASE_PATH", &em.SSH.PassphrasePath},
	} {
		val, ok := os.LookupEnv(key.env)
		if ok {
			*key.dst = val
			em.SSH.Values[key.name] = tfMarshalStringValue(val)
		}
	}
}

// MergeInto merges the embeddedTranspor into another instance.
func (em *embeddedTransportV1) MergeInto(defaults *embeddedTransportV1) error {
	defaults.mu.Lock()
	defer defaults.mu.Unlock()
	em.mu.Lock()
	defer em.mu.Unlock()

	startingVals := defaults.SSH.Values

	overJSON, err := json.Marshal(em)
	if err != nil {
		return err
	}

	err = json.Unmarshal(overJSON, defaults)
	if err != nil {
		return err
	}

	defaults.SSH.Values = startingVals
	for key, val := range em.SSH.Values {
		defaults.SSH.Values[key] = val
	}

	return nil
}

// ToPrivate returns the embeddedTransportV1's private state
func (em *embeddedTransportV1) ToPrivate() ([]byte, error) {
	// We keep the host in private state because we need it to determine if
	// the resource has changed. As we allow users to specify _all_ transport
	// level configuration at the provider level, we cannot rely on saving it
	// in the normal transport schema.
	p := &embeddedTransportPrivate{
		Host: em.SSH.Host,
	}

	return json.Marshal(p)
}

// FromPrivate loads the private state into the embeddedTransport
func (em *embeddedTransportV1) FromPrivate(in []byte) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if len(in) == 0 {
		return nil
	}

	p := &embeddedTransportPrivate{}
	err := json.Unmarshal(in, p)
	if err != nil {
		return err
	}

	em.SSH.Host = p.Host

	return nil
}

func transportReplacedAttributePaths(prior, proposed *embeddedTransportV1) []*tftypes.AttributePath {
	attrs := []*tftypes.AttributePath{}

	if prior.SSH.Host != "" && proposed.SSH.Host != "" && (prior.SSH.Host != proposed.SSH.Host) {
		attrs = append(attrs, tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
			tftypes.AttributeName("transport"),
			tftypes.AttributeName("ssh"),
			tftypes.AttributeName("host"),
		}))
	}

	if len(attrs) > 0 {
		return attrs
	}

	return nil
}
