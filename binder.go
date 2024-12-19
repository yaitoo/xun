package htmx

import (
	// "encoding/json"

	"io"
	"net/http"

	"github.com/go-playground/form/v4"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	jsoniter "github.com/json-iterator/go"
)

var (
	json = jsoniter.Config{UseNumber: false}.Froze()

	// use a single instance of Decoder, it caches struct info
	formDecoder = form.NewDecoder()
)

var (
	uni      = ut.New(en.New(), en.New(), zh.New())
	validate = validator.New()
)

func BindQuery[T any](req *http.Request) (*TEntity[T], error) {

	data := new(T)

	err := formDecoder.Decode(data, req.URL.Query())
	if err != nil {
		return nil, err
	}

	return &TEntity[T]{
		Data:   *data,
		Errors: make(map[string]string),
	}, nil
}

func BindForm[T any](req *http.Request) (*TEntity[T], error) {

	data := new(T)

	err := req.ParseForm()
	if err != nil {
		return nil, err
	}

	// r.PostForm is a map of our POST form values
	err = formDecoder.Decode(data, req.PostForm)
	if err != nil {
		return nil, err
	}

	return &TEntity[T]{
		Data:   *data,
		Errors: make(map[string]string),
	}, nil

}

func BindJSON[T any](req *http.Request) (*TEntity[T], error) {
	data := new(T)

	buf, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, data)

	if err != nil {
		return nil, err
	}

	return &TEntity[T]{
		Data:   *data,
		Errors: make(map[string]string),
	}, nil

}

type TEntity[T any] struct {
	Data   T                 `json:"data"`
	Errors map[string]string `json:"errors"`
}

func (t *TEntity[T]) Validate(languages ...string) bool {
	err := validate.Struct(t.Data)
	if err == nil {
		return true
	}

	errs := err.(validator.ValidationErrors)

	ut, ok := uni.FindTranslator(languages...)
	if !ok {
		ut = uni.GetFallback()
	}

	for _, err := range errs {
		n := err.Field()
		if n == "" {
			n = err.StructField()
		}

		t.Errors[n] = err.Translate(ut)
	}

	return false
}
