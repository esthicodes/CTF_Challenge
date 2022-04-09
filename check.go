package check

import "log"

func Fatal(message string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}

func Print(message string, err error) {
	if err != nil {
		log.Printf("%s: %v", message, err)
	}
}
