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

package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

import (
	"github.com/dubbogo/protoc-gen-go-triple/v3/util"
)

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
)

// generateAlias creates an alias for import paths to avoid package name conflicts
// by replacing "/" and "." with "_"
func generateAlias(importPath string) string {
	return strings.ReplaceAll(strings.ReplaceAll(importPath, "/", "_"), ".", "_")
}

// buildPackageLookupMap creates a map for efficient package name to file lookup
func buildPackageLookupMap(allFiles []*descriptorpb.FileDescriptorProto) map[string]*descriptorpb.FileDescriptorProto {
	lookupMap := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, file := range allFiles {
		lookupMap[file.GetPackage()] = file
	}
	return lookupMap
}

// buildDependencyLookupMap creates a map for efficient dependency path to file lookup
func buildDependencyLookupMap(allFiles []*descriptorpb.FileDescriptorProto) map[string]*descriptorpb.FileDescriptorProto {
	lookupMap := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, file := range allFiles {
		lookupMap[file.GetName()] = file
	}
	return lookupMap
}

// processTypeWithImport processes protobuf type names, handling imported types by creating aliases and collecting import paths
func processTypeWithImport(typeName string, file *descriptorpb.FileDescriptorProto, imports *[]string, allFiles []*descriptorpb.FileDescriptorProto) string {
	typeName = strings.TrimPrefix(typeName, ".") // Remove the leading dot

	parts := strings.Split(typeName, ".")
	if len(parts) > 1 {
		// Build lookup maps for efficient searching
		packageLookup := buildPackageLookupMap(allFiles)
		dependencyLookup := buildDependencyLookupMap(allFiles)

		// Try to find the longest matching package name
		for i := len(parts) - 1; i > 0; i-- {
			importedPackage := strings.Join(parts[:i], ".")
			localTypeName := parts[i]

			// Check if the type is from the same package
			if importedPackage == file.GetPackage() {
				// Same package, return just the type name
				return util.ToUpper(localTypeName)
			}

			// Check if the package exists in our lookup map
			if depFile, exists := packageLookup[importedPackage]; exists {
				// Verify this package is actually a dependency of the current file
				for _, dep := range file.GetDependency() {
					if depFile.GetName() == dep {
						importPath := findImportPathFromDependency(dep, dependencyLookup)
						found := false
						for _, imp := range *imports {
							if imp == importPath {
								found = true
								break
							}
						}
						if !found {
							*imports = append(*imports, importPath)
						}

						// Generate alias to avoid package name conflicts
						alias := generateAlias(importPath)
						return alias + "." + localTypeName
					}
				}
			}
		}
	}
	// For local types, use the original logic
	return util.ToUpper(parts[len(parts)-1])
}

// findImportPathFromDependency extracts the Go import path from a dependency file's go_package option
func findImportPathFromDependency(depPath string, dependencyLookup map[string]*descriptorpb.FileDescriptorProto) string {
	if depFile, exists := dependencyLookup[depPath]; exists {
		goPackage := depFile.Options.GetGoPackage()
		if goPackage != "" {
			parts := strings.Split(goPackage, ";")
			if len(parts) >= 1 {
				return parts[0]
			}
		}
		return strings.TrimSuffix(depPath, ".proto")
	}
	return strings.TrimSuffix(depPath, ".proto")
}

func (g *Generator) parseTripleToString(t TripleGo) (string, error) {
	var builder strings.Builder

	for _, tpl := range Tpls {
		err := tpl.Execute(&builder, t)
		if err != nil {
			return "", err
		}
	}

	return builder.String(), nil
}

func (g *Generator) generateToFile(filePath string, data []byte) error {
	err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.WriteFile(filePath, data, 0666)
	if err != nil {
		return err
	}
	return util.GoFmtFile(filePath)
}

func ProcessProtoFile(file *descriptorpb.FileDescriptorProto, allFiles []*descriptorpb.FileDescriptorProto) (TripleGo, error) {
	tripleGo := TripleGo{
		Source:       file.GetName(),
		ProtoPackage: file.GetPackage(),
		Services:     make([]Service, 0),
		Imports:      make([]string, 0), // Added to collect imports
	}
	for _, service := range file.GetService() {
		serviceMethods := make([]Method, 0)

		for _, method := range service.GetMethod() {
			serviceMethods = append(serviceMethods, Method{
				MethodName:     method.GetName(),
				RequestType:    processTypeWithImport(method.GetInputType(), file, &tripleGo.Imports, allFiles),
				StreamsRequest: method.GetClientStreaming(),
				ReturnType:     processTypeWithImport(method.GetOutputType(), file, &tripleGo.Imports, allFiles),
				StreamsReturn:  method.GetServerStreaming(),
			})
			if method.GetClientStreaming() || method.GetServerStreaming() {
				tripleGo.IsStream = true
			}
		}

		tripleGo.Services = append(tripleGo.Services, Service{
			ServiceName: service.GetName(),
			Methods:     serviceMethods,
		})
	}
	var goPkg string
	pkgs := strings.Split(file.Options.GetGoPackage(), ";")
	switch len(pkgs) {
	case 2:
		tripleGo.Package = pkgs[1]
		goPkg = pkgs[0]
	case 1:
		tripleGo.Package = file.GetPackage()
		goPkg = file.GetPackage()
	default:
		return tripleGo, errors.New("need to set the package name in go_package")
	}

	goPkg = strings.ReplaceAll(goPkg, "/", "_")
	_, fileName := filepath.Split(file.GetName())
	tripleGo.FileName = strings.Split(fileName, ".")[0]
	return tripleGo, nil
}

func GenTripleFile(genFile *protogen.GeneratedFile, triple TripleGo) error {
	g := &Generator{}
	data, err := g.parseTripleToString(triple)
	if err != nil {
		return err
	}

	_, err = genFile.Write([]byte(data))
	return err
}

type TripleGo struct {
	Source       string
	Package      string
	FileName     string
	ProtoPackage string
	Services     []Service
	IsStream     bool
	Imports      []string
}

type Service struct {
	ServiceName string
	Methods     []Method
}

type Method struct {
	MethodName     string
	RequestType    string
	StreamsRequest bool
	ReturnType     string
	StreamsReturn  bool
}
