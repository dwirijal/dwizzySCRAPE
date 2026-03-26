package snapshot

import (
	"fmt"
	"os"
	"time"
)

func osEnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir snapshot output root: %w", err)
	}
	return nil
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

func writerTimeNow(_ Context) time.Time {
	return timeNowUTC()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
