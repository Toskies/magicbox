package debug

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type UserRecord struct {
	ID    int
	Name  string
	Score int
}

func ParseUserRecord(line string) (UserRecord, error) {
	parts := strings.Split(line, ",")
	if len(parts) != 3 {
		return UserRecord{}, fmt.Errorf("expected 3 fields, got %d", len(parts))
	}

	idText := strings.TrimSpace(parts[0])
	id, err := strconv.Atoi(idText)
	if err != nil {
		return UserRecord{}, fmt.Errorf("invalid id %q: %w", idText, err)
	}

	name := strings.TrimSpace(parts[1])
	if name == "" {
		return UserRecord{}, fmt.Errorf("name is empty")
	}

	scoreText := strings.TrimSpace(parts[2])
	score, err := strconv.Atoi(scoreText)
	if err != nil {
		return UserRecord{}, fmt.Errorf("invalid score %q: %w", scoreText, err)
	}

	return UserRecord{
		ID:    id,
		Name:  name,
		Score: score,
	}, nil
}

func BuildSummarySlow(users []UserRecord) string {
	summary := ""
	for _, user := range users {
		summary += fmt.Sprintf("id=%d name=%s score=%d\n", user.ID, user.Name, user.Score)
	}
	return summary
}

func BuildSummaryFast(users []UserRecord) string {
	var builder strings.Builder
	builder.Grow(len(users) * 32)

	for _, user := range users {
		builder.WriteString("id=")
		builder.WriteString(strconv.Itoa(user.ID))
		builder.WriteString(" name=")
		builder.WriteString(user.Name)
		builder.WriteString(" score=")
		builder.WriteString(strconv.Itoa(user.Score))
		builder.WriteByte('\n')
	}

	return builder.String()
}

type UnsafeCounter struct {
	value int
}

func NewUnsafeCounter() *UnsafeCounter {
	return &UnsafeCounter{}
}

func (c *UnsafeCounter) Inc() {
	c.value++
}

func (c *UnsafeCounter) Value() int {
	return c.value
}

type LockedCounter struct {
	mu    sync.Mutex
	value int
}

func NewLockedCounter() *LockedCounter {
	return &LockedCounter{}
}

func (c *LockedCounter) Inc() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

func (c *LockedCounter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}
