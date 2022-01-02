package UJob

import (
	"context"
	"sync"
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

	ctx    context.Context
	cancel context.CancelFunc

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

	//signal channel
	runChannel  chan struct{}
	stopChannel chan struct{}
	//stop flag
	stopSignal     bool
	stopSignalLock sync.RWMutex
}

func Start(processFn func(), onPanic func(err interface{}), interval int64, jobType JobType, chkContinueFn func(*Job) bool, afCloseFn func(*Job)) *Job {
	j := &Job{
		Interval:      interval,
		JobType:       jobType,
		processFn:     processFn,
		chkContinueFn: chkContinueFn,
		afCloseFn:     afCloseFn,
		onPanic:       onPanic,
		CreateTime:    time.Now().Unix(),
		LastRuntime:   0,
		Status:        STATUS_WAITING,
		runChannel:    make(chan struct{}),
		stopChannel:   make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-j.stopChannel:
				if afCloseFn != nil {
					afCloseFn(j)
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
								j.stopChannel <- struct{}{}
							}
						}
					}()

					//check stop flag
					if j.getStopSignal() {
						j.Status = STATUS_DONE
						j.stopChannel <- struct{}{}
						return
					}

					//do job
					isGoOn := j.runOneCycle()
					if !isGoOn {
						j.stopChannel <- struct{}{}
						return
					}

					//check next run time
					nowUnixTime := time.Now().Unix()
					toSleepSecs := j.LastRuntime + j.Interval - nowUnixTime
					if toSleepSecs > 0 {
						time.Sleep(time.Duration(toSleepSecs) * time.Second)
					}
					//put runChannel back
					j.runChannel <- struct{}{}
				}()
			}
		}
	}()
	j.runChannel <- struct{}{}
	return j
}

func (j *Job) Cancel() {
	j.stopSignalLock.Lock()
	defer j.stopSignalLock.Unlock()
	j.stopSignal = true
}

func (j *Job) getStopSignal() bool {
	j.stopSignalLock.RLock()
	defer j.stopSignalLock.RUnlock()
	return j.stopSignal
}

//runOneCycle the job will stop if return false
func (j *Job) runOneCycle() bool {
	if j.chkContinueFn != nil && !j.chkContinueFn(j) {
		j.Status = STATUS_DONE
		return false
	}

	j.LastRuntime = time.Now().Unix()
	j.Status = STATUS_RUNNING

	//run
	j.processFn()

	//this cycle finish
	j.Status = STATUS_WAITING
	j.Cycles++
	return true
}
