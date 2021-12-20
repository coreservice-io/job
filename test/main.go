package main

import (
	"github.com/universe-30/UJob"
	"log"
	"time"
)

func div(a, b int) int {
	return a / b
}

func main() {
	jm := UJob.New()

	//Redo job
	count := 0
	jm.StartJob("redo job", UJob.TYPE_PANIC_REDO, 0, 2, func() {

		count++
		log.Println("redo job run", count)
		if count == 3 {
			div(3, 0)
		}
	}, nil, nil)

	time.Sleep(1 * time.Hour)
}
