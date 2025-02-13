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

	ok = verifyToken(&http.Cookie{
		Value: "BVaCD9NBke1Oq2rtkU_bjRcWOEGrNmTGYd9ikcQ_5HM=.0",
	}, nil, &Options{})
	require.Equal(t, false, ok)
}
