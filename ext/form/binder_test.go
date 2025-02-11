package form

import (
	"bytes"
	"crypto/tls"

	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	trans "github.com/go-playground/validator/v10/translations/zh"
	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestBinder(t *testing.T) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // skipcq: GSC-G402, GO-S1020
	client := http.Client{
		Transport: tr,
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	app := xun.New(xun.WithMux(mux))

	type Login struct {
		Email  string `form:"email" json:"email" validate:"required,email"`
		Passwd string `json:"passwd" validate:"required"`
	}

	AddValidator(ut.New(zh.New()).GetFallback(), trans.RegisterDefaultTranslations)

	app.Get("/login", func(c *xun.Context) error {
		it, err := BindQuery[Login](c.Request)
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return xun.ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "xun@yaitoo.cn" && it.Data.Passwd == "123" {
			return c.View(it)
		}
		c.WriteStatus(http.StatusBadRequest)
		// should `return ErrCancelled`` in production
		// `return c.View(it)` is here for test to check collected data
		return c.View(it)
	})

	app.Post("/login", func(c *xun.Context) error {
		it, err := BindForm[Login](c.Request)
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return xun.ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "xun@yaitoo.cn" && it.Data.Passwd == "123" {
			return c.View(it)
		}
		c.WriteStatus(http.StatusBadRequest)
		// should `return ErrCancelled`` in production
		// `return c.View(it)` is here for test to check collected data
		return c.View(it)
	})

	app.Put("/login", func(c *xun.Context) error {
		it, err := BindJson[Login](c.Request)
		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return xun.ErrCancelled
		}

		if it.Validate(c.AcceptLanguage()...) && it.Data.Email == "xun@yaitoo.cn" && it.Data.Passwd == "123" {
			return c.View(it)
		}
		c.WriteStatus(http.StatusBadRequest)
		// should `return ErrCancelled`` in production
		// `return c.View(it)` is here for test to check collected data
		return c.View(it)
	})

	app.Start()
	defer app.Close()

	var tests = []struct {
		Name       string
		NewRequest func(it Login) *http.Request
	}{
		{
			"BindQuery",
			func(it Login) *http.Request {
				req, _ := http.NewRequest("GET", srv.URL+"/login?email="+url.QueryEscape(it.Email)+"&Passwd="+url.QueryEscape(it.Passwd), nil)
				return req
			},
		},

		{
			"BindForm",
			func(it Login) *http.Request {
				form := url.Values{}
				form.Add("email", it.Email)
				form.Add("Passwd", it.Passwd)

				req, _ := http.NewRequest("POST", srv.URL+"/login", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req
			},
		},

		{
			"BindJson",
			func(it Login) *http.Request {
				buf, _ := json.Marshal(it)

				req, _ := http.NewRequest("PUT", srv.URL+"/login", bytes.NewReader(buf))
				// req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var result TEntity[Login]

			req := test.NewRequest(Login{Email: "xun@yaitoo.cn", Passwd: "123"})
			resp, err := client.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, resp.StatusCode)

			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			resp.Body.Close()
			require.Equal(t, "xun@yaitoo.cn", result.Data.Email)
			require.Equal(t, "123", result.Data.Passwd)
			require.Len(t, result.Errors, 0)

			req = test.NewRequest(Login{Email: "xun@yaitoo.cn", Passwd: "abc"})
			resp, err = client.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)

			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			resp.Body.Close()
			require.Equal(t, "xun@yaitoo.cn", result.Data.Email)
			require.Equal(t, "abc", result.Data.Passwd)
			require.Len(t, result.Errors, 0)

			req = test.NewRequest(Login{Email: "xun"})
			req.Header.Set("accept-language", "en-US,en;q=0.9,zh;q=0.8,zh-CN;q=0.7,zh-TW;q=0.6")
			// req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8,en-US;q=0.7,zh-TW;q=0.6")
			resp, err = client.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)

			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			resp.Body.Close()
			require.Len(t, result.Errors, 2)
			require.Equal(t, "Email must be a valid email address", result.Errors["Email"])
			require.Equal(t, "Passwd is a required field", result.Errors["Passwd"])

			req = test.NewRequest(Login{Email: "xun"})
			// req.Header.Set("accept-language", "en-US,en;q=0.9,zh;q=0.8,zh-CN;q=0.7,zh-TW;q=0.6")
			req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8,en-US;q=0.7,zh-TW;q=0.6")
			resp, err = client.Do(req)
			require.NoError(t, err)

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)

			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			resp.Body.Close()
			require.Len(t, result.Errors, 2)
			require.Equal(t, "Email必须是一个有效的邮箱", result.Errors["Email"])
			require.Equal(t, "Passwd为必填字段", result.Errors["Passwd"])
		})
	}

}
