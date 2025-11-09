package util

import (
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

func InitLogger() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.DateTime,
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			return frame.Function, "" // TODO: 生成包的简写
		},
	})
	logrus.SetReportCaller(true)
}
