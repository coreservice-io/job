package job

import (
	"context"
	"errors"
	"time"
)

type JobType string

const (
	TYPE_PANIC_REDO   JobType = "panic_redo"
	TYPE_PANIC_RETURN JobType = "panic_return"
)

type Job struct {
	//manual init data
	Name     string
	Interval int64
	JobType  JobType

	//callback
	processFn        func(job *Job)
	chkBeforeStartFn func(job *Job) bool
	finalFn          func(job *Job)
	onPanic          func(job *Job, err interface{})

	//update data in running
	CreateTime    int64
	LastRuntime   int64
	Cycles        int64
	LastPanicTime int64
	PanicCount    int64

	job_ctx    context.Context //job context
	context    context.Context
	cancelFunc context.CancelFunc

	nextRound chan struct{}

	Data interface{}
}

// intervalSecs will be replaced with 1 if <=0
func Start(job_ctx_ context.Context, name string, jobType JobType, intervalSecs int64, data interface{}, chkBeforeStartFn func(*Job) bool, processFn func(*Job), onPanic func(job *Job, panic_err interface{}), finalFn func(*Job)) error {

	if processFn == nil {
		return errors.New("processFn nil error")
	}

	//min interval is 1 second
	if intervalSecs <= 0 {
		return errors.New("intervalSecs should >= 1")
	}

	ctx, cancel_func := context.WithCancel(context.Background())

	j := &Job{
		Name:             name,
		Interval:         intervalSecs,
		JobType:          jobType,
		chkBeforeStartFn: chkBeforeStartFn,
		processFn:        processFn,
		finalFn:          finalFn,
		onPanic:          onPanic,
		CreateTime:       time.Now().Unix(),
		LastRuntime:      0,
		Cycles:           0,
		job_ctx:          job_ctx_,
		context:          ctx,
		cancelFunc:       cancel_func,
		nextRound:        make(chan struct{}),
		Data:             data,
	}

	go func() {
		for {
			select {

			case <-j.job_ctx.Done():
				j.cancelFunc()
				continue

			case <-j.context.Done():
				if finalFn != nil {
					finalFn(j)
				}
				return

			case <-j.nextRound:
				go func() {
					// if panic happen
					defer func() {
						if err := recover(); err != nil {
							j.addOneCycle() //compensate the err
							j.LastPanicTime = time.Now().Unix()
							j.PanicCount++
							if onPanic != nil {
								onPanic(j, err)
							}
							//check redo
							if j.JobType == TYPE_PANIC_REDO {
								j.nextRound <- struct{}{}
							} else {
								j.cancelFunc()
							}
						}
					}()
					//////////////////
					//one cycle process
					if j.chkBeforeStartFn != nil && !j.chkBeforeStartFn(j) {
						j.cancelFunc()
						return
					} else {
						//do the job
						j.LastRuntime = time.Now().Unix()
						j.processFn(j)
						j.addOneCycle()
						//check next run time
						nowUnixTime := time.Now().Unix()
						toSleepSecs := j.LastRuntime + j.Interval - nowUnixTime
						if toSleepSecs > 0 {
							time.Sleep(time.Duration(toSleepSecs) * time.Second)
						}
						//one more cycle
						j.nextRound <- struct{}{}
					}
				}()
			}
		}
	}()

	j.nextRound <- struct{}{}
	return nil
}

func (j *Job) addOneCycle() {
	j.Cycles++
	if j.Cycles < 0 {
		j.Cycles = 0
	}
}
