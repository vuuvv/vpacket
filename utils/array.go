package utils

// DifferenceBy returns the difference between two collections.
// The first value is the collection of element absent of list2, 第一个数组里有，第二个数组里没有.
// The second value is the collection of element absent of list1, 第二个数组里有，第一个数组里没有.
// 第三个数组是两个数组的交集.
func DifferenceBy[T any, R comparable](list1 []T, list2 []T, keyFn func(item T) R) ([]T, []T, []T) {
	var left []T
	var right []T
	var intersect []T

	seenLeft := map[R]struct{}{}
	seenRight := map[R]struct{}{}

	for _, elem := range list1 {
		key := keyFn(elem)
		seenLeft[key] = struct{}{}
	}

	for _, elem := range list2 {
		key := keyFn(elem)
		seenRight[key] = struct{}{}
	}

	for _, elem := range list1 {
		key := keyFn(elem)
		if _, ok := seenRight[key]; !ok {
			left = append(left, elem)
		}
	}

	for _, elem := range list2 {
		key := keyFn(elem)
		if _, ok := seenLeft[key]; !ok {
			right = append(right, elem)
		} else {
			intersect = append(intersect, elem)
		}
	}

	return left, right, intersect
}
