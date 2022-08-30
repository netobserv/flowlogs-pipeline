/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
)

func iterate(output io.Writer, data interface{}, indent int) {
	newIndent := indent + 1
	dataType := reflect.ValueOf(data).Kind()
	// DEBUG code: dataTypeName := reflect.ValueOf(data).Type().String()
	d := reflect.ValueOf(data)
	if dataType == reflect.Slice || dataType == reflect.Map {
		// DEBUG code: fmt.Fprintf(output, "%s %s <-- %s \n",strings.Repeat(" ",4*indent),dataTypeName,dataType )
		zeroElement := reflect.Zero(reflect.ValueOf(data).Type().Elem()).Interface()
		iterate(output, zeroElement, newIndent)
		return
	} else if dataType == reflect.Struct {
		// DEBUG code: fmt.Fprintf(output,"%s %s <-- %s \n",strings.Repeat(" ",4*indent),dataTypeName,dataType )
		for i := 0; i < d.NumField(); i++ {
			val := reflect.Indirect(reflect.ValueOf(data))
			fieldName := val.Type().Field(i).Tag.Get(api.TagYaml)
			fieldName = strings.ReplaceAll(fieldName, ",omitempty", "")

			fieldDocTag := val.Type().Field(i).Tag.Get(api.TagDoc)
			fieldEnumTag := val.Type().Field(i).Tag.Get(api.TagEnum)

			if fieldEnumTag != "" {
				enumType := api.GetEnumReflectionTypeByFieldName(fieldEnumTag)
				zeroElement := reflect.Zero(enumType).Interface()
				fmt.Fprintf(output, "%s %s: (enum) %s\n", strings.Repeat(" ", 4*newIndent), fieldName, fieldDocTag)
				iterate(output, zeroElement, newIndent)
				continue
			}
			if fieldDocTag != "" {
				if fieldDocTag[0:1] == "#" {
					fmt.Fprintf(output, "\n%s\n", fieldDocTag)
					fmt.Fprintf(output, "<pre>")
					fmt.Fprintf(output, "\n%s %s:\n", strings.Repeat(" ", 4*indent), fieldName)
					iterate(output, d.Field(i).Interface(), newIndent)
					fmt.Fprintf(output, "</pre>")
				} else {
					fmt.Fprintf(output, "%s %s: %s\n", strings.Repeat(" ", 4*newIndent), fieldName, fieldDocTag)
					iterate(output, d.Field(i).Interface(), newIndent)
				}
			}
		}
		return
	} else if dataType == reflect.Ptr {
		// DEBUG code: fmt.Fprintf(output, "%s %s <-- %s \n", strings.Repeat(" ", 4*indent), dataTypeName, dataType)
		elemType := reflect.TypeOf(data).Elem()
		zeroElement := reflect.Zero(elemType).Interface()
		// Since we only "converted" Ptr to Struct and the actual output is done in the next iteration, we call
		// iterate() with the same `indent` as the current level
		iterate(output, zeroElement, indent)
	}
}

func main() {
	output := new(bytes.Buffer)
	iterate(output, api.API{}, 0)
	fmt.Print(output)
}
