package htmx

import (
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	trans "github.com/go-playground/validator/v10/translations/en"
)

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