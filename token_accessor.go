// The MSAL Go library defines the types of cache file, and expect you to roll your own
// implementation. See below:
//   https://github.com/AzureAD/microsoft-authentication-library-for-go/blob/v1.2.2/apps/cache/cache.go
// One can base one's own cache accessor on below examples:
//   https://github.com/AzureAD/microsoft-authentication-library-for-go/blob/v1.2.2/apps/tests/integration/cache_accessor.go
//   https://github.com/AzureAD/microsoft-authentication-library-for-go/blob/v1.2.2/apps/tests/devapps/sample_cache_accessor.go
// This file is just a verbatim copy of above 'cache_accessor.go'.

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package maz

import (
	"context"
	"log"
	"os"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

type TokenCache struct {
	file string
}

func (t *TokenCache) Replace(ctx context.Context, cache cache.Unmarshaler, hints cache.ReplaceHints) error {
	data, err := os.ReadFile(t.file)
	if err != nil {
		log.Println(err)
	}
	return cache.Unmarshal(data)
}

func (t *TokenCache) Export(ctx context.Context, cache cache.Marshaler, hints cache.ExportHints) error {
	data, err := cache.Marshal()
	if err != nil {
		log.Println(err)
	}
	return os.WriteFile(t.file, data, 0600)
}

func (t *TokenCache) Print() string {
	data, err := os.ReadFile(t.file)
	if err != nil {
		return err.Error()
	}
	return string(data)
}
