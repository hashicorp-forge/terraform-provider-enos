package remoteflight

import "fmt"

// WrapErrorWith returns a custom error message
func WrapErrorWith(err error, msg ...string) error {
	for _, m := range msg {
		if m == "" {
			continue
		}
		err = fmt.Errorf("%w: %s", err, m)
	}

	return err
}
