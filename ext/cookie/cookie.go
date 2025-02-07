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
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yaitoo/xun"
)

var (
	ErrValueTooLong = errors.New("cookie value too long")
	ErrInvalidValue = errors.New("invalid cookie value")
)

func Set(ctx *xun.Context, v http.Cookie) error {
	// Encode the cookie value using base64.
	v.Value = base64.URLEncoding.EncodeToString([]byte(v.Value))

	// Check the total length of the cookie contents. Return the ErrValueTooLong
	// error if it's more than 4096 bytes.
	if len(v.String()) > 4096 {
		return ErrValueTooLong
	}

	// Write the cookie as normal.
	http.SetCookie(ctx.Writer(), &v)

	return nil
}

func Get(ctx *xun.Context, name string) (string, error) {
	// Read the v as normal.
	v, err := ctx.Request().Cookie(name)
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

func Del(ctx *xun.Context, v http.Cookie) {
	v.MaxAge = -1
	v.Value = ""
	http.SetCookie(ctx.Writer(), &v)
}

func SetSigned(ctx *xun.Context, v http.Cookie, secretKey []byte) error {
	// Calculate a HMAC signature of the cookie name and value, using SHA256 and
	// a secret key (which we will create in a moment).
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(v.Name))
	mac.Write([]byte(v.Value))
	signature := mac.Sum(nil)

	// Prepend the cookie value with the HMAC signature.
	v.Value = string(signature) + v.Value

	// Call our Set() helper to base64-encode the new cookie value and write
	// the cookie.
	return Set(ctx, v)
}

func GetSigned(ctx *xun.Context, name string, secretKey []byte) (string, error) {
	// Get in the signed value from the cookie. This should be in the format
	// "{signature}{original value}".
	signedValue, err := Get(ctx, name)
	if err != nil {
		return "", err
	}

	// A SHA256 HMAC signature has a fixed length of 32 bytes. To avoid a potential
	// 'index out of range' panic in the next step, we need to check sure that the
	// length of the signed cookie value is at least this long. We'll use the
	// sha256.Size constant here, rather than 32, just because it makes our code
	// a bit more understandable at a glance.
	if len(signedValue) < sha256.Size {
		return "", ErrInvalidValue
	}

	// Split apart the signature and original cookie value.
	signature := signedValue[:sha256.Size]
	value := signedValue[sha256.Size:]

	// Recalculate the HMAC signature of the cookie name and original value.
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(name))
	mac.Write([]byte(value))
	expectedSignature := mac.Sum(nil)

	// Check that the recalculated signature matches the signature we received
	// in the cookie. If they match, we can be confident that the cookie name
	// and value haven't been edited by the client.
	if !hmac.Equal([]byte(signature), expectedSignature) {
		return "", ErrInvalidValue
	}

	// Return the original cookie value.
	return value, nil
}

func SetEncrypted(ctx *xun.Context, cookie http.Cookie, secretKey []byte) error {
	// Create a new AES cipher block from the secret key.
	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return err
	}

	// Wrap the cipher block in Galois Counter Mode.
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Create a unique nonce containing 12 random bytes.
	nonce := make([]byte, aesGCM.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return err
	}

	// Prepare the plaintext input for encryption. Because we want to
	// authenticate the cookie name as well as the value, we make this plaintext
	// in the format "{cookie name}:{cookie value}". We use the : character as a
	// separator because it is an invalid character for cookie names and
	// therefore shouldn't appear in them.
	plaintext := fmt.Sprintf("%s:%s", cookie.Name, cookie.Value)

	// Encrypt the data using aesGCM.Seal(). By passing the nonce as the first
	// parameter, the encrypted data will be appended to the nonce — meaning
	// that the returned encryptedValue variable will be in the format
	// "{nonce}{encrypted plaintext data}".
	encryptedValue := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

	// Set the cookie value to the encryptedValue.
	cookie.Value = string(encryptedValue)

	// Write the cookie as normal.
	return Set(ctx, cookie)
}

func GetEncrypted(ctx *xun.Context, name string, secretKey []byte) (string, error) {
	// Read the encrypted value from the cookie as normal.
	encryptedValue, err := Get(ctx, name)
	if err != nil {
		return "", err
	}

	// Create a new AES cipher block from the secret key.
	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return "", err
	}

	// Wrap the cipher block in Galois Counter Mode.
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Get the nonce size.
	nonceSize := aesGCM.NonceSize()

	// To avoid a potential 'index out of range' panic in the next step, we
	// check that the length of the encrypted value is at least the nonce
	// size.
	if len(encryptedValue) < nonceSize {
		return "", ErrInvalidValue
	}

	// Split apart the nonce from the actual encrypted data.
	nonce := encryptedValue[:nonceSize]
	cipherText := encryptedValue[nonceSize:]

	// Use aesGCM.Open() to decrypt and authenticate the data. If this fails,
	// return a ErrInvalidValue error.
	plaintext, err := aesGCM.Open(nil, []byte(nonce), []byte(cipherText), nil)
	if err != nil {
		return "", ErrInvalidValue
	}

	// The plaintext value is in the format "{cookie name}:{cookie value}". We
	// use strings.Cut() to split it on the first ":" character.
	expectedName, value, ok := strings.Cut(string(plaintext), ":")
	if !ok {
		return "", ErrInvalidValue
	}

	// Check that the cookie name is the expected one and hasn't been changed.
	if expectedName != name {
		return "", ErrInvalidValue
	}

	// Return the plaintext cookie value.
	return value, nil
}
