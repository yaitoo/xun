package xun

import (
	"net/http"

	"github.com/go-playground/form/v4"
	"github.com/go-playground/validator/v10"
	jsoniter "github.com/json-iterator/go"
)

var (
	json = jsoniter.Config{UseNumber: false}.Froze()

	// use a single instance of Decoder, it caches struct info
	formDecoder = form.NewDecoder()
)

// BindQuery binds the query string to the given struct.
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

// BindForm binds the request body to the given struct.
//
// It supports application/x-www-form-urlencoded, multipart/form-data.
//
// If the request body is empty or the decoding fails, it returns an error.
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

// BindJson binds the JSON request body to the given struct.
//
// It attempts to decode the JSON body into the specified type.
//
// If the decoding fails, it returns an error.
func BindJson[T any](req *http.Request) (*TEntity[T], error) {
	data := new(T)

	err := json.NewDecoder(req.Body).Decode(data)
	if err != nil {
		return nil, err
	}

	return &TEntity[T]{
		Data:   *data,
		Errors: make(map[string]string),
	}, nil

}

// TEntity is a struct that contains the data and errors.
//
// It is used by the Bind functions to return the data and errors.
type TEntity[T any] struct {
	Data   T                 `json:"data"`
	Errors map[string]string `json:"errors"`
}

// Validate checks the data against the validation rules and populates errors if any.
//
// It uses the specified languages to find an appropriate validator and
// translates the error messages. Returns true if validation passes, otherwise false.
func (t *TEntity[T]) Validate(languages ...string) bool {
	validate := findValidator(languages...)

	err := validate.Struct(t.Data)
	if err == nil {
		return true
	}

	errs := err.(validator.ValidationErrors)

	for _, err := range errs {
		n := err.Field()
		if n == "" {
			n = err.StructField()
		}

		t.Errors[n] = err.Translate(validate.Translator)
	}

	return false
}
