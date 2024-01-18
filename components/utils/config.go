package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

var ErrJsonEmpty = errors.New("json is empty")

type configParams struct {
	fieldName  string
	isRequired bool
	isSecret   bool
}

func getConfigString(cfg *config.Config, params configParams) string {
	if params.isRequired {
		return cfg.Require(params.fieldName)
	} else {
		return cfg.Get(params.fieldName)
	}
}

func getConfigBool(cfg *config.Config, params configParams) bool {
	if params.isRequired {
		return cfg.RequireBool(params.fieldName)
	} else {
		return cfg.GetBool(params.fieldName)
	}
}

func getConfigInt(cfg *config.Config, params configParams) int {
	if params.isRequired {
		return cfg.RequireInt(params.fieldName)
	} else {
		return cfg.GetInt(params.fieldName)
	}
}

func getConfigFloat(cfg *config.Config, params configParams) float64 {
	if params.isRequired {
		return cfg.RequireFloat64(params.fieldName)
	} else {
		return cfg.GetFloat64(params.fieldName)
	}
}

func getSecretString(cfg *config.Config, params configParams) pulumi.StringOutput {
	if params.isRequired {
		return cfg.RequireSecret(params.fieldName)
	} else {
		return cfg.GetSecret(params.fieldName)
	}
}

func getSecretBool(cfg *config.Config, params configParams) pulumi.BoolOutput {
	if params.isRequired {
		return cfg.RequireSecretBool(params.fieldName)
	} else {
		return cfg.GetSecretBool(params.fieldName)
	}
}

func getSecretInt(cfg *config.Config, params configParams) pulumi.IntOutput {
	if params.isRequired {
		return cfg.RequireSecretInt(params.fieldName)
	} else {
		return cfg.GetSecretInt(params.fieldName)
	}
}

func getSecretFloat(cfg *config.Config, params configParams) pulumi.Float64Output {
	if params.isRequired {
		return cfg.RequireSecretFloat64(params.fieldName)
	} else {
		return cfg.GetSecretFloat64(params.fieldName)
	}
}

func loadJsonConfig(cfg *config.Config, fieldName string, isRequired bool, curr interface{}) ([]byte, error) {
	cfgJson := cfg.Get(fieldName)
	if isRequired && curr == nil && cfgJson == "" {
		return nil, fmt.Errorf("config %s is required", fieldName)
	} else if cfgJson == "" {
		return nil, ErrJsonEmpty
	}
	return []byte(cfgJson), nil
}

