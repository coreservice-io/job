package UJob

import (
	"runtime"
	"runtime/debug"
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
	STATUS_CLOSING RunType = "closing"
)

const PANIC_REDO_SECS = 30

type Job struct {
	//manual init data
	JobId        string
	JobName      string
	Interval     int64
	TargetCycles int64
	JobType      JobType

	//callback
	processFn     func()
	chkContinueFn func(job *Job) bool
	afCloseFn     func(job *Job)

	//update data in running
	CreateTime  int64
	LastRuntime int64
	//info        *fj.FastJson
	Status RunType
	Cycles int64

	//signal channel
	runToken     chan struct{}
	returnSignal chan struct{}

	//reference
	jobMgr *JobManager
}

func newJob(jobId string, jobName string, targetCycles int64, interval int64, jobType JobType, processFn func(), chkContinueFn func(*Job) bool, afCloseFn func(*Job), jm *JobManager) *Job {
	return &Job{
		JobId:        jobId,
		JobName:      jobName,
		Interval:     interval,
		TargetCycles: targetCycles,
		JobType:      jobType,

		processFn:     processFn,
		chkContinueFn: chkContinueFn,
		afCloseFn:     afCloseFn,

		CreateTime:  time.Now().Unix(),
		LastRuntime: 0,
		Status:      STATUS_WAITING,
		Cycles:      0,

		runToken:     make(chan struct{}),
		returnSignal: make(chan struct{}),

		jobMgr: jm,
	}
}

func (j *Job) run() {
	go func() {
		for {
			select {
			case <-j.runToken:
				go func() {
					// if panic happen
					defer func() {
						if err := recover(); err != nil {
							//record panic
							var ErrStr string
							switch e := err.(type) {
							case string:
								ErrStr = e
							case runtime.Error:
								ErrStr = e.Error()
							case error:
								ErrStr = e.Error()
							default:
								ErrStr = "recovered (default) panic"
							}

							j.jobMgr.recordPanicStack(j.JobName, ErrStr, string(debug.Stack()))
							//check redo
							if j.JobType == TYPE_PANIC_REDO {
								time.Sleep(PANIC_REDO_SECS * time.Second)
								j.runToken <- struct{}{}
							} else {
								j.returnSignal <- struct{}{}
							}
						}
					}()

					//do job
					j.runOneCycle()

					//check next run time
					nowUnixTime := time.Now().Unix()
					toSleepSecs := j.LastRuntime + j.Interval - nowUnixTime
					if toSleepSecs > 0 {
						time.Sleep(time.Duration(toSleepSecs) * time.Second)
					}
					//put runToken back
					j.runToken <- struct{}{}
				}()
			case <-j.returnSignal:
				if j.afCloseFn != nil {
					defer func() {
						if err := recover(); err != nil {
							//record panic
							var ErrStr string
							switch e := err.(type) {
							case string:
								ErrStr = e
							case runtime.Error:
								ErrStr = e.Error()
							case error:
								ErrStr = e.Error()
							default:
								ErrStr = "recovered (default) panic"
							}
							j.jobMgr.recordPanicStack(j.JobName, ErrStr, string(debug.Stack()))
						}
					}()
					j.afCloseFn(j)
				}
				j.jobMgr.AllJobs.Delete(j.JobId)
				return
			}
		}
	}()
	j.runToken <- struct{}{}
}

func (j *Job) runOneCycle() {
	if j.Status == STATUS_CLOSING {
		j.returnSignal <- struct{}{}
		return
	}
	if j.chkContinueFn != nil && !j.chkContinueFn(j) {
		//job finish
		j.returnSignal <- struct{}{}
		return
	}

	j.LastRuntime = time.Now().Unix()
	j.Status = STATUS_RUNNING

	//run
	j.processFn()

	//this cycle finish
	j.Status = STATUS_WAITING
	j.Cycles++
	if j.TargetCycles > 0 && j.Cycles >= j.TargetCycles {
		//job finish
		j.returnSignal <- struct{}{}
		return
	}
	if j.Status == STATUS_CLOSING {
		j.returnSignal <- struct{}{}
		return
	}
	if j.chkContinueFn != nil && !j.chkContinueFn(j) {
		//job finish
		j.returnSignal <- struct{}{}
		return
	}
}
