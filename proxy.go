package candyjs

import "C"
import (
	"encoding/json"
	"errors"
	"reflect"
)

// ErrUndefinedProperty is throw when a property for a given proxied object on
// javascript cannot be found, basically a valid method or field cannot found.
var ErrUndefinedProperty = errors.New("undefined property")

var (
	p = &proxy{}

	//internalKeys map contains the keys that are called by duktape and cannot
	//throw an error, the value of the map is the value returned when this keys
	//are requested.
	internalKeys = map[string]interface{}{
		"toJSON": nil, "valueOf": nil,
		"toString": func() string { return "[candyjs Proxy]" },
	}
)

// Proxy defines the GO interface for ECMASCRIPTs proxy objects.
type Proxy interface {
	Has(t interface{}, k string) bool
	Get(t interface{}, k string, recv interface{}) (interface{}, error)
	Set(t interface{}, k string, v, recv interface{}) (bool, error)
	Enumerate(t interface{}) (interface{}, error)
}

type proxy struct{}

func (p *proxy) Has(t interface{}, k string) bool {
	_, err := p.getProperty(t, k)
	return err != ErrUndefinedProperty
}

func (p *proxy) Get(t interface{}, k string, recv interface{}) (interface{}, error) {
	f, err := p.getProperty(t, k)
	if err != nil {
		if k == "toJSON" {
			// use GO's JSON marshalling for proxies
			// e.g. time.Time will correctly be marshalled into an RFC3339 date/time string-
			//      without this it would get "{}"
			return p.jsonMarshaller(t), nil
		}

		if v, isInternal := internalKeys[k]; isInternal {
			return v, nil
		}

		return nil, err
	}

	return f.Interface(), nil
}

func (p *proxy) Set(t interface{}, k string, v, recv interface{}) (bool, error) {
	f, err := p.getProperty(t, k)
	if err != nil {
		return false, err
	}

	if !f.CanSet() {
		return false, nil
	}

	value := reflect.Zero(f.Type())
	if v != nil {
		v, err = convert(f, v)
		if err != nil {
			return false, nil
		}
		value = reflect.ValueOf(v)
	}

	f.Set(value)
	return true, nil
}

func (p *proxy) getProperty(t interface{}, key string) (reflect.Value, error) {
	v := reflect.ValueOf(t)
	r, found := p.getValueFromKind(key, v)
	if !found {
		return r, ErrUndefinedProperty
	}

	return r, nil
}

func (p *proxy) getValueFromKind(key string, v reflect.Value) (reflect.Value, bool) {
	var value reflect.Value
	var found bool
	switch v.Kind() {
	case reflect.Ptr:
		value, found = p.getValueFromKindPtr(key, v)
	case reflect.Struct:
		value, found = p.getValueFromKindStruct(key, v)
	case reflect.Map:
		value, found = p.getValueFromKindMap(key, v)
	}

	if !found {
		return p.getMethod(key, v)
	}

	return value, found
}

func (p *proxy) getValueFromKindPtr(key string, v reflect.Value) (reflect.Value, bool) {
	r, found := p.getMethod(key, v)
	if !found {
		return p.getValueFromKind(key, v.Elem())
	}

	return r, found
}

func (p *proxy) getValueFromKindStruct(key string, v reflect.Value) (reflect.Value, bool) {
	var r reflect.Value
	for _, name := range nameToGo(key) {
		r = v.FieldByName(name)
		if r.IsValid() {
			break
		}
	}

	return r, r.IsValid()
}

func (p *proxy) getValueFromKindMap(key string, v reflect.Value) (reflect.Value, bool) {
	keyValue := reflect.ValueOf(key)
	r := v.MapIndex(keyValue)

	return r, r.IsValid()
}

func (p *proxy) getMethod(key string, v reflect.Value) (reflect.Value, bool) {
	var r reflect.Value
	for _, name := range nameToGo(key) {
		r = v.MethodByName(name)
		if r.IsValid() {
			break
		}
	}

	return r, r.IsValid()
}

func (p *proxy) Enumerate(t interface{}) (interface{}, error) {
	return p.getPropertyNames(t)
}

func (p *proxy) getPropertyNames(t interface{}) ([]string, error) {
	v := reflect.ValueOf(t)

	var names []string
	var err error
	switch v.Kind() {
	case reflect.Ptr:
		names, err = p.getPropertyNames(v.Elem().Interface())
		if err != nil {
			return nil, err
		}
	case reflect.Struct:
		cFields := v.NumField()
		for i := 0; i < cFields; i++ {
			fieldName := v.Type().Field(i).Name
			if !isExported(fieldName) {
				continue
			}

			names = append(names, nameToJavaScript(fieldName))
		}
	}

	mCount := v.NumMethod()
	for i := 0; i < mCount; i++ {
		methodName := v.Type().Method(i).Name
		if !isExported(methodName) {
			continue
		}

		names = append(names, nameToJavaScript(methodName))
	}

	return names, nil
}

func convert(t reflect.Value, value interface{}) (interface{}, error) {

	s := reflect.ValueOf(value)
	if t.Type() == s.Type() {
		return value, nil // no conversion required
	}

	value, ok := castNumberToGoType(t.Kind(), value)
	if ok {
		return value, nil
	}

	return convertUsingJSON(t, value)
}

func convertUsingJSON(t reflect.Value, value interface{}) (interface{}, error) {

	tv := reflect.New(t.Type()).Interface()
	js, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal([]byte(js), tv)
	}
	if err != nil {
		return nil, err
	}

	return reflect.ValueOf(tv).Elem().Interface(), nil
}

func castNumberToGoType(k reflect.Kind, v interface{}) (interface{}, bool) {
	switch k {
	case reflect.Int:
		v = int(v.(float64))
	case reflect.Int8:
		v = int8(v.(float64))
	case reflect.Int16:
		v = int16(v.(float64))
	case reflect.Int32:
		v = int32(v.(float64))
	case reflect.Int64:
		v = int64(v.(float64))
	case reflect.Uint:
		v = uint(v.(float64))
	case reflect.Uint8:
		v = uint8(v.(float64))
	case reflect.Uint16:
		v = uint16(v.(float64))
	case reflect.Uint32:
		v = uint32(v.(float64))
	case reflect.Uint64:
		v = uint64(v.(float64))
	case reflect.Float32:
		v = float32(v.(float64))
	default:
		return v, false
	}

	return v, true
}

func (p *proxy) jsonMarshaller(t interface{}) interface{} {
	return func(val interface{}, key interface{}) interface{} {
		js, err := json.Marshal(t)
		if err != nil {
			return t
		}
		// turn the JSON back into an object tree
		// this is either a primitive type, map[string]interface{} or []interface{}
		var v interface{}
		err = json.Unmarshal(js, &v)
		if err != nil {
			return t
		}
		return v
	}
}
