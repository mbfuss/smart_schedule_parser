// Package main запускает приложение Smart Schedule Parser,
// инициализирует логирование, контейнер зависимостей и HTTP-сервер.
package main

import (
	"io"
	"os"

	"smart_schedule_parser/internal/di"
	"smart_schedule_parser/internal/server"

	zerolog "github.com/rs/zerolog/log"
)

const (
	logDir      = "./log"
	logFilePath = logDir + "/app.log"
)

func main() {
	logFile := loggerInit()
	defer func(logFile *os.File) {
		err := logFile.Close()
		if err != nil {
			panic(err)
		}
	}(logFile)

	// Создание контейнер зависимостей
	di, err := di.NewContainer()
	if err != nil {
		panic(err)
	}

	// Запуск HTTP сервера
	err = server.StartServer(di.Mux, di.Config)
	if err != nil {
		panic(err)
	}
}

func loggerInit() *os.File {
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			panic("Не удалось создать директорию для логов: " + err.Error())
		}
	}

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	// MultiWriter для одновременного вывода в файл и консоль
	multi := io.MultiWriter(os.Stdout, logFile)
	zerolog.Logger = zerolog.Output(multi)

	return logFile
}
