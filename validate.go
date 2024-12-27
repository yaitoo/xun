package htmx

import (
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	trans "github.com/go-playground/validator/v10/translations/en"
)

// Validator validates struct and field values.
//
// It uses the specified languages to find an appropriate validator and
// translates the error messages.
type Validator struct {
	*validator.Validate
	Translator ut.Translator
}

var (
	validators       = make(map[string]*Validator)
	defaultValidator *Validator
)

func init() {
	uni := ut.New(en.New())
	defaultValidator = AddValidator(uni.GetFallback(), trans.RegisterDefaultTranslations)

	trans.RegisterDefaultTranslations(defaultValidator.Validate, defaultValidator.Translator) // nolint: errcheck

	validators[defaultValidator.Translator.Locale()] = defaultValidator
}

// AddValidator adds a new validator and translator to the map.
//
// It also registers the translations for the default locale.
func AddValidator(trans ut.Translator, register func(v *validator.Validate, trans ut.Translator) (err error)) *Validator {
	v := &Validator{
		Validate:   validator.New(),
		Translator: trans,
	}
	register(v.Validate, v.Translator) //nolint: errcheck
	validators[trans.Locale()] = v
	return v
}

func findValidator(locales ...string) *Validator {
	for _, locale := range locales {
		if v, ok := validators[locale]; ok {
			return v
		}
	}
	return defaultValidator
}
