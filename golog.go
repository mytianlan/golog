package golog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	sys         = &Sys{}
	logDir      string
	logging     loggingT
	logLevelInt int
	slogAlarmCb map[string]func(info []byte)
)

var levelMap = map[string]int{
	DEBUG:     1,
	INFO:      2,
	NOTICE:    3,
	WARNING:   4,
	ERROR:     5,
	CRITICAL:  6,
	ALERT:     7,
	EMERGENCY: 8,
}

//每3s往磁盘刷新一次
const flushInterval = 3 * time.Second

//默认日志目录
const defaultDir = "/data/logs/"

//bufio最大长度
const bufferSize = 10 * 1024 * 1024

//默认日志前缀
const logPrefix = "slog."

//日志等级
const EMERGENCY = "emergency"
const ALERT = "alert"
const CRITICAL = "critical"
const ERROR = "error"
const WARNING = "warning"
const NOTICE = "notice"
const INFO = "info"
const DEBUG = "debug"

//slog核心结构体
type loggingT struct {
	*bufio.Writer
	file     *os.File
	fileDate string
}

//异常退出
func (c *loggingT) exit(err error) {
	fmt.Println("loggingT exit:", err)
	return
}

//定时从缓存刷新到磁盘
func (c *loggingT) flushDaemon() {
	for _ = range time.NewTicker(flushInterval).C {
		now := time.Now()
		if now.Format("20060102") != c.fileDate {
			c.file, _ = create(logDir, now)
			c.fileDate = now.Format("20060102")
			c.Writer = bufio.NewWriterSize(c.file, bufferSize)
		}
		c.Flush()
	}
}

//通用打印方法
func (c *loggingT) output(l *slog) {
	if levelMap[l.Level] < logLevelInt {
		return
	}
	b, _ := json.Marshal(l)
	b = append(b, 0x0a)
	c.file.Write(b)
	return
}

//日志消息体
type slog struct {
	Time  string                 `json:"time"`
	Level string                 `json:"level"`
	Msg   string                 `json:"msg"`
	Cate  string                 `json:"cate"`
	Sys   *Sys                   `json:"sys"`
	Meta  map[string]interface{} `json:"meta"`
	File  string                 `json:"file"`
}

//日志消息体-Sys
type Sys struct {
	Idc string `json:"idc"`
	IP  string `json:"IP"`
	Ver string `json:"ver"`
}

func (c *slog) alermInfo() []byte {
	return []byte(fmt.Sprintf("[%s-%s]-%s", c.Level, c.Cate, c.Msg))
}

func init() {
	//默认日志等级为ERROR
	logLevelInt = levelMap[ERROR]

	slogAlarmCb = make(map[string]func(info []byte))

	now := time.Now()
	logging.file, _ = create(logDir, now)
	logging.fileDate = now.Format("20060102")
	logging.Writer = bufio.NewWriterSize(logging.file, bufferSize)
	defer logging.Flush()

	go logging.flushDaemon()
}

//设置日志级别
func SetLogLevel(level string) {
	if v, ok := levelMap[level]; ok {
		logLevelInt = v
	}
	return
}

func SetAlarmCb(level string, f func([]byte)) {
	slogAlarmCb[level] = f
	return
}

//创建文件
func create(dir string, t time.Time) (f *os.File, err error) {
	if len(dir) == 0 {
		dir = defaultDir
	}
	filename := filepath.Join(dir, logPrefix+t.Format("2006-01-02"))

	f, err = os.OpenFile(filename, os.O_RDWR|os.O_APPEND, 0777)
	if err != nil && os.IsNotExist(err) {
		f, err = os.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("log: cannot create log: %v", err)
		}
	}
	return f, err
}

//初始化时设置系统参数
func SetInitInfo(idc string, IP string, ver string, dir string) {
	sys.Idc = idc
	sys.IP = IP
	sys.Ver = ver
	logDir = dir
}

func base(level string, cate string, msg string, meta map[string]interface{}) *slog {
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["cate"] = cate

	l := &slog{}
	l.Time = time.Now().Format("2006-01-02 15:04:05")
	l.Msg = msg
	l.Cate = cate
	l.Sys = sys
	l.Meta = meta
	l.Level = level

	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	l.File = fmt.Sprintf("%s:%d", short, line)
	if v, ok := slogAlarmCb[level]; ok {
		v(l.alermInfo())
	}
	return l
}

//这个写法很牛，暂时用不上
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

//事件记录
func Debug(cate string, msg string, meta map[string]interface{}) {
	l := base(DEBUG, cate, msg, meta)
	//l.Level = DEBUG
	logging.output(l)
}

//事件记录
func Info(cate string, msg string, meta map[string]interface{}) {
	l := base(INFO, cate, msg, meta)
	//l.Level = INFO
	logging.output(l)
}

//运行时出现的错误
func Error(cate string, msg string, meta map[string]interface{}) {
	l := base(ERROR, cate, msg, meta)
	//l.Level = ERROR
	logging.output(l)
}

//程序组件不可用或者出现非预期的异常
func Critical(cate string, msg string, meta map[string]interface{}) {
	l := base(CRITICAL, cate, msg, meta)
	//l.Level = CRITICAL
	logging.output(l)
}

//系统不可用
func Emergency(cate string, msg string, meta map[string]interface{}) {
	l := base(EMERGENCY, cate, msg, meta)
	//l.Level = EMERGENCY
	logging.output(l)
}
