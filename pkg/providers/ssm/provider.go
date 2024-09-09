/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ssm

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
)

type Provider interface {
	List(context.Context, string) (map[string]string, error)
}

type ssmapi interface {
	GetParametersByPathPages(ctx context.Context, params *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error
}

type DefaultProvider struct {
	sync.Mutex
	cache  *cache.Cache
	ssmapi *ssm.Client
}

func NewDefaultProvider(ssmapi *ssm.Client, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ssmapi: ssmapi,
		cache:  cache,
	}
}

// List calls GetParametersByPath recursively with the provided input path.
// The result is a map of paths to values for those paths.
func (p *DefaultProvider) List(ctx context.Context, path string) (map[string]string, error) {
	p.Lock()
	defer p.Unlock()
	if paths, ok := p.cache.Get(path); ok {
		return paths.(map[string]string), nil
	}
	values := map[string]string{}
	paginator := ssm.NewGetParametersByPathPaginator(p.ssmapi, &ssm.GetParametersByPathInput{
		Recursive: lo.ToPtr(true),
		Path:      &path,
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting ssm parameters for path %q, %w", path, err)
		}

		for _, parameter := range output.Parameters {
			if parameter.Name == nil || parameter.Value == nil {
				continue
			}
			values[*parameter.Name] = *parameter.Value
		}
	}

	p.cache.SetDefault(path, values)
	return values, nil
}
