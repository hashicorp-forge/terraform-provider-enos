package hcl

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"

	"github.com/zclconf/go-cty/cty/gocty"

	"github.com/hashicorp/hcl/v2/hclwrite"
)

// hclEntry is any item that can be appended into an *hclwrite.Body, such as a block or an attribute.
type hclEntry interface {
	// appendTo appends this item to the provided body
	appendTo(*hclwrite.Body) error
}

// Builder can be used to build structured hcl content, supporting appending attributes and blocks.
type Builder struct {
	entries []hclEntry
}

// NewBuilder creates a new Builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// attribute is a key/value HCL entry.
type attribute struct {
	typeName string
	value    interface{}
}

// block is an HCL block entry.
type block struct {
	name   string
	labels []string

	builder *Builder
}

// appendTo appends this attribute into the provided *hclwrite.Body.
func (a attribute) appendTo(b *hclwrite.Body) error {
	value, err := toCtyValue(a.value)
	if err != nil {
		return fmt.Errorf("failed to append attribute, key=%s, value=%#v, due to: %w", a.typeName, a.value, err)
	}
	b.SetAttributeValue(a.typeName, value)

	return nil
}

// appendTo appends this block and all it's nested attributes and blocks into the provided
// hclwrite.Body.
func (b *block) appendTo(body *hclwrite.Body) error {
	nestedBlock := body.AppendNewBlock(b.name, b.labels)
	nestedBody := nestedBlock.Body()
	for _, entry := range b.builder.entries {
		err := entry.appendTo(nestedBody)
		if err != nil {
			return err
		}
	}

	return nil
}

// AppendAttribute appends a new attribute to the current block, returning the Builder for that block.
func (h *Builder) AppendAttribute(key string, value interface{}) *Builder {
	h.entries = append(h.entries, attribute{key, value})
	return h
}

// AppendBlock constructs a new block given the provided name and labels, appends it to current
// block and returns the Builder for the new block.
func (h *Builder) AppendBlock(name string, labels []string) *Builder {
	builder := &Builder{}
	h.entries = append(h.entries, &block{name, labels, builder})

	return builder
}

// AppendAttributes appends the provided attributes to the current block and returns the Builder for
// the current block.
func (h *Builder) AppendAttributes(attributes map[string]interface{}) *Builder {
	for k, v := range attributes {
		h.AppendAttribute(k, v)
	}

	return h
}

// BuildHCL builds an HCL document for the Builder using hclwrite.
func (h *Builder) BuildHCL() (string, error) {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	for _, entry := range h.entries {
		if err := entry.appendTo(rootBody); err != nil {
			return "", err
		}
	}

	return string(f.Bytes()), nil
}

// toCtyValue takes a Go type and attempts to convert it to a cty type.
func toCtyValue(value interface{}) (cty.Value, error) {
	ctyType, err := gocty.ImpliedType(value)
	if err != nil {
		return cty.NilVal, err
	}

	return gocty.ToCtyValue(value, ctyType)
}
