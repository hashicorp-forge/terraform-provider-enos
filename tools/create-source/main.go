package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"
)

// Temp is the struct we'll pass into our resource template
type Temp struct {
	Struct    string // resourceName
	StructCap string // ResourceName
	State     string // resourceNameStateVersion
	StateCap  string // ResourceNameState
	Name      string // enos_resource_name
	BaseName  string // resource_name
}

var name = flag.String("name", "", "the name of the resource in camel_case, eg: remote_exec")
var pluginType = flag.String("type", "resource", "the type of plugin source to make, 'resource' or 'datasource'")

var snakeReg = regexp.MustCompile("(^[A-Za-z])|_([A-Za-z])")

func snakeToCamel(str string) string {
	return snakeReg.ReplaceAllStringFunc(str, func(s string) string {
		return strings.ToUpper(strings.Replace(s, "_", "", -1))
	})
}

func newTemp(name string) Temp {
	tmp := Temp{BaseName: name}
	tmp.Name = fmt.Sprintf("enos_%s", tmp.BaseName)

	camel := snakeToCamel(tmp.BaseName)
	f, n := utf8.DecodeRuneInString(camel)

	tmp.Struct = string(unicode.ToLower(f)) + camel[n:]
	tmp.StructCap = string(unicode.ToUpper(f)) + camel[n:]
	tmp.State = fmt.Sprintf("%sStateV1", tmp.Struct)
	tmp.StateCap = fmt.Sprintf("%sStateV1", tmp.StructCap)

	return tmp
}

func main() {
	exit := func(err error) {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	flag.Parse()
	if *name == "" {
		exit(fmt.Errorf("you must provide a name"))
	}

	if *pluginType != "resource" && *pluginType != "datasource" {
		exit(fmt.Errorf("you must provide a valid source type: 'resource' or 'datasource'"))
	}

	temp := newTemp(*name)

	var err error
	tmplPath := fmt.Sprintf("./tools/create-source/%s.go.tmpl", *pluginType)
	destPath := fmt.Sprintf("./internal/plugin/%s_%s.go", *pluginType, temp.BaseName)
	tmplPath, err = filepath.Abs(tmplPath)
	if err != nil {
		exit(err)
	}
	destPath, err = filepath.Abs(destPath)
	if err != nil {
		exit(err)
	}

	tmplF, err := os.Open(tmplPath)
	defer tmplF.Close() // nolint: staticcheck
	if err != nil {
		exit(err)
	}

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(tmplF)
	if err != nil {
		exit(err)
	}

	tmpl, err := template.New(temp.Name).Parse(buf.String())
	if err != nil {
		exit(err)
	}
	buf.Reset()

	err = tmpl.Execute(&buf, temp)
	if err != nil {
		exit(err)
	}

	fmt.Printf("writing to %s\n", destPath)
	dest, err := os.Create(destPath)
	defer dest.Close() // nolint: staticcheck
	if err != nil {
		exit(err)
	}

	_, err = buf.WriteTo(dest)
	if err != nil {
		exit(err)
	}

	fmt.Printf("success! remember to register your %s in the server\n", *pluginType)
}
