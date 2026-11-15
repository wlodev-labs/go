package postgres

import "strings"

func GetColumns(cols []string) string {
	return strings.Join(cols, ", ")
}

func GetColumnsWithPrefix(prefix string, cols []string) string {
	var prefixed []string
	for _, col := range cols {
		prefixed = append(prefixed, prefix+"."+col+` AS "`+col+`"`)
	}
	return strings.Join(prefixed, ", ")
}

// SliceToIdsMap transforms a slice of pointers to T into a slice of ints using
// an extractor function. This helps with preparing the IDs for SQL ANY clauses
// used for child relations instead of multiple joins. Postgres filters using
// the ANY clause, so duplicated IDs are not an issue, but we still want to
// avoid unnecessary duplicates for better readibility during debugging, so we
// use a map to ensure uniqueness.
func SliceToIdsSlice[T any](items []*T, extractor func(*T) int) []int {
	uniqueIds := make(map[int]struct{})
	for _, item := range items {
		id := extractor(item)
		uniqueIds[id] = struct{}{}
	}

	result := make([]int, 0, len(uniqueIds))
	for id := range uniqueIds {
		result = append(result, id)
	}
	return result
}

// SliceToOptionalIdsMap works like SliceToIdsMap but handles optional IDs
// (pointers to int). Sometimes we have relations that are optional, so the
// foreign key can be null. This function extracts only the non-nil IDs and
// returns them as a slice of ints.
func SliceToOptionalIdsSlice[T any](items []*T, extractor func(*T) *int) []int {
	uniqueIds := make(map[int]struct{})
	for _, item := range items {
		idPtr := extractor(item)
		if idPtr != nil {
			uniqueIds[*idPtr] = struct{}{}
		}
	}

	result := make([]int, 0, len(uniqueIds))
	for id := range uniqueIds {
		result = append(result, id)
	}
	return result
}

// SliceToStringSlice transforms a slice of pointers to T into a slice of
// strings using an extractor function. This helps with preparing the strings
// for SQL ANY clauses used for child relations instead of multiple joins.
// Postgres filters using the ANY clause, so duplicated strings are not an
// issue, but we still want to avoid unnecessary duplicates for better
// readibility during debugging, so we use a map to ensure uniqueness.
func SliceToStringSlice[T any](items []*T, extractor func(*T) string) []string {
	uniqueStrings := make(map[string]struct{})
	for _, item := range items {
		str := extractor(item)
		uniqueStrings[str] = struct{}{}
	}

	result := make([]string, 0, len(uniqueStrings))
	for str := range uniqueStrings {
		result = append(result, str)
	}
	return result
}

// SliceToIndexMap converts a slice of pointers to a map of pointers. It's
// helpful for quickly accessing items by their ID without iterating through the
// slice and it's primarily used in database query results merging, like
// associating child records with their parents.
func SliceToIndexMap[T any, K comparable](items []*T, keyExtractor func(*T) K) map[K]*T {
	result := make(map[K]*T, len(items))
	for _, item := range items {
		key := keyExtractor(item)
		result[key] = item
	}
	return result
}

// SliceToResponseMap transforms a slice of pointers to T into a map of R using
// keyExtractor and extractor functions. It's useful in a final stage of data
// processing, where we take database models and convert them into response
// models (DTO), while also organizing them in a map for quick access by a
// specific key. Then at the end, the parent will use this map to associate the
// child records efficiently.
func SliceToResponseMap[T any, R any, K comparable](items []*T, extractor func(*T) (K, R)) map[K]R {
	result := make(map[K]R, len(items))
	for _, item := range items {
		key, value := extractor(item)
		result[key] = value
	}
	return result
}

// SliceToGroupMap is the one-to-many counterpart of SliceToIndexMap. It groups
// a slice of pointers by the key returned from keyExtractor, usually the
// parent's foreign key (e.g. widget_id on widget_parts). The input order is
// preserved inside each group, so ORDER BY of the child query carries over.
// Looking up a key that had no items returns a nil slice.
func SliceToGroupMap[T any, K comparable](items []*T, keyExtractor func(*T) K) map[K][]*T {
	result := make(map[K][]*T)
	for _, item := range items {
		key := keyExtractor(item)
		result[key] = append(result[key], item)
	}
	return result
}

// SliceToGroupResponseMap is the one-to-many counterpart of SliceToResponseMap.
// It converts every item into a response model (DTO) and groups the results by
// the returned key, usually the parent's internal ID. The parent then takes its
// children straight from the map. The input order is preserved inside each
// group. Looking up a key that had no items returns a nil slice.
func SliceToGroupResponseMap[T any, R any, K comparable](items []*T, extractor func(*T) (K, R)) map[K][]R {
	result := make(map[K][]R)
	for _, item := range items {
		key, value := extractor(item)
		result[key] = append(result[key], value)
	}
	return result
}
