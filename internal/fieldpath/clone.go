package fieldpath

import "reflect"

// Clone detaches acyclic JSON-compatible values, preserving concrete container
// and numeric types. Raw-object parsing accepts typed containers as well as the
// map[string]any/[]any shapes produced by JSON decoding. No JSON round trip is
// used: int64, json.Number, byte slices and named types remain lossless.
func Clone(value any) any {
	if value == nil {
		return nil
	}
	return cloneValue(reflect.ValueOf(value)).Interface()
}

func cloneValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(cloneValue(value.Elem()))
		return out
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(cloneValue(value.Elem()))
		return out.Convert(value.Type())
	case reflect.Map:
		return cloneMap(value)
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		cloneElements(out, value)
		return out
	case reflect.Array:
		out := reflect.New(value.Type()).Elem()
		cloneElements(out, value)
		return out
	case reflect.Struct:
		return cloneStruct(value)
	default:
		return value
	}
}

func cloneMap(value reflect.Value) reflect.Value {
	if value.IsNil() {
		return value
	}
	out := reflect.MakeMapWithSize(value.Type(), value.Len())
	entries := value.MapRange()
	for entries.Next() {
		out.SetMapIndex(entries.Key(), cloneValue(entries.Value()))
	}
	return out
}

func cloneElements(out, value reflect.Value) {
	for i := 0; i < value.Len(); i++ {
		out.Index(i).Set(cloneValue(value.Index(i)))
	}
}

func cloneStruct(value reflect.Value) reflect.Value {
	out := reflect.New(value.Type()).Elem()
	out.Set(value)
	// Only exported fields participate in the ordinary JSON value shape.
	for i := 0; i < value.NumField(); i++ {
		if out.Field(i).CanSet() && value.Field(i).CanInterface() {
			out.Field(i).Set(cloneValue(value.Field(i)))
		}
	}
	return out
}
