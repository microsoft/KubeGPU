package utils

import (
	"fmt"
	"reflect"
)

func toInterfaceArray(val interface{}) []interface{} {
	t := reflect.TypeOf(val)
	tv := reflect.ValueOf(val)
	if t.Kind() == reflect.Array || t.Kind() == reflect.Slice {
		a := make([]interface{}, tv.Len())
		for i := 0; i < tv.Len(); i++ {
			a[i] = tv.Index(i).Interface()
		}
		return a
	}
	panic("Not an array or slice")
}

func assignMapK(x interface{}, keys []interface{}, val interface{}) {
	t := reflect.TypeOf(x)
	if t.Kind() == reflect.Map {
		mv := reflect.ValueOf(x)
		key0, keyR := keys[0], keys[1:]
		if len(keyR) == 0 {
			// at end
			mv.SetMapIndex(reflect.ValueOf(key0), reflect.ValueOf(val))
		} else {
			k := reflect.ValueOf(key0)
			v := mv.MapIndex(k)
			if v == reflect.ValueOf(nil) {
				v = reflect.MakeMap(t.Elem())
				mv.SetMapIndex(reflect.ValueOf(key0), v)
			}
			assignMapK(v.Interface(), keyR, val)
		}
	} else {
		panic("Not a map")
	}
}

func AssignMap(x interface{}, keys interface{}, val interface{}) {
	keysA := toInterfaceArray(keys)
	assignMapK(x, keysA, val)
}

func getMapK(x interface{}, keys []interface{}) interface{} {
	t := reflect.TypeOf(x)
	if t.Kind() == reflect.Map {
		mv := reflect.ValueOf(x)
		key0, keyR := keys[0], keys[1:]
		v := mv.MapIndex(reflect.ValueOf(key0))
		if len(keyR) == 0 {
			// at end
			if v == reflect.ValueOf(nil) {
				return reflect.Zero(t.Elem()).Interface()
			}
			return v.Interface()
		}
		return getMapK(v.Interface(), keyR)
	}
	panic("Not a map")
}

func GetMap(x interface{}, keys interface{}) interface{} {
	return getMapK(x, toInterfaceArray(keys))
}

// Cannot do since no way to figure out if map contains a particular value, MapIndex only returns zero value, not a bool specifying
// if map contains key
//
// func CompareMaps(x interface {}, y interface{}) {
// 	tx := reflect.TypeOf(x)
// 	ty := reflect.TypeOf(y)
// 	if !(tx.Kind() == reflect.Map && ty.Kind() == reflect.Map) {
// 		fmt.Printf("Both are not map types: %v %v", tx.Kind(), ty.Kind())
// 		return
// 	}
// 	mx := reflect.ValueOf(x)
// 	my := reflect.ValueOf(y)
// 	ex := reflect.TypeOf(mx.Elem())
// 	ey := reflect.TypeOf(my.Elem())
// 	if (ex != ey) {
// 		fmt.Printf("Map types don't match %v %v", ex, ey)
// 		return
// 	}
// 	if (ex.Key() != ey.Key()) {
// 		fmt.Printf("Map keys don't match %v %v", ex.Key(), ey.Key())
// 		return
// 	}

// 	if (mx.Len != my.Len) {
// 		fmt.Printf("Lengths don't match first: %d second: %d", mx.Len, my.Len)
// 	}
// 	keyx := mx.Keys()
// 	keyy := my.Keys()
// 	for _, key := range keyx {
// 		vx :=
// 	}
// }

func CompareMapStringString(x map[string]string, y map[string]string) {
	if len(x) != len(y) {
		fmt.Printf("Lengths don't match %d %d\n", len(x), len(y))
	}
	for keyx, valx := range x {
		valy, ok := y[keyx]
		if !ok {
			fmt.Printf("Key %s does not exist in second first: %s\n", keyx, valx)
		} else if valx != valy {
			fmt.Printf("Key %s does not match first: %s second: %s\n", keyx, valx, valy)
		}
	}
	for keyy, valy := range y {
		_, ok := x[keyy]
		if !ok {
			fmt.Printf("Key %s does not exist in first second: %s\n", keyy, valy)
		}
	}
}