// ExtractConfig extracts the pulumi config and populates the object.
// It requires tags to be set on the struct fields, e.g.:
// json/config - the name of the config key
// secret - the name of the secret config key
// required - whether the config is required
// The tags are used to map the config to the struct fields.
func ExtractConfig(ctx *pulumi.Context, namespace string, obj interface{}) error {
	cfg := config.New(ctx, namespace)
	// Get the reflect.Value of the object
	v := reflect.ValueOf(obj)

	// Check if the object is a pointer to a struct
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return errors.New("obj must be a pointer to a struct")
	}

	// Get the reflect.Type of the object
	t := v.Elem().Type()

	// Iterate over the fields of the struct
	for i := 0; i < t.NumField(); i++ {
		// Get the reflect.Value of the field
		fv := v.Elem().Field(i)
		ff := t.Field(i)

		isSecret := false
		// Get the name of the field
		var fieldName string
		// Get the tag of the field. If the tag is not empty, use it as the field name
		if tagConfig := ff.Tag.Get("config"); tagConfig != "" {
			fieldName = tagConfig
		} else if tagConfig := ff.Tag.Get("json"); tagConfig != "" {
			// fallback to json tag
			fieldName = tagConfig
		} else if tagConfig := ff.Tag.Get("secret"); tagConfig != "" {
			fieldName = tagConfig
			isSecret = true
		} else {
			// Skip field if config tag not found
			continue
		}
		_, isRequired := ff.Tag.Lookup("required")

		params := configParams{
			fieldName:  fieldName,
			isRequired: isRequired,
			isSecret:   isSecret,
		}
		// Get the value of the field from the config
		switch fv.Kind() {
		case reflect.Bool:
			params.isRequired = isRequired && fv.Bool()
			val := getConfigBool(cfg, params)
			if isSecret {
				return fmt.Errorf("field '%s' is marked as secret but type is not a pulumi output", fieldName)
			}
			if val {
				fv.SetBool(val)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			params.isRequired = isRequired && fv.Int() == 0
			val := getConfigInt(cfg, params)
			if isSecret {
				return fmt.Errorf("field '%s' is marked as secret but type is not a pulumi output", fieldName)
			}
			if val != 0 {
				fv.SetInt(int64(val))
			}
		case reflect.Float32, reflect.Float64:
			params.isRequired = isRequired && fv.Float() == 0.0
			val := getConfigFloat(cfg, params)
			if isSecret {
				return fmt.Errorf("field '%s' is marked as secret but type is not a pulumi output", fieldName)
			}
			if val != 0.0 {
				fv.SetFloat(val)
			}
		case reflect.String:
			params.isRequired = isRequired && fv.String() == ""
			val := getConfigString(cfg, params)
			if isSecret {
				return fmt.Errorf("field '%s' is marked as secret but type is not a pulumi output", fieldName)
			}
			if val != "" {
				fv.SetString(val)
			}
		case reflect.Map:
			bytes, err := loadJsonConfig(cfg, fieldName, isRequired, fv.Interface())
			if err != nil {
				if errors.Is(err, ErrJsonEmpty) {
					continue
				}
				return fmt.Errorf("failed to load json config for field '%s': %w", fieldName, err)
			}
			if err = json.Unmarshal(bytes, fv.Addr().Interface()); err != nil {
				return fmt.Errorf("failed to unmarshal json config for field '%s': %w", fieldName, err)
			}
		case reflect.Struct, reflect.Ptr, reflect.Array, reflect.Slice:
			var val reflect.Value
			if fv.Kind() == reflect.Ptr {
				// handle pointer to struct
				if !fv.IsNil() {
					// directly use the pointer if not nil
					val = fv
				} else {
					val = reflect.New(ff.Type.Elem())
				}
			} else {
				if !fv.IsZero() {
					val = fv.Addr()
				} else {
					val = reflect.New(ff.Type)
				}
			}
			if data, err := loadJsonConfig(cfg, fieldName, isRequired, val.Interface()); err != nil {
				if errors.Is(err, ErrJsonEmpty) {
					continue
				}
				return fmt.Errorf("failed to load json config for field '%s': %w", fieldName, err)
			} else {
				if err = UnmarshalJSONConfig(data, val.Interface()); err != nil {
					return fmt.Errorf("failed to unmarshal json config for field '%s': %w", fieldName, err)
				}
			}
			if fv.Kind() == reflect.Ptr {
				fv.Set(val)
			} else {
				fv.Set(val.Elem())
			}
		case reflect.Interface:
			switch ff.Type {
			case reflect.TypeOf((*pulumi.StringInput)(nil)).Elem():
				curr, ok := fv.Interface().(pulumi.String)
				params.isRequired = isRequired && (fv.IsNil() || (ok && curr == ""))
				if isSecret {
					fv.Set(reflect.ValueOf(getSecretString(cfg, params)))
				} else {
					val := getConfigString(cfg, params)
					if val != "" && (ok || fv.IsNil()) {
						fv.Set(reflect.ValueOf(pulumi.String(val)))
					}
				}
				// the other case is that it's a string output so do nothing
			case reflect.TypeOf((*pulumi.BoolInput)(nil)).Elem():
				curr, ok := fv.Interface().(pulumi.Bool)
				params.isRequired = isRequired && (fv.IsNil() || (ok && !bool(curr)))
				if isSecret {
					fv.Set(reflect.ValueOf(getSecretBool(cfg, params)))
				} else {
					val := getConfigBool(cfg, params)
					if val && (ok || fv.IsNil()) {
						fv.Set(reflect.ValueOf(pulumi.Bool(val)))
					}
				}
				// the other case is that it's a bool output so do nothing
			case reflect.TypeOf((*pulumi.IntInput)(nil)).Elem():
				curr, ok := fv.Interface().(pulumi.Int)
				params.isRequired = isRequired && (fv.IsNil() || (ok && int(curr) == 0))
				if isSecret {
					fv.Set(reflect.ValueOf(getSecretInt(cfg, params)))
				} else {
					val := getConfigInt(cfg, params)
					if val != 0 && (ok || fv.IsNil()) {
						fv.Set(reflect.ValueOf(pulumi.Int(val)))
					}
				}
				// the other case is that it's a int output so do nothing
			case reflect.TypeOf((*pulumi.Float64Input)(nil)).Elem():
				curr, ok := fv.Interface().(pulumi.Float64)
				params.isRequired = isRequired && (fv.IsNil() || (ok && float64(curr) == 0.0))
				if isSecret {
					fv.Set(reflect.ValueOf(getSecretFloat(cfg, params)))
				} else {
					val := getConfigFloat(cfg, params)
					if val != 0.0 && (ok || fv.IsNil()) {
						fv.Set(reflect.ValueOf(pulumi.Float64(val)))
					}
				}
			}
		default:
			return fmt.Errorf("unsupported field name: %s, type: %v", fieldName, fv.Kind())
		}
	}

	return nil
}

