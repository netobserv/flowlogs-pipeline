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
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline"
)

func main() {
	// Do not remove this unnamed variable ---> This is needed for goland compiler/linker to init the
	// variables and functions for all the modules including the operational metrics variables which
	// fills up `metricsOpts` with information
	var _ *pipeline.Pipeline

	header := `
> Note: this file was automatically generated, to update execute "make docs"  
	 
# flowlogs-pipeline Operational Metrics  
	 
Each table below provides documentation for an exported flowlogs-pipeline operational metric. 

	`
	doc := operational.GetDocumentation()
	data := fmt.Sprintf("%s\n%s\n", header, doc)
	fmt.Printf("%s", data)
}
