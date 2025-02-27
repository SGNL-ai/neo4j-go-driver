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

package auth_test

import (
	"context"
	"fmt"
	"github.com/SGNL-ai/neo4j-go-driver/v5/neo4j"
	"github.com/SGNL-ai/neo4j-go-driver/v5/neo4j/auth"
	"os"
	"time"
)

func ExampleExpirationBasedTokenManager() {
	myProvider := func(ctx context.Context) (neo4j.AuthToken, *time.Time, error) {
		// some way to getting a token
		token, err := getSsoToken(ctx)
		if err != nil {
			return neo4j.AuthToken{}, nil, err
		}
		// assume we know our tokens expire every 60 seconds

		expiresIn := time.Now().Add(60 * time.Second)
		// Include a little buffer so that we fetch a new token *before* the old one expires
		expiresIn = expiresIn.Add(-10 * time.Second)
		// or return nil instead of `&expiresIn` if we don't expect it to expire
		return token, &expiresIn, nil
	}

	_, _ = neo4j.NewDriverWithContext(getUrl(), auth.ExpirationBasedTokenManager(myProvider))
}

func getSsoToken(context.Context) (neo4j.AuthToken, error) {
	return neo4j.NoAuth(), nil
}

func getUrl() string {
	return fmt.Sprintf("%s://%s:%s", os.Getenv("TEST_NEO4J_SCHEME"), os.Getenv("TEST_NEO4J_HOST"), os.Getenv("TEST_NEO4J_PORT"))
}
