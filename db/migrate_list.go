package db

func List() []string {
	result := make([]string, len(migrations))
	i := 0
	for k, _ := range migrations {
		result[i] = k
	}
	return result
}
