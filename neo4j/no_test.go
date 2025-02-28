/*
 * Copyright (c) "Neo4j"
 * Neo4j Sweden AB [https://neo4j.com]
 *
 * This file is part of Neo4j.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package neo4j

import (
	"reflect"
	"testing"
)

func assertErrorEq(t *testing.T, err1, err2 error) {
	t.Helper()
	if !reflect.DeepEqual(err1, err2) {
		t.Errorf("Wrong type of error, '%s' != '%s'", err1, err2)
	}
}

func assertUsageError(t *testing.T, err error) {
	t.Helper()
	if !IsUsageError(err) {
		t.Errorf("Expected %T but was %T:%s", &UsageError{}, err, err)
	}
}
