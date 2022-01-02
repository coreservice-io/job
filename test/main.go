package main

import (
	"log"
	"time"

	"github.com/universe-30/UJob"
)

func div(a, b int) int {
	return a / b
}

func main() {
	count := 0
	// start a loop job
	job := UJob.Start(
		// job process
		func() {
			count++
			log.Println(count)

			//example, panic here
			//if count == 6 {
			//	div(3, 0)
			//}
		},
		// onPanic callback, run if panic happened
		func(err interface{}) {
			log.Println("panic catch")
			log.Println(err)
		},
		// job interval
		2,
		// job type
		// UJob.TYPE_PANIC_REDO  auto restart if panic
		// UJob.TYPE_PANIC_RETURN  stop if panic
		UJob.TYPE_PANIC_REDO,
		// check continue callback, the job will stop running if return false
		// the job will keep running if this callback is nil
		func(job *UJob.Job) bool {

			return true
		},
		// onFinish callback
		func(inst *UJob.Job) {
			log.Println("finish", "cycle", inst.Cycles)
		},
	)
	_ = job

	// if you want to stop job, use job.Cancel()
	go func() {
		time.Sleep(10 * time.Second)
		job.Cancel()
	}()

	time.Sleep(1 * time.Hour)
}
