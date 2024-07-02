package plugin

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// vaultSealsConfig is the Vault Enterprise HA seals block. It supports up to the
// maximum of three HA seals.
type vaultSealsConfig struct {
	Primary   *vaultConfigBlock
	Secondary *vaultConfigBlock
	Tertiary  *vaultConfigBlock
	// keep these around for marshaling the dynamic value
	RawValues map[string]tftypes.Value
	RawValue  tftypes.Value

	Unknown bool
	Null    bool
}

func newVaultSealsConfig() *vaultSealsConfig {
	return &vaultSealsConfig{
		Unknown:   false,
		Null:      true,
		Primary:   newVaultConfigBlock("config", "seals", "primary"),
		Secondary: newVaultConfigBlock("config", "seals", "secondary"),
		Tertiary:  newVaultConfigBlock("config", "seals", "tertiary"),
	}
}

// FromTerraform5Value unmarshals the value to the struct.
func (s *vaultSealsConfig) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return AttributePathError(fmt.Errorf("cannot unmarshal %s into nil vaultSealsConfig", val.String()),
			"config", "seals",
		)
	}

	if val.IsNull() {
		s.Null = true
		s.Unknown = false

		return nil
	}

	if !val.IsKnown() {
		s.Unknown = true

		return nil
	}

	s.Null = false
	s.Unknown = false
	s.RawValue = val
	s.RawValues = map[string]tftypes.Value{}
	err := val.As(&s.RawValues)
	if err != nil {
		return AttributePathError(fmt.Errorf("unable to decode object value: %w", err), "config", "seals")
	}

	// Okay, we've been given either an object or map. Since our input schema is a dynamic pseudo
	// type, the user can pass whatever they want in and terraform won't enforce any schema rules.
	// We'll have ensure what we've been passed in matches what we actually support, otherwise
	// the user could run into a nasty Terraform diagnostic that isn't helpful.

	// Make sure we didn't configure any keys that we don't support.
	for key := range s.RawValues {
		switch key {
		case "primary", "secondary", "tertiary":
		default:
			return AttributePathError(fmt.Errorf("unknown configuration '%s', expected 'primary', 'secondary', or 'tertiary'", key),
				"config", "seals", "primary",
			)
		}
	}

	// We support an object or map. If the user has passed in a map then all thevalue types
	// all must be the same. We'll tell them to redeclare the value as an object and not use
	// strings as the keys to ensure we get an object whose attribute values don't all have to be
	// the same.
	if s.RawValue.Type().Is(tftypes.Map{}) && len(s.RawValues) > 1 {
		var lastType tftypes.Type
		for key, val := range s.RawValues {
			if lastType == nil {
				lastType = val.Type()
				continue
			}

			if !val.Type().Equal(lastType) {
				return AttributePathError(fmt.Errorf(
					"unable to configure more than one seal type as a map value. Try unquoting '%s', and all other seals, to set them as an object attributes", key),
					"config", "seals", key,
				)
			}
			lastType = val.Type()
		}
	}

	primary, ok := s.RawValues["primary"]
	if ok {
		err = s.Primary.FromTerraform5Value(primary)
		if err != nil {
			return AttributePathError(err, "config", "seals", "primary")
		}
	}

	secondary, ok := s.RawValues["secondary"]
	if ok {
		err = s.Secondary.FromTerraform5Value(secondary)
		if err != nil {
			return AttributePathError(err, "config", "seals", "secondary")
		}
	}

	tertiary, ok := s.RawValues["tertiary"]
	if ok {
		err = s.Tertiary.FromTerraform5Value(tertiary)
		if err != nil {
			return AttributePathError(err, "config", "seals", "secondary")
		}
	}

	return nil
}

// Terraform5Type is the tftypes.Type.
func (s *vaultSealsConfig) Terraform5Type() tftypes.Type {
	return tftypes.DynamicPseudoType
}

// Terraform5Value is the tftypes.Value.
func (s *vaultSealsConfig) Terraform5Value() tftypes.Value {
	if s.Null {
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil)
	}

	if s.Unknown {
		return tftypes.NewValue(tftypes.DynamicPseudoType, tftypes.UnknownValue)
	}

	attrs := map[string]tftypes.Type{}
	vals := map[string]tftypes.Value{}
	for name := range s.RawValues {
		switch name {
		case "primary":
			attrs[name] = s.Primary.Terraform5Type()
			vals[name] = s.Primary.Terraform5Value()
		case "secondary":
			attrs[name] = s.Secondary.Terraform5Type()
			vals[name] = s.Secondary.Terraform5Value()
		case "tertiary":
			attrs[name] = s.Tertiary.Terraform5Type()
			vals[name] = s.Secondary.Terraform5Value()
		default:
		}
	}

	if len(vals) == 0 {
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil)
	}

	// Depending on how many are set, Terraform might pass the configuration over
	// as a map or object, so we need to handle both.
	if s.RawValue.Type().Is(tftypes.Map{}) {
		for _, val := range vals {
			return tftypes.NewValue(tftypes.Map{ElementType: val.Type()}, vals)
		}
	}

	return tftypes.NewValue(tftypes.Object{AttributeTypes: attrs}, vals)
}

func (s *vaultSealsConfig) Set(name string, set *vaultConfigBlockSet) error {
	if s == nil {
		return fmt.Errorf("cannot set seal config for %s to nil vaultSealsConfig", name)
	}

	s.Unknown = false
	s.Null = false

	switch name {
	case "primary":
		s.set(s.Primary, set)
	case "secondary":
		s.set(s.Secondary, set)
	case "tertiary":
		s.set(s.Tertiary, set)
	default:
		return fmt.Errorf("unsupport seals name '%s', must be one of 'primary', 'secondary', 'tertiary'", name)
	}

	return nil
}

func (s *vaultSealsConfig) SetSeals(sets map[string]*vaultConfigBlockSet) error {
	if s == nil {
		return errors.New("cannot set seal config for nil vaultSealsConfig")
	}

	for name, set := range sets {
		err := s.Set(name, set)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *vaultSealsConfig) set(blk *vaultConfigBlock, set *vaultConfigBlockSet) {
	if blk == nil {
		nBlk := newVaultConfigBlock()
		blk = nBlk
	}

	blk.Set(set)
}

func (s *vaultSealsConfig) Value() map[string]*vaultConfigBlock {
	if s == nil || s.Unknown || s.Null {
		return nil
	}

	return map[string]*vaultConfigBlock{
		"primary":   s.Primary,
		"secondary": s.Secondary,
		"tertiary":  s.Tertiary,
	}
}

func (s *vaultSealsConfig) needsMultiseal() bool {
	if s == nil {
		return false
	}

	enabled := 0
	if s.Primary != nil && !s.Primary.Null {
		enabled++
	}
	if s.Secondary != nil && !s.Secondary.Null {
		enabled++
	}
	if s.Tertiary != nil && !s.Tertiary.Null {
		enabled++
	}

	//nolint:mnd
	return enabled >= 2
}
