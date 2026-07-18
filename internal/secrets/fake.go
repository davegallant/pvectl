package secrets

// FakeStore is an in-memory Store for tests, avoiding the real OS keychain.
type FakeStore struct {
	data map[string]string
}

func NewFakeStore() *FakeStore {
	return &FakeStore{data: make(map[string]string)}
}

func (f *FakeStore) Get(host string) (string, error) {
	secret, ok := f.data[host]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

func (f *FakeStore) Set(host, secret string) error {
	f.data[host] = secret
	return nil
}
