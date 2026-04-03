package debug

import "fmt"

func benchmarkUsers() []UserRecord {
	users := make([]UserRecord, 0, 128)
	for i := 0; i < 128; i++ {
		users = append(users, UserRecord{
			ID:    1000 + i,
			Name:  fmt.Sprintf("user-%03d", i),
			Score: 60 + i%40,
		})
	}
	return users
}
