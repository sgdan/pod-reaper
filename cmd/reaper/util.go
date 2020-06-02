package main

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
