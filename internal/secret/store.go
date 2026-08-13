package secret

import "errors"

var ErrUnavailable = errors.New("secure credential storage is unavailable")

type Store interface {
	Save(account, value string) error
	Load(account string) (string, error)
}
