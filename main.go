/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// protoc-gen-go-triple is a plugin for the Google protocol buffer compiler to
// generate Go code.
//
// Check readme for how to use it.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

import (
	"github.com/dubbogo/protoc-gen-go-triple/v3/gen/generator"
	"github.com/dubbogo/protoc-gen-go-triple/v3/internal/old_triple"
	"github.com/dubbogo/protoc-gen-go-triple/v3/internal/version"
)

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	usage = "See https://connect.build/docs/go/getting-started to learn how to use this plugin.\n\nFlags:\n  -h, --help\tPrint this help and exit.\n      --version\tPrint the version and exit."
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Fprintln(os.Stdout, version.Version)
		os.Exit(0)
	}
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Fprintln(os.Stdout, usage)
		os.Exit(0)
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	var flags flag.FlagSet
	useOld := flags.Bool("useOldVersion", false, "print the version and exit")
	old_triple.RequireUnimplemented = flags.Bool("require_unimplemented_servers", true, "set to false to match legacy behavior")

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(
		func(plugin *protogen.Plugin) error {
			plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
			if *useOld {
				return genOldTriple(plugin)
			}
			return genTriple(plugin)
		},
	)
}

func genTriple(plugin *protogen.Plugin) error {
	var errors []error

	allFiles := make([]*descriptorpb.FileDescriptorProto, 0, len(plugin.Files))
	for _, file := range plugin.Files {
		allFiles = append(allFiles, file.Proto)
	}

	for _, file := range plugin.Files {
		// Skip files that are not marked for generation
		if !file.Generate {
			continue
		}

		// Skip files that don't contain any service definitions
		if len(file.Proto.GetService()) == 0 {
			continue
		}

		tripleGo, err := generator.ProcessProtoFile(file.Proto, allFiles)
		if err != nil {
			errors = append(errors, fmt.Errorf("processing %s: %w", file.Desc.Path(), err))
			continue
		}
		// Ensure the generated file uses the exact Go package name computed by protoc-gen-go.
		tripleGo.Package = string(file.GoPackageName)
		filename := file.GeneratedFilenamePrefix + ".triple.go"
		// Use the same import path as the pb.go file to ensure they're in the same package
		// Extract the package name from the go_package option
		goPackage := file.Proto.Options.GetGoPackage()
		var importPath protogen.GoImportPath
		if goPackage != "" {
			parts := strings.Split(goPackage, ";")
			importPath = protogen.GoImportPath(parts[0])
		} else {
			importPath = file.GoImportPath
		}
		g := plugin.NewGeneratedFile(filename, importPath)
		err = generator.GenTripleFile(g, tripleGo)
		if err != nil {
			errors = append(errors, fmt.Errorf("generating %s: %w", filename, err))
		}
	}
	if len(errors) > 0 {
		var errorMessages []string
		for _, err := range errors {
			errorMessages = append(errorMessages, err.Error())
		}
		return fmt.Errorf("multiple errors occurred:\n%s", strings.Join(errorMessages, "\n"))
	}
	return nil
}

func genOldTriple(plugin *protogen.Plugin) error {
	for _, file := range plugin.Files {
		if file.Generate {
			old_triple.GenerateFile(plugin, file)
		}
	}
	return nil
}
