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

type JobConfig struct {
	Name                string
	Job_type            JobType
	Interval_secs       int64
	Chk_before_start_fn func(*Job) bool
	Process_fn          func(*Job)
	On_panic            func(job *Job, panic_err interface{})
	Panic_sleep_secs    int64 //how long to sleep before next panic redo
	Final_fn            func(*Job)
}

const default_panic_redo_sleep_secs = 30 //sleep 30 secs  before next panic redo

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

	panic_redo_delay_secs int64

	job_ctx    context.Context //job context
	context    context.Context
	cancelFunc context.CancelFunc

	nextRound chan struct{}

	Data interface{}
}

// intervalSecs will be replaced with 1 if <=0
func Start(job_ctx_ context.Context, job_conf JobConfig, data interface{}) error {

	if job_conf.Process_fn == nil {
		return errors.New("processFn nil error")
	}

	//min interval is 1 second
	if job_conf.Interval_secs <= 0 {
		return errors.New("intervalSecs should >= 1")
	}

	if job_conf.Panic_sleep_secs <= 0 {
		job_conf.Panic_sleep_secs = default_panic_redo_sleep_secs
	}

	ctx, cancel_func := context.WithCancel(context.Background())

	j := &Job{
		Name:             job_conf.Name,
		Interval:         job_conf.Interval_secs,
		JobType:          job_conf.Job_type,
		chkBeforeStartFn: job_conf.Chk_before_start_fn,
		processFn:        job_conf.Process_fn,
		finalFn:          job_conf.Final_fn,
		onPanic:          job_conf.On_panic,
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

			case <-j.context.Done():
				if j.finalFn != nil {
					j.finalFn(j)
				}
				return

			case <-j.job_ctx.Done():
				j.cancelFunc()
				continue

			case <-j.nextRound:
				go func() {
					// if panic happen
					defer func() {
						if err := recover(); err != nil {
							j.addOneCycle() //compensate the err
							j.LastPanicTime = time.Now().Unix()
							j.PanicCount++
							if j.onPanic != nil {
								j.onPanic(j, err)
							}
							//check redo
							if j.JobType == TYPE_PANIC_REDO {
								select {
								case <-j.job_ctx.Done(): //context cancelled
								case <-time.After(time.Duration(j.panic_redo_delay_secs) * time.Second): //timeout
								}
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
							select {
							case <-j.job_ctx.Done(): //context cancelled
							case <-time.After(time.Duration(toSleepSecs) * time.Second): //timeout
							}
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
