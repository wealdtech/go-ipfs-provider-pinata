package pinata

import "errors"

// Provider is an implementation of the IPFS provider for Pinata.
type Provider struct {
	apiKey    string
	apiSecret string
}

// NewProvider creates a new Pinata provider.
func NewProvider(apiKey string, apiSecret string) (*Provider, error) {

	provider := &Provider{
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}

	// Try a ping to ensure the service and credentials look good
	alive, err := provider.Ping()
	if err != nil {
		return nil, err
	}
	if !alive {
		return nil, errors.New("service unavailable")
	}
	return provider, nil
}
