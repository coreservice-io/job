# job

### job will be auto restared using Panic_Redo type
### !!important : Don't write your own go-routine inside job function

### example
```go
package main

import (
	"log"
	"time"

	"github.com/coreservice-io/job"
)

// func div(a, b int) int {
// 	return a / b
// }

func main() {

	my_data_counter := 0
	// start a loop job
	job_, err := job.Start(
		"job name",
		// job type
		// job.TYPE_PANIC_REDO  auto restart if panic
		// job.TYPE_PANIC_RETURN  stop if panic
		job.TYPE_PANIC_REDO,
		// job interval in seconds
		1,
		//define you data here ,can be anything
		&my_data_counter,
		// check before proces fn , the job will stop running if return false
		// the job will bypass if function is nil
		func(job *job.Job) bool {
			return true
		},
		// job process
		func(j *job.Job) {

			log.Println("count", *j.Data.(*int))
			log.Println("cycle", j.Cycles)
			*j.Data.(*int)++
			//trigger example panic here
			// if j.Cycles == 6 {
			// 	div(0, 0)
			// }
		},
		// onPanic callback, run if panic happened
		func(j *job.Job, err interface{}) {
			log.Println("panic catch", err)
			time.Sleep(5 * time.Second)
		},
		// onFinal callback
		func(j *job.Job) {
			log.Println("finish", "cycle", j.Cycles)
		},
	)

	if err != nil {
		panic(err)
	}

	// if you want to stop job, use job.SetToCancel()
	// after the job finish the current loop it will quit and call the finalFn function
	go func() {
		time.Sleep(20 * time.Second)
		job_.SetToCancel()
	}()

	time.Sleep(1 * time.Hour)
}

```