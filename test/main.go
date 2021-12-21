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
	jm.StartJob_Panic_Redo("redo job", 12, 1, func() {

		count++
		log.Println("redo job run", count)
		if count%10 == 0 {
			div(3, 0)
		}

		if count == 66 {
			panic("some panic")
		}
	}, nil, nil)

	time.Sleep(1 * time.Hour)
}
