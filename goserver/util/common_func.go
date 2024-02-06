package util

func VectorFind[T any](vec []T, f func(*T) bool) int {
	for i, v := range vec {
		if f(&v) {
			return i
		}
	}
	return -1
}

func VectorRemoveNoSort[T any](vec []T, f func(*T) bool) []T {
	len := len(vec)
	for i := len - 1; i >= 0; i-- {
		if f(&vec[i]) {
			vec[i], vec[len-1] = vec[len-1], vec[i]
			len -= 1
		}
	}
	if len == 0 {
		return []T{}
	}
	return vec[:len]
}
