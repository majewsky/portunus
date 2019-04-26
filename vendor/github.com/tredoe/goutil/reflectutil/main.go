// Copyright 2014 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package reflectutil implements some reflection utility functions.
package reflectutil

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
)

// GetFunctionName returns the name of a function.
func GetFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// PrintStruct prints the field names and values of the given struct.
// It is used to debug.
func PrintStruct(v interface{}) {
	valueof := reflect.ValueOf(v).Elem()
	typeof := valueof.Type()
	var value interface{}

	for i := 0; i < valueof.NumField(); i++ {
		fieldT := typeof.Field(i)
		fieldV := valueof.Field(i)

		switch fieldV.Kind() {
		case reflect.Bool:
			value = fieldV.Bool()
		case reflect.Int:
			value = strconv.Itoa(int(fieldV.Int()))
		case reflect.Slice:
			//value = fieldV.Slice(0, fieldV.Len())

			/*for j := 0; j < fieldV.NumField(); j++ {
				fmt.Println(fieldV.Index[j])
			}*/
			//fmt.Println(fieldV.Elem())

			fallthrough
		case reflect.String:
			value = fieldV.String()
		default:
			panic(fieldV.Kind().String() + ": type not added")
		}

		fmt.Printf(" %s: %v\n", fieldT.Name, value)
	}
}
