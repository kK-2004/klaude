//go:build !darwin

package secret

type unavailableStore struct{}

func NewStore() Store                                { return unavailableStore{} }
func (unavailableStore) Save(string, string) error   { return ErrUnavailable }
func (unavailableStore) Load(string) (string, error) { return "", ErrUnavailable }
