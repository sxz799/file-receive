package utils

import "fmt"

func FormatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB"}
	i := 0
	for ; bytes >= k && i < len(sizes)-1; i++ {
		bytes /= k
	}
	return fmt.Sprintf("%d %s", bytes, sizes[i])
}
