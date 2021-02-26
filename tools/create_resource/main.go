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
	temp := newTemp(*name)

	templPath, err := filepath.Abs("./tools/create_resource/templ.text")
	if err != nil {
		exit(err)
	}

	templF, err := os.Open(templPath)
	defer templF.Close() // nolint: staticcheck
	if err != nil {
		exit(err)
	}

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(templF)
	if err != nil {
		exit(err)
	}

	templ, err := template.New(temp.Name).Parse(buf.String())
	if err != nil {
		exit(err)
	}
	buf.Reset()

	err = templ.Execute(&buf, temp)
	if err != nil {
		exit(err)
	}

	destPath, err := filepath.Abs(fmt.Sprintf("./internal/plugin/resource_%s.go", temp.BaseName))
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

	fmt.Println("success! remember to register your resource in the server")
}
