// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2015 The Go Authors.  All rights reserved.
// https://github.com/golang/protobuf
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Package grpc outputs gRPC service descriptions in Go code.
// It runs as a plugin for the Go protocol buffer compiler plugin.
// It is linked in to protoc-gen-go.
package grpc

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/ekle/protoc-gen-goweb/generator"
	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

// Paths for packages used by code generated in this file,
// relative to the import_prefix of the generator.Generator.
const (
	contextPkgPath = "golang.org/x/net/context"
	grpcPkgPath    = "google.golang.org/grpc"
)

func init() {
	generator.RegisterPlugin(new(grpc))
}

// grpc is an implementation of the Go protocol buffer compiler's
// plugin architecture.  It generates bindings for gRPC support.
type grpc struct {
	gen *generator.Generator
}

// Name returns the name of this plugin, "grpc".
func (g *grpc) Name() string {
	return "grpc"
}

// The names for packages imported in the generated code.
// They may vary from the final path component of the import path
// if the name is used by other packages.
var (
	contextPkg string
	grpcPkg    string
)

// Init initializes the plugin.
func (g *grpc) Init(gen *generator.Generator) {
	g.gen = gen
	contextPkg = generator.RegisterUniquePackageName("context", nil)
	grpcPkg = generator.RegisterUniquePackageName("grpc", nil)
}

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (g *grpc) objectNamed(name string) generator.Object {
	g.gen.RecordTypeUse(name)
	return g.gen.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (g *grpc) typeName(str string) string {
	return g.gen.TypeName(g.objectNamed(str))
}

// P forwards to g.gen.P.
func (g *grpc) P(args ...interface{}) { g.gen.P(args...) }

// Generate generates code for the services in the given file.
func (g *grpc) Generate(file *generator.FileDescriptor) {
	for i, service := range file.FileDescriptorProto.Service {
		g.generateService(file, service, i)
	}
}

// GenerateImports generates the import declaration for this file.
func (g *grpc) GenerateImports(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}
	g.P("import (")
	g.P(contextPkg, " ", strconv.Quote(path.Join(g.gen.ImportPrefix, contextPkgPath)))
	//g.P(grpcPkg, " ", strconv.Quote(path.Join(g.gen.ImportPrefix, grpcPkgPath)))
	g.P("\"github.com/zenazn/goji/web\"")
	g.P("\"net/http\"")
	g.P("\"io/ioutil\"")
	g.P("\"log\"")
	//g.P("\"strings\"")
	g.P("\"encoding/json\"")
	g.P(")")
	g.P()
	g.P("// Reference imports to suppress errors if they are not otherwise used.")
	g.P("var _ ", contextPkg, ".Context")
	//g.P("var _ ", grpcPkg, ".ClientConn")
	g.P("var _ web.C")
	g.P()
}

// reservedClientName records whether a client name is reserved on the client side.
var reservedClientName = map[string]bool{
// TODO: do we need any in gRPC?
}

func unexport(s string) string { return strings.ToLower(s[:1]) + s[1:] }

// generateService generates all the code for the named service.
func (g *grpc) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	//path := fmt.Sprintf("6,%d", index) // 6 means service.

	origServName := service.GetName()
	//fullServName := file.GetPackage() + "." + origServName
	servName := generator.CamelCase(origServName)
	g.P("// Server API for ", servName, " service")
	g.P()

	// Server interface.
	serverType := servName + "Server"
	g.P()

	g.P("func New", servName, "Mux(h ", serverType, ", prefix string) *web.Mux {")
	g.P("	t := _", serverType, "{}")
	g.P("	t.handler = h")
	g.P("	router := web.New()")
	for _, method := range service.Method {
		path := strings.ToLower(servName) + "/" + method.GetName()
		methName := generator.CamelCase(method.GetName())
		// there should be a better way to get the options
		m := method.GetOptions().String()
		if m != "" {
			parts := strings.Split(m, "\"")
			if len(parts) == 3 {
				if parts[0] == "10000:" {
					path = parts[1]
				}
			}
		}
		g.P("router.Handle(prefix+\"", strings.ToLower(path), "\", t.", methName, ")")
	}
	g.P("	return router")
	g.P("}")
	g.P()

	g.P("type _", serverType, " struct {")
	g.P("	handler ", serverType)
	g.P("}")
	g.P()

	// Server handler implementations.
	for _, method := range service.Method {
		g.generateServerMethod(servName, method)
	}

}

// generateServerSignature returns the server-side signature for a method.
func (g *grpc) generateServerSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}

	var reqArgs []string
	ret := "error"
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		reqArgs = append(reqArgs, contextPkg+".Context")
		ret = "(*" + g.typeName(method.GetOutputType()) + ", error)"
	}
	if !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "*"+g.typeName(method.GetInputType()))
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		reqArgs = append(reqArgs, servName+"_"+generator.CamelCase(origMethName)+"Server")
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func (g *grpc) generateServerMethod(servName string, method *pb.MethodDescriptorProto) string {
	methName := generator.CamelCase(method.GetName())
	hname := fmt.Sprintf("_%s_%s_Handler", servName, methName)
	inType := g.typeName(method.GetInputType())
	outType := g.typeName(method.GetOutputType())

	g.P()
	serverType := servName + "Server"

	g.P("// _", serverType, ".", methName, "(", inType, ") ", outType)
	g.P("func (impl* _", serverType, " )", methName, "(c web.C, w http.ResponseWriter, r *http.Request) {")

	if method.GetServerStreaming() || method.GetClientStreaming() {
		g.P("		w.WriteHeader(501)")
		g.P("		w.Write([]byte(`Streaming functions over http are not supported`))")
		g.P("		return")
	} else {
		g.P("	in := ", inType, "{}")
		g.P("	content, err := ioutil.ReadAll(r.Body)")
		g.P("	defer r.Body.Close()")
		g.P("	if err != nil {")
		g.P("		w.WriteHeader(408)")
		g.P("		w.Write([]byte(err.Error()))")
		g.P("		log.Println(err.Error())")
		g.P("		return")
		g.P("	}")
		g.P("	err = json.Unmarshal(content, &in)")
		g.P("	if err != nil {")
		g.P("		w.WriteHeader(400)")
		g.P("		w.Write([]byte(err.Error()))")
		g.P("		log.Println(err.Error())")
		g.P("		return")
		g.P("	}")
		g.P("	res,err := impl.handler.", methName, "(context.TODO(),&in)")
		g.P("	if err != nil {")
		g.P("		w.WriteHeader(500)")
		g.P("		w.Write([]byte(err.Error()))")
		g.P("		log.Println(err.Error())")
		g.P("		return")
		g.P("	}")
		g.P("	json.NewEncoder(w).Encode(res)")
	}
	g.P("}")
	g.P()

	return hname
}
