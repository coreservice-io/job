package UJob

import (
	"time"
)

type JobType string

const (
	TYPE_PANIC_REDO   JobType = "panic_redo"
	TYPE_PANIC_RETURN JobType = "panic_return"
)

type RunType string

const (
	STATUS_RUNNING RunType = "running"
	STATUS_WAITING RunType = "waiting"
	STATUS_FAILED  RunType = "failed"
	STATUS_DONE    RunType = "done"
)

const PANIC_REDO_SECS = 30

type Job struct {
	//manual init data
	Interval int64
	JobType  JobType

	//callback
	processFn     func()
	chkContinueFn func(job *Job) bool
	afCloseFn     func(job *Job)
	onPanic       func(err interface{})

	//update data in running
	CreateTime  int64
	LastRuntime int64
	Status      RunType
	Cycles      uint64

	ToCancel bool

	//signal channel
	runChannel    chan struct{}
	returnChannel chan struct{}
}

func Start(processFn func(), onPanic func(panic_err interface{}), intervalSecs int64, jobType JobType, chkContinueFn func(*Job) bool, finalFn func(*Job)) *Job {
	j := &Job{
		Interval:      intervalSecs,
		JobType:       jobType,
		processFn:     processFn,
		chkContinueFn: chkContinueFn,
		afCloseFn:     finalFn,
		onPanic:       onPanic,
		CreateTime:    time.Now().Unix(),
		LastRuntime:   0,
		Status:        STATUS_WAITING,
		ToCancel:      false,
		runChannel:    make(chan struct{}),
		returnChannel: make(chan struct{}),
	}

	go func() {

		for {
			select {
			case <-j.returnChannel:
				if finalFn != nil {
					finalFn(j)
				}
				return

			case <-j.runChannel:
				go func() {
					// if panic happen
					defer func() {
						if err := recover(); err != nil {
							if onPanic != nil {
								onPanic(err)
							}
							//check redo
							if j.JobType == TYPE_PANIC_REDO {
								j.Status = STATUS_WAITING
								time.Sleep(PANIC_REDO_SECS * time.Second)
								j.runChannel <- struct{}{}
							} else {
								j.Status = STATUS_FAILED
								j.returnChannel <- struct{}{}
							}
						}
					}()

					//one cycle process
					if j.chkContinueFn != nil && !j.chkContinueFn(j) {
						j.Status = STATUS_DONE
						j.returnChannel <- struct{}{}
						return
					} else {
						//do the job
						j.Status = STATUS_RUNNING
						j.LastRuntime = time.Now().Unix()
						j.processFn()
						j.Status = STATUS_WAITING
						j.Cycles++

						//check continue again ! important!
						if j.chkContinueFn != nil && !j.chkContinueFn(j) {
							j.Status = STATUS_DONE
							j.returnChannel <- struct{}{}
							return
						}

						//check next run time
						nowUnixTime := time.Now().Unix()
						toSleepSecs := j.LastRuntime + j.Interval - nowUnixTime
						if toSleepSecs > 0 {
							time.Sleep(time.Duration(toSleepSecs) * time.Second)
						}
						//one more cycle
						j.runChannel <- struct{}{}
					}

				}()
			}
		}

	}()

	j.runChannel <- struct{}{}
	return j
}

//when SetToCancel() is called the job will not continue after finishing current round
func (j *Job) SetToCancel() {
	j.ToCancel = true
}
