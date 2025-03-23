package file

import (
	"errors"
	"fmt"
	"os"
	"time"
)

func generateUniqueFileNameForSnapshot(format string) (string, error) {
	// Get the current date and time
	currentTime := time.Now()

	// Format the current date in YYYYMMDD format
	currentDate := currentTime.Format("20060102")

	slug := fmt.Sprintf("meshery-cluster-snapshot-%s", currentDate)

	name := ""
	gotTheName := false
	for i := 0; i < 1024; i++ {
		name = fmt.Sprintf("%s-%02d.%s", slug, i, format)
		// Use os.Stat to check if the file exists
		_, err := os.Stat(name)
		if os.IsNotExist(err) {
			gotTheName = true
			break
		}
	}

	if !gotTheName {
		return "", errors.New("no unique name available")
	}
	return name, nil
}
