// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package artifactory

import (
	"encoding/json"
	"maps"
	"text/template"
)

// AQLQueryTemplate is our default query template. The syntax for AQL, combined
// a go template might make this hard to read, but most of what we're doing
// is dealing with trailing commas. The rendered query will look something like
// the following:
//
//	items.find({
//	  "repo": { "$match": "hashicorp-packagespec-buildcache-local*" },
//	  "path": { "$match": "cache-v1/vault-enterprise/*" },
//	  "name": { "$match": "*.zip" },
//	  "@EDITION": { "$match": "ent" },
//	  "@GOARCH": { "$match": "amd64" },
//	  "@GOOS": { "$match": "linux" },
//	  "@artifactType": { "$match": "package" },
//	  "@productRevision": { "$match": "f45845666b4e552bfc8ca775834a3ef6fc097fe0" },
//	  "@productVersion": { "$match": "1.7.0" }
//	}) .include("*", "property.*") .sort({"$desc": ["modified"]}) .limit(1)
var AQLQueryTemplate = template.Must(template.New("aql_query").Parse(`items.find({
  {{ if .Repo -}}
  "repo": { "$match": "{{ .Repo }}" }{{ if or .Path .Name .Properties -}},{{ end }}
  {{ end -}}
  {{ if .Path -}}
  "path": { "$match": "{{ .Path }}" }{{ if or .Name .Properties -}},{{ end }}
  {{ end -}}
  {{ if .Name -}}
  "name": { "$match": "{{ .Name }}" }{{ if .Properties -}},{{ end }}
  {{ end -}}
  {{ if .Properties -}}
  {{ $first := true -}}
  {{ range $k, $v := .Properties -}}
  {{ if $first -}}
	{{ $first = false -}}
  {{ else -}},{{ end -}}
  "@{{ $k }}": { "$match": "{{ $v }}" }
  {{ end -}}
  {{ end -}}
}) .include("*", "property.*") .sort({"$desc": ["modified"]}) {{ if .Limit -}}.limit({{ .Limit -}}){{ end -}}
`))

type SearchAQLOpt func(*SearchAQLRequest) *SearchAQLRequest

type SearchAQLResponse struct {
	Results []struct {
		Repo       string      `json:"repo"`
		Path       string      `json:"path"`
		Name       string      `json:"name"`
		Type       string      `json:"type"`
		Size       json.Number `json:"size"`
		SHA256     string      `json:"sha256"`
		Properties []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"properties"`
	} `json:"results"`
}

type SearchAQLRequest struct {
	Repo          string
	Path          string
	Name          string
	Properties    map[string]string
	QueryTemplate *template.Template
	Limit         string
}

func NewSearchAQLRequest(opts ...SearchAQLOpt) *SearchAQLRequest {
	req := &SearchAQLRequest{
		QueryTemplate: AQLQueryTemplate,
		Properties:    map[string]string{},
	}

	for _, opt := range opts {
		req = opt(req)
	}

	return req
}

func WithRepo(repo string) SearchAQLOpt {
	return func(req *SearchAQLRequest) *SearchAQLRequest {
		req.Repo = repo
		return req
	}
}

func WithPath(path string) SearchAQLOpt {
	return func(req *SearchAQLRequest) *SearchAQLRequest {
		req.Path = path
		return req
	}
}

func WithName(name string) SearchAQLOpt {
	return func(req *SearchAQLRequest) *SearchAQLRequest {
		req.Name = name
		return req
	}
}

func WithLimit(limit string) SearchAQLOpt {
	return func(req *SearchAQLRequest) *SearchAQLRequest {
		req.Limit = limit
		return req
	}
}

func WithQueryTemplate(temp *template.Template) SearchAQLOpt {
	return func(req *SearchAQLRequest) *SearchAQLRequest {
		req.QueryTemplate = temp
		return req
	}
}

func WithProperty(key, value string) SearchAQLOpt {
	return func(req *SearchAQLRequest) *SearchAQLRequest {
		req.Properties[key] = value
		return req
	}
}

func WithProperties(props map[string]string) SearchAQLOpt {
	return func(req *SearchAQLRequest) *SearchAQLRequest {
		maps.Copy(req.Properties, props)

		return req
	}
}
