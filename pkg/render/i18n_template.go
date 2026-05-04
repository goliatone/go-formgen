package render

import (
	"fmt"
	"reflect"
	"strings"
)

// TemplateI18nConfig configures template-level translation helpers.
type TemplateI18nConfig struct {
	// LocaleKey selects the field/key used to infer locale from template data
	// when callers pass a struct or map instead of a raw string.
	LocaleKey string
	// FuncName customizes the translator helper name (defaults to "translate").
	FuncName string
	// OnMissing controls the string returned when a translation is missing.
	OnMissing MissingTranslationHandler
}

// TemplateI18nFuncs returns a map suitable for injecting into go-template
// engines (e.g. via vanilla.WithTemplateFuncs / preact.WithTemplateFuncs).
//
// The main helper signature is:
//
//	translate(localeSrc, key, ...args) string
//
// Where localeSrc can be a string locale (e.g. "en-US") or a map/struct that
// contains a locale value under cfg.LocaleKey.
func TemplateI18nFuncs(t Translator, cfg TemplateI18nConfig) map[string]any {
	localeKey := strings.TrimSpace(cfg.LocaleKey)
	if localeKey == "" {
		localeKey = "locale"
	}

	translateName := strings.TrimSpace(cfg.FuncName)
	if translateName == "" {
		translateName = "translate"
	}

	onMissing := cfg.OnMissing
	if onMissing == nil {
		onMissing = missingTranslationDefault
	}

	return map[string]any{
		translateName: func(localeSrc any, key string, params ...any) string {
			key = strings.TrimSpace(key)
			if key == "" {
				return ""
			}
			locale := resolveLocale(localeSrc, localeKey)
			if t == nil {
				return onMissing(locale, key, params, ErrMissingTranslator)
			}
			msg, err := t.Translate(locale, key, params...)
			if err != nil || strings.TrimSpace(msg) == "" {
				return onMissing(locale, key, params, err)
			}
			return msg
		},
		"current_locale": func(localeSrc any) string {
			return resolveLocale(localeSrc, localeKey)
		},
	}
}

func resolveLocale(src any, key string) string {
	if src == nil {
		return ""
	}

	if str, ok := src.(string); ok {
		return str
	}

	if key == "" {
		return ""
	}

	if locale := localeFromKnownMap(src, key); locale != "" {
		return locale
	}
	return localeFromReflection(src, key)
}

func localeFromKnownMap(src any, key string) string {
	switch data := src.(type) {
	case map[string]any:
		value, ok := data[key]
		if !ok {
			return ""
		}
		return localeFromAnyValue(value)
	case map[string]string:
		return data[key]
	default:
		return ""
	}
}

func localeFromAnyValue(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func localeFromReflection(src any, key string) string {
	value := reflect.ValueOf(src)
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}

	if !value.IsValid() {
		return ""
	}

	switch value.Kind() {
	case reflect.Struct:
		field := value.FieldByNameFunc(func(name string) bool {
			return name == key
		})
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	case reflect.Map:
		if value.Type().Key().Kind() == reflect.String {
			val := value.MapIndex(reflect.ValueOf(key))
			if val.IsValid() && val.Kind() == reflect.String {
				return val.String()
			}
		}
	}

	return ""
}
