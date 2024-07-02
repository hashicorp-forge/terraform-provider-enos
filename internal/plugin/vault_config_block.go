package plugin

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultConfigBlock struct {
	AttributePaths []string // the attribute path to the vault config block
	Type           *tfString
	Attrs          *tfObject
	AttrsValues    map[string]tftypes.Value
	AttrsRaw       tftypes.Value

	Unknown bool
	Null    bool
}

type vaultConfigBlockSet struct {
	typ   string
	attrs map[string]any
	paths []string
}

func newVaultConfigBlock(attributePaths ...string) *vaultConfigBlock {
	return &vaultConfigBlock{
		AttributePaths: attributePaths,
		Attrs:          newTfObject(),
		AttrsValues:    map[string]tftypes.Value{},
		Type:           newTfString(),
		Unknown:        false,
		Null:           true,
	}
}

func newVaultConfigBlockSet(typ string, attrs map[string]any, paths ...string) *vaultConfigBlockSet {
	return &vaultConfigBlockSet{
		typ:   typ,
		attrs: attrs,
		paths: paths,
	}
}

func (s *vaultConfigBlock) Set(set *vaultConfigBlockSet) {
	if s == nil || set == nil {
		return
	}

	s.Unknown = false
	s.Null = false
	s.AttributePaths = set.paths
	s.Type.Set(set.typ)
	s.Attrs.Set(set.attrs)
}

// FromTerraform5Value unmarshals the value to the struct.
func (s *vaultConfigBlock) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal %s into nil vaultConfigBlock", val.String())
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

	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return err
	}

	// Since attributes is a dynamic pseudo type we have to decode it only
	// if it's known.
	for k, v := range vals {
		switch k {
		case "type":
			err = s.Type.FromTFValue(v)
			if err != nil {
				return err
			}
		case "attributes":
			if !v.IsKnown() {
				// Attrs are a DynamicPseudoType but the value is unknown. Terraform expects us to be a
				// dynamic value that we'll know after apply.
				s.Attrs.Unknown = true
				continue
			}
			if v.IsNull() {
				// We can't unmarshal null or unknown things
				continue
			}

			s.AttrsRaw = v
			err = v.As(&s.AttrsValues)
			if err != nil {
				return err
			}
			err = s.Attrs.FromTFValue(v)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported attribute in vault config block: %s", k)
		}
	}

	return err
}

// Terraform5Type is the tftypes.Type.
func (s *vaultConfigBlock) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       s.Type.TFType(),
		"attributes": tftypes.DynamicPseudoType,
	}}
}

// Terraform5Value is the tftypes.Value.
func (s *vaultConfigBlock) Terraform5Value() tftypes.Value {
	if s.Unknown {
		return tftypes.NewValue(s.Terraform5Type(), tftypes.UnknownValue)
	}

	if s.Null {
		return tftypes.NewValue(s.Terraform5Type(), nil)
	}

	var attrsVal tftypes.Value

	if s.AttrsRaw.Type() == nil {
		// We don't have a type, which means we're a DynamicPseudoType with either a nil or unknown
		// value.
		if s.Attrs.Unknown {
			attrsVal = tftypes.NewValue(tftypes.DynamicPseudoType, tftypes.UnknownValue)
		} else {
			attrsVal = tftypes.NewValue(tftypes.DynamicPseudoType, nil)
		}
	} else {
		var err error
		attrsVal, err = encodeTfObjectDynamicPseudoType(s.AttrsRaw, s.AttrsValues)
		if err != nil {
			attrsVal = tftypes.NewValue(tftypes.DynamicPseudoType, nil)
		}
	}

	return terraform5Value(map[string]tftypes.Value{
		"type":       s.Type.TFValue(),
		"attributes": attrsVal,
	})
}
