package postgres

import "reflect"

type UpdateItem interface {
	IsSet() bool
	Get() any
}

// UpdateField represents a 3-state update field that can be set to a value, set
// to null, or left unchanged.
type UpdateField[T any] struct {
	// Value is the new value to set for the field. If nil, the field will be
	// set to null. If not provided, the field will remain unchanged.
	Value *T
	// False means undefined, omitted, or unchanged.
	Set bool
}

func (uf UpdateField[T]) IsSet() bool {
	return uf.Set
}

func (uf UpdateField[T]) Get() any {
	if uf.Value == nil {
		return nil
	}
	return *uf.Value
}

// IsUpdatePayloadEmpty checks if the given payload has any fields set for
// update.
func IsUpdatePayloadEmpty(payload any) bool {
	v := reflect.ValueOf(payload)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}

	for _, field := range v.Fields() {
		if item, ok := field.Interface().(UpdateItem); ok && item.IsSet() {
			return false
		}
	}
	return true
}
