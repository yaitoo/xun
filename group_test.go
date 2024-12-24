package htmx

import (
	"net/http"
	"os"
	"testing"
)

func TestGroup(t *testing.T) {
	app := New(WithMux(http.NewServeMux()),
		WithFsys(os.DirFS(".")))

	app.Get("/hello", func(c *Context) error {
		//c.View(map[string]string{"name": "World"})

		return nil
	})

	admin := app.Group("/admin")

	admin.Use(func(next HandleFunc) HandleFunc {
		return func(c *Context) error {
			if c.routing.Options.String(NavigationAccess) != "admin:*" {
				c.WriteStatus(http.StatusForbidden)
				return ErrCancelled
			}

			return next(c)
		}

	})

	admin.Get("/", func(c *Context) error {
		return c.View(nil)

	}, WithNavigation("admin", "fa fa-home", "admin:*"))

	admin.Post("/form", func(c *Context) error {
		data, err := BindJSON[TestData](c.Request())

		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if !data.Validate(c.AcceptLanguage()...) {
			c.WriteStatus(http.StatusBadRequest)
			return c.View(data)
		}

		return c.View(data)
	})

	admin.Get("/search", func(c *Context) error {
		data, err := BindQuery[TestData](c.Request())

		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if !data.Validate(c.AcceptLanguage()...) {
			c.WriteStatus(http.StatusBadRequest)
			return c.View(data)
		}

		return c.View(data)
	})

	admin.Post("/form", func(c *Context) error {
		data, err := BindForm[TestData](c.Request())

		if err != nil {
			c.WriteStatus(http.StatusBadRequest)
			return ErrCancelled
		}

		if !data.Validate(c.AcceptLanguage()...) {
			c.WriteStatus(http.StatusBadRequest)
			return c.View(data)
		}

		return c.View(data)
	})
}

type TestData struct {
}
