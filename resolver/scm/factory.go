// Package scm provides SCM (Source Code Management) integrations for the resolver.
// It implements ResourceLoader and TemplateStore using go-scm clients.
package scm

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/driver/azure"
	"github.com/drone/go-scm/scm/driver/bitbucket"
	"github.com/drone/go-scm/scm/driver/gitee"
	"github.com/drone/go-scm/scm/driver/github"
	"github.com/drone/go-scm/scm/driver/gitlab"
	"github.com/drone/go-scm/scm/driver/harness"
	"github.com/drone/go-scm/scm/driver/stash"
	"github.com/drone/go-scm/scm/transport"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
)

// NewClient creates a go-scm client for the given SCM provider config.
func NewClient(cfg resolver.SCMProviderConfig) (*scm.Client, error) {
	var client *scm.Client
	var err error

	driver := resolver.Driver(strings.ToLower(string(cfg.Driver)))

	switch driver {
	case resolver.DriverGitHub:
		client, err = github.New(cfg.URL)
	case resolver.DriverGitLab:
		client, err = gitlab.New(cfg.URL)
	case resolver.DriverBitbucket:
		client, err = bitbucket.New(cfg.URL)
	case resolver.DriverStash:
		client, err = stash.New(cfg.URL)
	case resolver.DriverAzure:
		client, err = azure.New(cfg.URL, "", "")
	case resolver.DriverGitee:
		client, err = gitee.New(cfg.URL)
	case resolver.DriverHarness:
		account, org, project := parseHarnessOwner(cfg.Owner)
		client, err = harness.New(cfg.URL, account, org, project)
	default:
		return nil, fmt.Errorf("unsupported SCM driver: %q", cfg.Driver)
	}
	if err != nil {
		return nil, fmt.Errorf("create %s client for %s: %w", driver, cfg.URL, err)
	}

	token := cfg.ResolveToken()
	if token != "" {
		if driver == resolver.DriverHarness {
			client.Client = &http.Client{
				Transport: &transport.Custom{
					Before: func(r *http.Request) {
						r.Header.Set("x-api-key", token)
					},
				},
			}
		} else {
			client.Client = &http.Client{
				Transport: &transport.BearerToken{Token: token},
			}
		}
	}

	return client, nil
}

// parseHarnessOwner splits a Harness Code owner string ("account/org/project")
// into its three components. If the owner has fewer than three segments, the
// missing parts default to empty strings.
func parseHarnessOwner(owner string) (account, org, project string) {
	parts := strings.SplitN(owner, "/", 3)
	if len(parts) >= 1 {
		account = parts[0]
	}
	if len(parts) >= 2 {
		org = parts[1]
	}
	if len(parts) >= 3 {
		project = parts[2]
	}
	return
}

// NewClients creates a map of named go-scm clients from provider configs.
func NewClients(cfg resolver.SCMConfig) (map[string]*scm.Client, error) {
	clients := make(map[string]*scm.Client, len(cfg.Providers))
	for name, provCfg := range cfg.Providers {
		client, err := NewClient(provCfg)
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", name, err)
		}
		clients[name] = client
	}
	return clients, nil
}
