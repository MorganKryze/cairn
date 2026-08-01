package status

import (
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
)

// config cannot import this package: it knows about nothing else, which is the
// rule the dependency graph is built on. So it keeps its own list of the
// providers status.provider accepts, and this is the test that stops the two
// from drifting.
//
// The drift it catches has a shape: a provider registered here but not there
// loads as an error nobody can fix, and a provider named there but not here
// loads clean and then fails every poll with a message the operator cannot act
// on, since the key they wrote is exactly the one the docs told them to.
func TestConfigAcceptsExactlyTheProvidersThatExist(t *testing.T) {
	if got, want := providerNames(), strings.Join(config.StatusProviders(), ", "); got != want {
		t.Errorf("the poller reads %s, the config accepts %s; whichever was added has to be added to both", got, want)
	}
}
