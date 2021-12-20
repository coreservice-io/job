package UJob

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type PanicInfo struct {
	ErrHash  string
	JobName  string
	ErrorStr []string
}

type JobManager struct {
	AllJobs sync.Map

	PanicMap sync.Map

	logger *logrus.Logger
}

func New() *JobManager {
	jm := &JobManager{}

	//loggerr
	jm.logger = logrus.New()
	jm.logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	jm.logger.SetLevel(logrus.InfoLevel)

	jm.printPanic()

	return jm
}

func (jm *JobManager) SetLogLevel(level logrus.Level) {
	jm.logger.SetLevel(level)
}

func (jm *JobManager) SetOutput(output io.Writer) {
	jm.logger.SetOutput(output)
}

func (jm *JobManager) SetLogger(logger *logrus.Logger) {
	jm.logger = logger
}

func (jm *JobManager) LogTest() {
	jm.logger.Debugln("log debug")
	jm.logger.Infoln("log info")
	jm.logger.Warnln("log warn")
	jm.logger.Errorln("log error")

	jm.logger.Debugln("log debug")
	jm.logger.Infoln("log info")
	jm.logger.Warnln("log warn")
	jm.logger.Errorln("log error")
}

func (jm *JobManager) GetJob(jobid string) *Job {
	jobh_, ok := jm.AllJobs.Load(jobid)
	if ok {
		return jobh_.(*Job)
	} else {
		return nil
	}
}

func (jm *JobManager) CloseAndDeleteJob(jobid string) {
	jobh_, ok := jm.AllJobs.Load(jobid)
	if ok {
		jobh_.(*Job).Status = STATUS_CLOSING
	}
}

func (jm *JobManager) CloseAndDeleteAllJobs() {
	jm.AllJobs.Range(func(_, value interface{}) bool {
		value.(*Job).Status = STATUS_CLOSING
		return true
	})
}

//StartJob_Panic_Redo Start a background job which run periodically at given intervals.
//It will restart if panic happen.
//Job will run forever if cycleLimit is 0.
func (jm *JobManager) StartJob_Panic_Redo(jobName string, cycleLimit int64, interval int64, processFn func(), chkContinueFn func(*Job) bool, afCloseFn func(*Job)) (jobId string, err error) {
	return jm.startJob(jobName, TYPE_PANIC_REDO, cycleLimit, interval, processFn, chkContinueFn, afCloseFn)
}

//StartJob_Panic_Return Start a background job which run periodically at given intervals.
//It will stop if panic happen.
func (jm *JobManager) StartJob_Panic_Return(jobName string, cycleLimit int64, interval int64, processFn func(), chkContinueFn func(*Job) bool, afCloseFn func(*Job)) (jobId string, err error) {
	return jm.startJob(jobName, TYPE_PANIC_RETURN, cycleLimit, interval, processFn, chkContinueFn, afCloseFn)
}

func (jm *JobManager) startJob(jobName string, jobType JobType, targetCycles int64, interval int64, processFn func(), chkContinueFn func(*Job) bool, afCloseFn func(*Job)) (jobId string, err error) {
	if jobType != TYPE_PANIC_REDO && jobType != TYPE_PANIC_RETURN {
		return "", errors.New("job type error")
	}

	if interval < 1 {
		return "", errors.New("interval at least 1 second")
	}

	//generate a random job id that not exist yet
	for {
		jobId = randJobId()
		_, ok := jm.AllJobs.Load(jobId)
		if !ok {
			break
		}
	}

	job := newJob(jobId, jobName, targetCycles, interval, jobType, processFn, chkContinueFn, afCloseFn, jm)

	jm.AllJobs.Store(jobId, job)

	job.run()

	return
}

func (jm *JobManager) recordPanicStack(jobName string, panicstr string, stack string) {

	errorsInfo := []string{panicstr}
	errstr := panicstr

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

	panicInfo := &PanicInfo{
		ErrHash:  errhash,
		JobName:  jobName,
		ErrorStr: errorsInfo,
	}
	jm.PanicMap.Store(errhash, panicInfo)

	//if jm.logger != nil {
	//	jm.logger.Error("bgjob-catch-panic: ", " jobname:", jobName, " errhash:", errhash, " errors:", errorsInfo)
	//}
}

func (jm *JobManager) printPanic() {
	_, err := jm.StartJob_Panic_Redo("UJob PrintPanic", 0, 300, func() {
		jm.PanicMap.Range(func(key, value interface{}) bool {
			panicInfo, ok := value.(*PanicInfo)
			if !ok {
				return true
			}

			jm.logger.Error("bgjob-catch-panic:", " jobname:", panicInfo.JobName, " errors:", panicInfo.ErrorStr)

			jm.PanicMap.Delete(key)
			return true
		})
	}, nil, nil)

	jm.logger.Fatalln("UJob printPanic error:", err)
}
