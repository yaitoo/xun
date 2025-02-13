package csrf

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToken(t *testing.T) {

	ok := verifyToken(&http.Cookie{
		Value: "",
	}, nil, &Options{})

	require.Equal(t, false, ok)

	ok = verifyToken(&http.Cookie{
		Value: ".",
	}, nil, &Options{})
	require.Equal(t, false, ok)

	ok = verifyToken(&http.Cookie{
		Value: "0.",
	}, nil, &Options{})
	require.Equal(t, false, ok)
}
