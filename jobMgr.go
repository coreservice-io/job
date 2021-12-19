package UJob

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func randJobId() string {
	b := make([]rune, 8)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type JobManager struct {
	AllJobs    sync.Map
	PanicExist bool
	PanicJson  *fj.FastJson

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

func (jm *JobManager) GetGBJob(jobid string) *Job {
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
		jobh_.(*Job).status = STATUS_CLOSING
	}
}

func (jm *JobManager) CloseAndDeleteAllJobs() {
	jm.AllJobs.Range(func(_, value interface{}) bool {
		value.(*Job).status = STATUS_CLOSING
		return true
	})
}

func (jm *JobManager) RunOnceJob() {

}

func (jm *JobManager) ScheduleJob() {

}

func (jm *JobManager) startJob(jobName string, jobType JobType, targetCycles int64, interval int64, processFn func(), chkContinueFn func() bool, afCloseFn func()) (jobId string, err error) {
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

	errors := []string{panicstr}
	errstr := panicstr

	errors = append(errors, "last err unix-time:"+strconv.FormatInt(time.Now().Unix(), 10))

	lines := strings.Split(stack, "\n")
	maxlines := len(lines)
	if maxlines >= 100 {
		maxlines = 100
	}

	if maxlines >= 3 {
		for i := 2; i < maxlines; i = i + 2 {
			fomatstr := strings.ReplaceAll(lines[i], "	", "")
			errstr = errstr + "#" + fomatstr
			errors = append(errors, fomatstr)
		}
	}

	h := md5.New()
	h.Write([]byte(errstr))
	errhash := hex.EncodeToString(h.Sum(nil))

	jm.PanicExist = true
	//jm.PanicJson.SetStringArray(errors, "errors", jb.jobName, errhash)

	if jm.logger != nil {
		jm.logger.Error("bgjob-catch-panic: ", " jobname:", jobName, " errhash:", errhash, " errors:", errors)
	}

}
