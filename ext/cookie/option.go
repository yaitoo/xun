package cookie

import "net/http"

type Option func(v *http.Cookie)

func WithSign(secretKey []byte) Option {
	return func(v *http.Cookie) {
		v.SigningKey = secretKey
	}
}
