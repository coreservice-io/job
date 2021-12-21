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
	count := 0
	job := UJob.StartLoopJob(
		func() {
			count++
			log.Println(count)
			if count%6 == 0 {
				div(3, 0)
			}
		},
		func(panicInfo *UJob.PanicInfoInst) {
			log.Println("panic catch")
			log.Println(panicInfo.ErrHash)
			for _, v := range panicInfo.ErrorStr {
				log.Println(v)
			}
		},
		2, UJob.TYPE_PANIC_REDO, nil, func(inst *UJob.Job) {
			log.Println("finish", "cycle", inst.Cycles)
		},
	)

	_ = job

	//go func() {
	//	time.Sleep(10 * time.Second)
	//	job.Stop()
	//}()

	time.Sleep(1 * time.Hour)
}
