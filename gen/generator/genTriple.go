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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/dubbogo/protoc-gen-go-triple/v3/util"
)

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

// processTypeWithImport processes protobuf type names, handling imported types and collecting import paths
func processTypeWithImport(typeName string, file *descriptorpb.FileDescriptorProto, imports *[]string, allFiles []*descriptorpb.FileDescriptorProto, existingAliases map[string]bool) string {
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

						// Use package name from go_package option for consistent type references
						goPackage := depFile.Options.GetGoPackage()
						if goPackage != "" {
							parts := strings.Split(goPackage, ";")
							if len(parts) >= 2 {
								// Use the package name from go_package option
								return parts[1] + "." + localTypeName
							}
						}
						// Fallback: use the last part of import path
						importPathParts := strings.Split(importPath, "/")
						packageName := importPathParts[len(importPathParts)-1]
						return packageName + "." + localTypeName
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

	// Track existing aliases to avoid conflicts
	existingAliases := make(map[string]bool)
	for _, service := range file.GetService() {
		serviceMethods := make([]Method, 0)

		for _, method := range service.GetMethod() {
			serviceMethods = append(serviceMethods, Method{
				MethodName:     method.GetName(),
				RequestType:    processTypeWithImport(method.GetInputType(), file, &tripleGo.Imports, allFiles, existingAliases),
				StreamsRequest: method.GetClientStreaming(),
				ReturnType:     processTypeWithImport(method.GetOutputType(), file, &tripleGo.Imports, allFiles, existingAliases),
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
	// Package name will be set by main.go using file.GoPackageName
	// to ensure consistency with protoc-gen-go
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

// generateAlias creates a shorter, more readable alias for import paths to avoid package name conflicts
// It tries to use meaningful parts of the import path and adds numbers when conflicts occur
func generateAlias(importPath string, existingAliases map[string]bool) string {
	// Split the import path into parts
	parts := strings.Split(importPath, "/")

	// Try different strategies to create a short, meaningful alias
	candidates := []string{}

	// Strategy 1: Use the last part (most common case)
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Remove .proto extension if present
		lastPart = strings.TrimSuffix(lastPart, ".proto")
		candidates = append(candidates, lastPart)
	}

	// Strategy 2: Use the last two parts if they exist
	if len(parts) >= 2 {
		lastTwo := parts[len(parts)-2] + "_" + parts[len(parts)-1]
		lastTwo = strings.TrimSuffix(lastTwo, ".proto")
		candidates = append(candidates, lastTwo)
	}

	// Strategy 3: Use meaningful parts (skip common prefixes like "proto", "v1", etc.)
	if len(parts) > 2 {
		meaningfulParts := []string{}
		for _, part := range parts {
			// Skip common prefixes that don't add meaning
			if part != "proto" && part != "v1" && part != "v2" && part != "v3" &&
				part != "api" && part != "pkg" && part != "internal" && part != "external" {
				meaningfulParts = append(meaningfulParts, part)
			}
		}
		if len(meaningfulParts) > 0 {
			meaningful := strings.Join(meaningfulParts, "_")
			meaningful = strings.TrimSuffix(meaningful, ".proto")
			candidates = append(candidates, meaningful)
		}
	}

	// Strategy 4: Fallback to a shortened version of the full path
	shortened := strings.ReplaceAll(strings.ReplaceAll(importPath, "/", "_"), ".", "_")
	shortened = strings.TrimSuffix(shortened, ".proto")
	// Limit length to avoid extremely long aliases
	if len(shortened) > 30 {
		shortened = shortened[:30]
	}
	candidates = append(candidates, shortened)

	// Try each candidate, adding numbers if there are conflicts
	for _, candidate := range candidates {
		// Clean up the candidate (remove any invalid characters for Go identifiers)
		candidate = strings.ReplaceAll(candidate, "-", "_")
		candidate = strings.ReplaceAll(candidate, ".", "_")

		// Ensure it starts with a letter or underscore
		if len(candidate) > 0 && !((candidate[0] >= 'a' && candidate[0] <= 'z') ||
			(candidate[0] >= 'A' && candidate[0] <= 'Z') || candidate[0] == '_') {
			candidate = "pkg_" + candidate
		}

		// Try the candidate without number first
		if !existingAliases[candidate] {
			existingAliases[candidate] = true
			return candidate
		}

		// If there's a conflict, try with numbers
		for i := 1; i <= 999; i++ {
			numberedCandidate := fmt.Sprintf("%s_%d", candidate, i)
			if !existingAliases[numberedCandidate] {
				existingAliases[numberedCandidate] = true
				return numberedCandidate
			}
		}
	}

	// If all else fails, use a hash-based approach
	hash := fmt.Sprintf("pkg_%x", importPath)
	if len(hash) > 16 {
		hash = hash[:16]
	}

	// Ensure uniqueness
	counter := 1
	finalHash := hash
	for existingAliases[finalHash] {
		finalHash = fmt.Sprintf("%s_%d", hash, counter)
		counter++
	}
	existingAliases[finalHash] = true
	return finalHash
}
