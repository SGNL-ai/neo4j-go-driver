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
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package neo4j

import (
	"testing"
)

func TestNoAuth(t *testing.T) {
	token := NoAuth()

	if len(token.Tokens) != 1 {
		t.Errorf("should only contain the key scheme")
	}

	if token.Tokens[keyScheme] != schemeNone {
		t.Errorf("the key scheme should be 'none' %v", token.Tokens[keyScheme])
	}
}

func TestBasicAuth(t *testing.T) {
	userName := "user"
	password := "password"
	realm := ""

	token := BasicAuth(userName, password, realm)

	if len(token.Tokens) != 3 {
		t.Errorf("should contain 3 keys when no realm data was passed")
	}

	if token.Tokens[keyScheme] != schemeBasic {
		t.Errorf("the key scheme should be 'basic' %v", token.Tokens[keyScheme])
	}

	if token.Tokens[keyPrincipal] != userName {
		t.Errorf("the key principal was not properly set %v", token.Tokens[keyPrincipal])
	}

	if token.Tokens[keyCredentials] != password {
		t.Errorf("the key credentials was not properly set %v", token.Tokens[keyCredentials])
	}
}

func TestBasicAuthWithRealm(t *testing.T) {
	userName := "user"
	password := "password"
	realm := "test"

	token := BasicAuth(userName, password, realm)

	if len(token.Tokens) != 4 {
		t.Errorf("should contain 4 keys when realm data was passed")
	}

	if token.Tokens[keyScheme] != schemeBasic {
		t.Errorf("the key scheme should be 'basic' %v", token.Tokens[keyScheme])
	}

	if token.Tokens[keyPrincipal] != userName {
		t.Errorf("the key principal was not properly set %v", token.Tokens[keyPrincipal])
	}

	if token.Tokens[keyCredentials] != password {
		t.Errorf("the key credentials was not properly set %v", token.Tokens[keyCredentials])
	}

	if token.Tokens[keyRealm] != realm {
		t.Errorf("the key realm was not properly set %v", token.Tokens[keyRealm])
	}
}

func TestKerberosAuth(t *testing.T) {
	ticket := "123456789"

	token := KerberosAuth(ticket)

	if len(token.Tokens) != 3 {
		t.Errorf("should contain 3 keys")
	}

	if token.Tokens[keyScheme] != schemeKerberos {
		t.Errorf("the key scheme should be 'kerberos' %v", token.Tokens[keyScheme])
	}

	if token.Tokens[keyPrincipal] != "" {
		t.Errorf("the key principal was not properly set %v", token.Tokens[keyPrincipal])
	}

	if token.Tokens[keyCredentials] != ticket {
		t.Errorf("the key ticket was not properly set %v", token.Tokens[keyCredentials])
	}
}

func TestCustomAuthWithNilParameters(t *testing.T) {
	scheme := "custom_scheme"
	userName := "user"
	password := "password"
	realm := "test"

	token := CustomAuth(scheme, userName, password, realm, nil)

	if len(token.Tokens) != 4 {
		t.Errorf("should contain 4 keys no parameters data was passed %v", len(token.Tokens))
	}

	if token.Tokens[keyScheme] != scheme {
		t.Errorf("the key scheme was not properly set %v", token.Tokens[keyScheme])
	}

	if token.Tokens[keyPrincipal] != userName {
		t.Errorf("the key principal was not properly set %v", token.Tokens[keyPrincipal])
	}

	if token.Tokens[keyCredentials] != password {
		t.Errorf("the key credentials was not properly set %v", token.Tokens[keyCredentials])
	}

	if token.Tokens[keyRealm] != realm {
		t.Errorf("the key realm was not properly set %v", token.Tokens[keyRealm])
	}
}

func TestCustomAuthWithEmptyParameters(t *testing.T) {
	scheme := "custom_scheme"
	userName := "user"
	password := "password"
	realm := "test"
	parameters := map[string]any{}

	token := CustomAuth(scheme, userName, password, realm, parameters)

	if len(token.Tokens) != 4 {
		t.Errorf("should contain 4 keys when parameters data was passed %v", len(token.Tokens))
	}

	if token.Tokens[keyScheme] != scheme {
		t.Errorf("the key scheme was not properly set %v", token.Tokens[keyScheme])
	}

	if token.Tokens[keyPrincipal] != userName {
		t.Errorf("the key principal was not properly set %v", token.Tokens[keyPrincipal])
	}

	if token.Tokens[keyCredentials] != password {
		t.Errorf("the key credentials was not properly set %v", token.Tokens[keyCredentials])
	}

	if token.Tokens[keyRealm] != realm {
		t.Errorf("the key realm was not properly set %v", token.Tokens[keyRealm])
	}
}

func TestCustomAuthWithParameters(t *testing.T) {
	scheme := "custom_scheme"
	userName := "user"
	password := "password"
	realm := "test"
	parameters := map[string]any{
		"user_id":     "1234",
		"user_emails": []string{"a@b.com", "b@c.com"},
	}

	token := CustomAuth(scheme, userName, password, realm, parameters)

	if len(token.Tokens) != 5 {
		t.Errorf("should contain 5 keys when parameters data was passed %v", len(token.Tokens))
	}

	if token.Tokens[keyScheme] != scheme {
		t.Errorf("the key scheme was not properly set %v", token.Tokens[keyScheme])
	}

	if token.Tokens[keyPrincipal] != userName {
		t.Errorf("the key principal was not properly set %v", token.Tokens[keyPrincipal])
	}

	if token.Tokens[keyCredentials] != password {
		t.Errorf("the key credentials was not properly set %v", token.Tokens[keyCredentials])
	}

	if token.Tokens[keyRealm] != realm {
		t.Errorf("the key realm was not properly set %v", token.Tokens[keyRealm])
	}

	if token.Tokens["parameters"] == nil {
		t.Errorf("the key parameters was not properly set %v", token.Tokens["parameters"])
	}
}
