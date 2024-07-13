package utils

import "strings"

// lil hack to get the date in correct format
func FormatDate(date string) string {
	temp := strings.Split(date, "T")
	return temp[0]
}