func setFieldValue(fv reflect.Value, val interface{}, fieldName string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("failed to set field '%s' (type %v) to %v: %v", fieldName, fv.Elem().Kind(), val, e)
		}
	}()
	if !fv.CanSet() || fv.Kind() != reflect.TypeOf(val).Kind() {
		// handle exceptional cases individually
		if fv.Kind() == reflect.Int && reflect.TypeOf(val).Kind() == reflect.Float64 {
			// handle int field and float64 json value
			val = int(val.(float64))
		} else {
			err = fmt.Errorf("field '%s' is not settable to %v (%v)", fieldName, val, reflect.TypeOf(val))
			return
		}
	}
	fv.Set(reflect.ValueOf(val).Convert(fv.Type()))
	return
}

func unmarshallJSONMap(dict map[string]interface{}, obj interface{}) error {
	// Get the reflect.Value of the object
	v := reflect.ValueOf(obj)
	if obj == nil || v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return errors.New("obj must be a pointer to a struct")
	}
	// Get the reflect.Type of the object
	t := v.Elem().Type()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// fmt.Printf("setting '%v' to struct '%v', curr value: %+v\n", dict, t, v)

	// Iterate over the fields of the struct
	for i := 0; i < t.NumField(); i++ {
		ff := t.Field(i)
		// Get the reflect.Value of the field
		fe := v.Elem()
		if fe.Kind() == reflect.Pointer {
			fe = fe.Elem()
		}
		fv := fe.Field(i)

		// Get the name of the field
		var fieldName string
		// Get the tag of the field. If the tag is not empty, use it as the field name
		if tagConfig := ff.Tag.Get("config"); tagConfig != "" {
			fieldName = tagConfig
		} else if tagConfig := ff.Tag.Get("json"); tagConfig != "" {
			// fallback to json tag
			fieldName = tagConfig
		} else {
			// Skip field if config tag not found
			continue
		}

		switch fv.Kind() {
		case reflect.Interface:
			switch ff.Type {
			case reflect.TypeOf((*pulumi.StringInput)(nil)).Elem():
				_, ok := fv.Interface().(pulumi.String)
				if val, newOk := dict[fieldName]; newOk && (ok || fv.IsNil()) {
					fv.Set(reflect.ValueOf(pulumi.String(val.(string))))
				}
			case reflect.TypeOf((*pulumi.BoolInput)(nil)).Elem():
				_, ok := fv.Interface().(pulumi.Bool)
				if val, newOk := dict[fieldName]; newOk && (ok || fv.IsNil()) {
					fv.Set(reflect.ValueOf(pulumi.Bool(val.(bool))))
				}
			case reflect.TypeOf((*pulumi.IntInput)(nil)).Elem():
				_, ok := fv.Interface().(pulumi.Int)
				if val, newOk := dict[fieldName]; newOk && (ok || fv.IsNil()) {
					fv.Set(reflect.ValueOf(pulumi.Int(val.(int))))
				}
			case reflect.TypeOf((*pulumi.Float64Input)(nil)).Elem():
				_, ok := fv.Interface().(pulumi.Float64)
				if val, newOk := dict[fieldName]; newOk && (ok || fv.IsNil()) {
					fv.Set(reflect.ValueOf(pulumi.Float64(val.(float64))))
				}
			default:
				return fmt.Errorf("unsupported interface %v for field: %s", ff.Type, fieldName)
			}
		case reflect.Slice, reflect.Array, reflect.Struct, reflect.Pointer, reflect.Map:
			if _, ok := dict[fieldName]; !ok {
				continue
			}
			if fv.Kind() == reflect.Struct {
				if err := unmarshallJSONMap(dict[fieldName].(map[string]interface{}), fv.Addr().Interface()); err != nil {
					return fmt.Errorf("failed to unmarshal json map for field '%s': %w", fieldName, err)
				}
			} else if fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				if childDict, ok := dict[fieldName].(map[string]interface{}); ok {
					if err := unmarshallJSONMap(childDict, fv.Interface()); err != nil {
						return fmt.Errorf("failed to unmarshal json map for field '%s': %w", fieldName, err)
					}
				} else if childArr, ok := dict[fieldName].([]interface{}); ok {
					if err := unmarshallJSONArray(childArr, fv.Interface()); err != nil {
						return fmt.Errorf("failed to unmarshal json array for field '%s': %w", fieldName, err)
					}
				} else {
					// pointer to simple fields/interface
					if val, ok := dict[fieldName]; ok {
						return setFieldValue(fv.Elem(), val, fieldName)
					}
				}
			} else if fv.Kind() == reflect.Slice || fv.Kind() == reflect.Array {
				if fv.IsNil() {
					fv.Set(reflect.MakeSlice(fv.Type(), 0, 0))
				}
				if err := unmarshallJSONArray(dict[fieldName].([]interface{}), fv.Addr().Interface()); err != nil {
					return fmt.Errorf("failed to unmarshal json array for field '%s': %w", fieldName, err)
				}
			}
		default:
			if val, ok := dict[fieldName]; ok {
				if err := setFieldValue(fv, val, fieldName); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// unmarshallJSONArray unmarshalls a json array into a slice of struct.
// obj must be a pointer to a slice of struct.
// The default behavior is to append the existing slice from the config.
func unmarshallJSONArray(arr []interface{}, obj interface{}) error {
	if obj == nil {
		return errors.New("obj must be a pointer to a slice of struct")
	}
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("obj is of type %v, expected pointer to a slice", rv.Kind())
	}
	// initalLen is used to handle the case where the slice is already populated
	// the behavior is that we append the existing slice from the config
	initalLen := rv.Elem().Len()
	for i, val := range arr {
		dict := val.(map[string]interface{})
		if initalLen+i >= rv.Elem().Len() {
			rv.Elem().Set(reflect.Append(rv.Elem(), reflect.New(rv.Elem().Type().Elem()).Elem()))
		}
		if err := unmarshallJSONMap(dict, rv.Elem().Index(initalLen+i).Addr().Interface()); err != nil {
			return fmt.Errorf("failed to unmarshal json array at index %d: %w", i, err)
		}
	}
	return nil
}

func UnmarshalJSONConfig(data []byte, obj interface{}) error {
	if len(data) == 0 {
		return nil
	}

	var tmp interface{}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return fmt.Errorf("failed to unmarshal json (%s) into interface{}: %w", string(data), err)
	}

	switch val := tmp.(type) {
	case []interface{}:
		return unmarshallJSONArray(val, obj)
	case map[string]interface{}:
		return unmarshallJSONMap(val, obj)
	default:
		return fmt.Errorf("unsupported type %v", reflect.TypeOf(val))
	}
}

func MarshalJSONConfig(obj interface{}) ([]byte, error) {
	rVal := reflect.ValueOf(obj)

	if rVal.Kind() == reflect.Ptr {
		rVal = rVal.Elem()
	}
	if rVal.Kind() != reflect.Struct {
		return nil, fmt.Errorf("provided interface is not a struct: %v", rVal.Kind())
	}

	processedMetadata := make(map[string]interface{})
	for i := 0; i < rVal.NumField(); i++ {
		typeField := rVal.Type().Field(i)
		field := rVal.Field(i)
		key := typeField.Tag.Get("json")
		if key == "" {
			key = typeField.Name
		}
		switch field.Interface().(type) {
		case pulumi.StringOutput:
			processedMetadata[key] = "[StringOutput]"
		case pulumi.BoolOutput:
			processedMetadata[key] = "[BoolOutput]"
		case pulumi.IntOutput:
			processedMetadata[key] = "[IntOutput]"
		case pulumi.Float64Output:
			processedMetadata[key] = "[Float64Output]"
		default:
			processedMetadata[key] = field.Interface()
		}
	}
	return json.Marshal(processedMetadata)
}
