package rpc

func makeMapSet(data []string) map[string]bool {
	result := make(map[string]bool)

	for _, item := range data {
		result[item] = true
	}

	return result
}

func diff(left []string, right []string) bool {
	s1 := makeMapSet(left)
	s2 := makeMapSet(right)

	if len(s1) != len(s2) {
		return true
	}

	for k, v := range s1 {
		if s2[k] != v {
			return true
		}
	}

	return false
}
