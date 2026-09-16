package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	goTemplate "text/template"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// Load reads config.json from the directory containing the running
// executable, expands `{{.ENV_VAR}}` references against the process
// environment, unmarshals the result into cfg and validates it using the
// `validate` struct tags (powered by go-playground/validator).
//
// cfg must be a non-nil pointer to a struct, typically one that embeds
// config.Config.
func Load(cfg any) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("config: resolve executable path: %w", err)
	}
	return LoadFile(filepath.Join(filepath.Dir(exePath), "config.json"), cfg)
}

// LoadFile behaves like Load but reads the configuration from an explicit
// path instead of assuming config.json next to the executable.
func LoadFile(path string, cfg any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: read %s: %w", path, err)
	}

	expanded, err := expandEnv(raw)
	if err != nil {
		return fmt.Errorf("config: expand environment variables: %w", err)
	}

	var asMap map[string]interface{}
	if err := json.Unmarshal(expanded, &asMap); err != nil {
		return fmt.Errorf("config: parse %s: %w", path, err)
	}

	fillStructFromMap(asMap, reflect.ValueOf(cfg))

	if err := validate.Struct(cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// expandEnv treats raw as a Go template and executes it against the current
// process environment, so config.json can reference `{{.MY_ENV_VAR}}`.
// Referencing a variable that is not set is an error: silently expanding it
// would produce a subtly misconfigured application instead of a clear
// startup failure.
func expandEnv(raw []byte) ([]byte, error) {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}

	tmpl, err := goTemplate.New("config").Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, env); err != nil {
		return nil, err
	}
	return []byte(out.String()), nil
}

// fillStructFromMap copies values from a generically-decoded JSON map into a
// (possibly nested) struct using reflection, performing best-effort type
// coercion. This allows config.json values produced by template expansion
// (which are always strings) to land in typed struct fields such as int,
// bool or time.Duration.
func fillStructFromMap(from map[string]interface{}, rv reflect.Value) {
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldVal := rv.Field(i)
		if !fieldVal.CanSet() {
			continue
		}

		// Embedded structs (e.g. config.Config embedded in a caller's own
		// configuration type) are filled from the same map level.
		if field.Anonymous {
			fv := fieldVal
			if fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				fv = fv.Elem()
			}
			if fv.Kind() == reflect.Struct {
				fillStructFromMap(from, fv)
			}
			continue
		}

		key := jsonKey(field)
		if key == "-" {
			continue
		}

		fromVal, ok := lookupCaseInsensitive(from, key)
		if !ok {
			continue
		}

		fv := fieldVal
		if fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				fv.Set(reflect.New(fv.Type().Elem()))
			}
			fv = fv.Elem()
		}

		if nested, ok := fromVal.(map[string]interface{}); ok && fv.Kind() == reflect.Struct {
			fillStructFromMap(nested, fv)
			continue
		}

		setScalar(fv, fromVal)
	}
}

func lookupCaseInsensitive(m map[string]interface{}, key string) (interface{}, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

// jsonKey returns the JSON key a struct field maps to, honoring the `json`
// tag and falling back to a lower-camel-case version of the field name.
func jsonKey(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	name := lowerFirst(field.Name)
	if tag == "" {
		return name
	}
	parts := strings.SplitN(tag, ",", 2)
	if parts[0] == "" {
		return name
	}
	return parts[0]
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// setScalar assigns val to fv, coercing between JSON's native types
// (string/float64/bool) and the destination field's Go type, including
// time.Duration.
func setScalar(fv reflect.Value, val interface{}) {
	if !fv.CanSet() {
		return
	}

	str := toStringValue(val)
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(str)
	case reflect.Bool:
		if b, err := strconv.ParseBool(str); err == nil {
			fv.SetBool(b)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		if n, err := strconv.ParseInt(str, 10, 64); err == nil {
			fv.SetInt(n)
		}
	case reflect.Int64:
		// Special-case time.Duration so config.json may use "30s" style
		// strings instead of raw nanosecond counts.
		if d, err := time.ParseDuration(str); err == nil {
			fv.SetInt(int64(d))
			return
		}
		if n, err := strconv.ParseInt(str, 10, 64); err == nil {
			fv.SetInt(n)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, err := strconv.ParseUint(str, 10, 64); err == nil {
			fv.SetUint(n)
		}
	case reflect.Float32, reflect.Float64:
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			fv.SetFloat(f)
		}
	}
}

// toStringValue converts a generically-decoded JSON value to its string
// representation, avoiding scientific notation for whole-number floats.
func toStringValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}
