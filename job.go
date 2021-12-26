package UJob

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
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
	onPanic       func(panicInfo *PanicInfoInst)

	//update data in running
	CreateTime  int64
	LastRuntime int64
	Status      RunType
	Cycles      uint64

	//signal channel
	runChannel chan struct{}
}

type PanicInfoInst struct {
	ErrHash  string
	ErrorStr []string
}

func Start(processFn func(), onPanic func(panicInfo *PanicInfoInst), interval int64, jobType JobType, chkContinueFn func(*Job) bool, afCloseFn func(*Job)) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{
		Interval:      interval,
		JobType:       jobType,
		ctx:           ctx,
		cancel:        cancel,
		processFn:     processFn,
		chkContinueFn: chkContinueFn,
		afCloseFn:     afCloseFn,
		onPanic:       onPanic,
		CreateTime:    time.Now().Unix(),
		LastRuntime:   0,
		Status:        STATUS_WAITING,
		runChannel:    make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				if afCloseFn != nil {
					afCloseFn(j)
				}
				return // returning not to leak the goroutine
			case <-j.runChannel:
				go func() {
					// if panic happen
					defer func() {
						if err := recover(); err != nil {

							if onPanic != nil {
								onPanic(handlePanicStack(err))
							}
							//check redo
							if j.JobType == TYPE_PANIC_REDO {
								j.Status = STATUS_WAITING
								time.Sleep(PANIC_REDO_SECS * time.Second)
								j.runChannel <- struct{}{}
							} else {
								j.Status = STATUS_FAILED
								cancel()
							}
						}
					}()

					//do job
					isGoOn := j.runOneCycle()
					if !isGoOn {
						cancel()
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
	if j.cancel != nil {
		j.cancel()
	}
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

func handlePanicStack(err interface{}) *PanicInfoInst {
	//record panic
	var panicStr string
	switch e := err.(type) {
	case string:
		panicStr = e
	case runtime.Error:
		panicStr = e.Error()
	case error:
		panicStr = e.Error()
	default:
		panicStr = "recovered (default) panic"
	}

	stack := string(debug.Stack())

	errorsInfo := []string{panicStr}
	errstr := panicStr

	errorsInfo = append(errorsInfo, "last err unix-time:"+strconv.FormatInt(time.Now().Unix(), 10))

	lines := strings.Split(stack, "\n")
	maxlines := len(lines)
	if maxlines >= 100 {
		maxlines = 100
	}

	if maxlines >= 3 {
		for i := 2; i < maxlines; i = i + 2 {
			fomatstr := strings.ReplaceAll(lines[i], "	", "")
			errstr = errstr + "#" + fomatstr
			errorsInfo = append(errorsInfo, fomatstr)
		}
	}

	h := md5.New()
	h.Write([]byte(errstr))
	errhash := hex.EncodeToString(h.Sum(nil))

	panicInfo := &PanicInfoInst{
		ErrHash:  errhash,
		ErrorStr: errorsInfo,
	}

	return panicInfo
}
