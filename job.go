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

const PANIC_REDO_SECS = 60

type Job struct {
	//manual init data
	jobId        string
	jobName      string
	interval     int64
	targetCycles int64
	jobType      JobType

	processFn     func()
	chkContinueFn func() bool
	afCloseFn     func()

	createTime  int64
	lastRuntime int64
	//info        *fj.FastJson

	status RunType
	cycles int64

	runToken     chan struct{}
	returnSignal chan struct{}

	context interface{}

	jobMgr *JobManager
}

func newJob(jobId string, jobName string, interval int64, targetCycles int64, jobType JobType, processFn func(), chkContinueFn func() bool, afCloseFn func(), jm *JobManager) *Job {
	return &Job{
		jobId:        jobId,
		jobName:      jobName,
		interval:     interval,
		targetCycles: targetCycles,
		jobType:      jobType,

		processFn:     processFn,
		chkContinueFn: chkContinueFn,
		afCloseFn:     afCloseFn,

		createTime:  time.Now().Unix(),
		lastRuntime: 0,
		status:      STATUS_WAITING,
		cycles:      0,

		runToken:     make(chan struct{}),
		returnSignal: make(chan struct{}),

		jobMgr: jm,
	}
}

func (j *Job) run() {
	j.runToken <- struct{}{}
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

							j.jobMgr.recordPanicStack(j.jobName, ErrStr, string(debug.Stack()))
							//check redo
							if j.jobType == TYPE_PANIC_REDO {
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
					toSleepSecs := j.lastRuntime + j.interval - nowUnixTime
					if toSleepSecs > 0 {
						time.Sleep(time.Duration(toSleepSecs) * time.Second)
					}
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
							j.jobMgr.recordPanicStack(j.jobName, ErrStr, string(debug.Stack()))
						}
					}()
					j.afCloseFn()
				}
				j.jobMgr.AllJobs.Delete(j.jobId)
				return
			}
		}
	}()
}

func (j *Job) runOneCycle() {
	if j.status == STATUS_CLOSING {
		j.returnSignal <- struct{}{}
		return
	}
	if j.chkContinueFn != nil && !j.chkContinueFn() {
		//job finish
		j.returnSignal <- struct{}{}
		return
	}

	j.lastRuntime = time.Now().Unix()
	j.status = STATUS_RUNNING

	//run
	j.processFn()

	//this cycle finish
	j.status = STATUS_WAITING
	j.cycles++
	if j.targetCycles > 0 && j.cycles >= j.targetCycles {
		//job finish
		j.returnSignal <- struct{}{}
		return
	}
	if j.status == STATUS_CLOSING {
		j.returnSignal <- struct{}{}
		return
	}
	if j.chkContinueFn != nil && !j.chkContinueFn() {
		//job finish
		j.returnSignal <- struct{}{}
		return
	}

	//put runToken back
	j.runToken <- struct{}{}
}
