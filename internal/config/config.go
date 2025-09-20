// Package config предоставляет загрузку и доступ к конфигурации приложения.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gookit/ini/v2/dotenv"
)

// Config - конфигурация приложения.
type Config struct {
	// Host - адрес, на котором будет запущен сервер.
	Host string
	// Port - порт, на котором будет запущен сервер.
	Port string
	// OutputDir - директория для сохранения файлов.
	OutputDir string
}

var cfg *Config // Конфигурация приложения. Должна быть проинициализирована при старте сервера.

const (
	// Путь к файлу конфигурации.
	envFilePath = ".env"
	// Пустая строка для валидации параметров конфигурации.
	undefinedStringValue string = ""
)

// ErrLoadEnv - ошибка загрузки конфигурации приложения в окружение.
var ErrLoadEnv = errors.New("загрузка конфигурации приложения в окружение: %w")

// Load - возвращает структуру со значениями из конфиг файла.
func Load() error {
	err := dotenv.Load(".", envFilePath)
	if err != nil {
		return fmt.Errorf(ErrLoadEnv.Error(), err)
	}

	cfg = &Config{
		Host:      dotenv.Get("HOST", undefinedStringValue),
		Port:      dotenv.Get("PORT", undefinedStringValue),
		OutputDir: dotenv.Get("OUTPUT_DIR", undefinedStringValue),
	}

	isValid, validationErrors := validateConfig(cfg)
	if !isValid {
		return fmt.Errorf("валидация конфигурации сервиса: %s (задайте настройки через .env в корневой директории приложения)", strings.Join(validationErrors, ", "))
	}
	return nil
}

// validateConfig выполняет валидация прочитанного файла конфигурации.
func validateConfig(cfg *Config) (bool, []string) {
	validationErrors := make([]string, 0)
	if cfg.Host == undefinedStringValue {
		validationErrors = append(validationErrors, "HOST не задан")
	}
	if cfg.Port == undefinedStringValue {
		validationErrors = append(validationErrors, "PORT не задан")
	}
	if cfg.OutputDir == undefinedStringValue {
		validationErrors = append(validationErrors, "OUTPUT_DIR не задан")
	}

	if _, err := os.Stat(cfg.OutputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
			panic("Не удалось создать директорию для записи pdf файлов: " + err.Error())
		}
	}

	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}

// GetConfig возвращает конфигурацию приложения.
// Запускает панику, если конфигурация не была проинициализирована до попытки чтения.
func GetConfig() *Config {
	if cfg == nil {
		panic("чтение не инициализированной конфигурации приложения")
	}
	return cfg
}
