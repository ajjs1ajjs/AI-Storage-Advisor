package utils

import "fmt"

func FormatSize(sizeBytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	val := float64(sizeBytes)
	for _, unit := range units {
		if val < 1024.0 {
			return fmt.Sprintf("%.2f %s", val, unit)
		}
		val /= 1024.0
	}
	return fmt.Sprintf("%.2f PB", val)
}
