// Package cookie provides functions for securely setting and retrieving HTTP
// cookies using the SecureCookie library.
//
// See the SecureCookie README for more information on how this library works.
//
// This package provides the following functions and types:
// - Set: Set a cookie value using base64 encoding and append the HMAC signature.
// - Get: Retrieve a cookie value and verify its HMAC signature.
// - SetSigned: Set a cookie value using base64 encoding and append the HMAC signature, then sign the entire value with the secret key.
// - GetSigned: Retrieve a cookie value and verify its HMAC signature, then decode the value using the secret key.
package cookie

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/yaitoo/xun"
)

var (
	ErrValueTooLong = errors.New("cookie value too long")
	ErrInvalidValue = errors.New("invalid cookie value")
)

// Set sets a cookie value using base64 encoding
//
// If the length of the resulting cookie is longer than 4096 bytes, an ErrValueTooLong error is returned.
func Set(ctx *xun.Context, v http.Cookie) error {
	// Encode the cookie value using base64.
	v.Value = base64.URLEncoding.EncodeToString([]byte(v.Value))

	// Check the total length of the cookie contents. Return the ErrValueTooLong
	// error if it's more than 4096 bytes.
	if len(v.String()) > 4096 {
		return ErrValueTooLong
	}

	// Write the cookie as normal.
	http.SetCookie(ctx.Response, &v)

	return nil
}

// Get retrieves a cookie value using base64 decoding
//
// If the length of the resulting cookie is longer than 4096 bytes, an ErrValueTooLong error is returned.
func Get(ctx *xun.Context, name string) (string, error) {
	// Read the v as normal.
	v, err := ctx.Request.Cookie(name)
	if err != nil {
		return "", err
	}

	// Decode the base64-encoded cookie value. If the cookie didn't contain a
	// valid base64-encoded value, this operation will fail and we return an
	// ErrInvalidValue error.
	value, err := base64.URLEncoding.DecodeString(v.Value)
	if err != nil {
		return "", err
	}

	// Return the decoded cookie value.
	return string(value), nil
}

// Delete deletes a cookie by setting the MaxAge to -1 and setting the value to an empty string.
func Delete(ctx *xun.Context, v http.Cookie) {
	v.MaxAge = -1
	v.Value = ""
	http.SetCookie(ctx.Response, &v)
}

// SetSigned sets a cookie value using HMAC-SHA256 and appends the signature to
// the value.
//
// This function takes a secret key as an argument and uses it to calculate a
// HMAC signature of the cookie name and value. This signature is prepended to
// the value before setting the cookie.
//
// If the length of the resulting cookie value is longer than 4096 bytes, an
// ErrValueTooLong error is returned.
func SetSigned(ctx *xun.Context, v http.Cookie, secretKey []byte) (time.Time, error) {
	ts := time.Now()

	// Calculate a HMAC signature of the cookie name and value, using SHA256 and
	// a secret key (which we will create in a moment).
	// Prepend the cookie value with the HMAC signature.
	_, v.Value = signValue(secretKey, v.Name, v.Value, ts)

	// Call our Set() helper to base64-encode the new cookie value and write
	// the cookie.
	return ts, Set(ctx, v)
}

func signValue(secretKey []byte, name, value string, ts time.Time) (string, string) {
	v := ts.UTC().Format(time.RFC3339)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(name))
	mac.Write([]byte(v))
	mac.Write([]byte(value))
	signature := mac.Sum(nil)

	return string(signature), string(signature) + v + value
}

// GetSigned retrieves a cookie value using HMAC-SHA256 verification
//
// This function takes a secret key as an argument and uses it to calculate a
// HMAC signature of the cookie name and value. This signature is compared to the
// signature stored in the cookie. If the two signatures match, the original
// cookie value is returned. Otherwise, an ErrInvalidValue error is returned.
//
// If the length of the resulting cookie value is longer than 4096 bytes, an
// ErrValueTooLong error is returned.
func GetSigned(ctx *xun.Context, name string, secretKey []byte) (string, *time.Time, error) {
	// Get in the signed value from the cookie. This should be in the format
	// "{signature}{original value}".
	signedValue, err := Get(ctx, name)
	if err != nil {
		return "", nil, err
	}

	// A SHA256 HMAC signature has a fixed length of 32 bytes. To avoid a potential
	// 'index out of range' panic in the next step, we need to check sure that the
	// length of the signed cookie value is at least this long. We'll use the
	// sha256.Size constant here, rather than 32, just because it makes our code
	// a bit more understandable at a glance.
	if len(signedValue) < sha256.Size {
		return "", nil, ErrInvalidValue
	}

	// Split apart the signature and original cookie value.
	signature := signedValue[:sha256.Size]
	tv := signedValue[sha256.Size : sha256.Size+20]
	value := signedValue[sha256.Size+20:]

	ts, err := time.Parse(time.RFC3339, tv)
	if err != nil {
		return "", nil, ErrInvalidValue
	}

	// Recalculate the HMAC signature of the cookie name and original value.
	expectedSignature, _ := signValue(secretKey, name, value, ts)

	// Check that the recalculated signature matches the signature we received
	// in the cookie. If they match, we can be confident that the cookie name
	// and value haven't been edited by the client.
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return "", nil, ErrInvalidValue
	}

	// Return the original cookie value.
	return value, &ts, nil
}
