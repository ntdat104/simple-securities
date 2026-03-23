package model

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

type Meta struct {
	QueryFields  string
	InsertFields string
	UpdateFields string
}

var cache sync.Map

func GetMeta(model any) *Meta {
	t := reflect.TypeOf(model)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if v, ok := cache.Load(t); ok {
		return v.(*Meta)
	}

	meta := buildMeta(t)
	cache.Store(t, meta)

	return meta
}

func buildMeta(t reflect.Type) *Meta {

	var queryFields []string
	var insertFields []string
	var updateFields []string

	for i := 0; i < t.NumField(); i++ {

		field := t.Field(i)
		dbTag := field.Tag.Get("db")

		if dbTag == "" {
			continue
		}

		col := strings.Split(dbTag, ",")[0]

		queryFields = append(queryFields, col)

		// skip auto id
		if col != "id" {
			insertFields = append(insertFields, col)
		}

		// skip immutable
		if col != "id" &&
			col != "uuid" &&
			col != "created_at" &&
			col != "created_by" {

			updateFields = append(updateFields,
				fmt.Sprintf("%s = :%s", col, col))
		}
	}

	return &Meta{
		QueryFields:  strings.Join(queryFields, ", "),
		InsertFields: strings.Join(insertFields, ", "),
		UpdateFields: strings.Join(updateFields, ", "),
	}
}
