// Package agenticrctest walks .agenticrc.toml config types by their toml tags, for tests that check every key is
// wired up. It takes the type as a parameter, so agenticrc's own tests can use it without an import cycle.
package agenticrctest

import (
	"fmt"
	"reflect"
	"strings"
)

// MapKey is the key Filled puts in every map, so two filled configs merge per key.
const MapKey = "x"

// Keys returns the dotted toml key of every setting in T, in field order: embedded structs share their parent's
// table, and a list of tables (run.proxy.credentials) or a map (run.args) is one key.
func Keys[T any]() []string {
	var keys []string
	walk(zero[T](), "", nil, func(key string, _ reflect.Value) {
		keys = append(keys, key)
	})
	return keys
}

// AllKeys returns Keys plus the keys inside each list of tables, e.g. run.proxy.credentials.host.
func AllKeys[T any]() []string {
	return allKeys(reflect.TypeFor[T](), "")
}

// Tables returns the dotted name of every nested table in T, e.g. run.proxy.
func Tables[T any]() []string {
	var tables []string
	walk(zero[T](), "", func(key string) {
		tables = append(tables, key)
	}, nil)
	return tables
}

// Leaves returns the value of every setting in cfg by its key from Keys.
func Leaves[T any](cfg *T) map[string]reflect.Value {
	leaves := make(map[string]reflect.Value)
	walk(reflect.ValueOf(cfg).Elem(), "", nil, func(key string, v reflect.Value) {
		leaves[key] = v
	})
	return leaves
}

// Filled returns a T with every setting set: strings to s, bools and *bools to true, slices to one filled element,
// maps to one filled entry under MapKey, structs recursively.
func Filled[T any](s string) T {
	var cfg T
	fill(reflect.ValueOf(&cfg).Elem(), s)
	return cfg
}

// allKeys returns the keys of t's settings under prefix, recursing into the element of each list of tables.
func allKeys(t reflect.Type, prefix string) []string {
	var keys []string
	walk(reflect.New(t).Elem(), prefix, nil, func(key string, v reflect.Value) {
		keys = append(keys, key)
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Struct {
			keys = append(keys, allKeys(v.Type().Elem(), key+".")...)
		}
	})
	return keys
}

// walk calls table for every nested table in struct v and setting for every other field, each with its dotted key.
func walk(v reflect.Value, prefix string, table func(key string), setting func(key string, v reflect.Value)) {
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")
		switch {
		case field.Anonymous:
			walk(v.Field(i), prefix, table, setting)
		case field.Type.Kind() == reflect.Struct:
			if table != nil {
				table(prefix + name)
			}
			walk(v.Field(i), prefix+name+".", table, setting)
		case setting != nil:
			setting(prefix+name, v.Field(i))
		}
	}
}

// fill sets v and everything below it to a non-zero value built from s; a kind no config uses yet panics.
func fill(v reflect.Value, s string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem(), s)
	case reflect.Slice:
		elem := reflect.New(v.Type().Elem()).Elem()
		fill(elem, s)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), elem))
	case reflect.Map:
		val := reflect.New(v.Type().Elem()).Elem()
		fill(val, s)
		v.Set(reflect.MakeMap(v.Type()))
		v.SetMapIndex(reflect.ValueOf(MapKey).Convert(v.Type().Key()), val)
	case reflect.Struct:
		for i := range v.NumField() {
			fill(v.Field(i), s)
		}
	default:
		panic(fmt.Sprintf("agenticrctest: no fill value for kind %s", v.Kind()))
	}
}

// zero returns an addressable zero T.
func zero[T any]() reflect.Value {
	return reflect.New(reflect.TypeFor[T]()).Elem()
}
