package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/caarlos0/env"
)

type ServerFlags struct {
	FlagRunAddr     string
	FlagDatabaseURI string
	FlagASAddr      string
	EnvRunAddr      string `env:"RUN_ADDRESS"`
	EnvDatabaseURI  string `env:"DATABASE_URI"`
	EnvASAddr       string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

// parseFlags обрабатывает аргументы командной строки
// и сохраняет их значения в соответствующих переменных
func ParseFlags() ServerFlags {
	fmt.Fprintln(os.Stdout, "read flags")
	// для случаев, когда в переменных окружения присутствует непустое значение,
	// переопределим их, даже если они были переданы через аргументы командной строки
	cfg := &ServerFlags{}
	if err := env.Parse(cfg); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(os.Stdout, " flags 0")
	fmt.Fprintln(os.Stdout, cfg.EnvRunAddr)
	fmt.Fprintln(os.Stdout, cfg.EnvDatabaseURI)
	fmt.Fprintln(os.Stdout, cfg.EnvASAddr)

	cfg.EnvRunAddr = strings.ReplaceAll(cfg.EnvRunAddr, "\"", "")
	cfg.EnvRunAddr = strings.ReplaceAll(cfg.EnvRunAddr, " ", "")
	cfg.EnvDatabaseURI = strings.ReplaceAll(cfg.EnvDatabaseURI, "\"", "")
	cfg.EnvDatabaseURI = strings.ReplaceAll(cfg.EnvDatabaseURI, " ", "")
	cfg.EnvASAddr = strings.ReplaceAll(cfg.EnvASAddr, "\"", "")
	cfg.EnvASAddr = strings.ReplaceAll(cfg.EnvASAddr, " ", "")

	// Регистрируем переменные:
	// Строка с адресом и портом запуска сервиса должна получаться из переменной окружения RUN_ADDRESS или флага командной строки -a
	flag.StringVar(&cfg.FlagRunAddr, "a", "localhost:8080", "Address and port to run server")
	// Строка с адресом подключения к БД должна получаться из переменной окружения DATABASE_DSN или флага командной строки -d
	flag.StringVar(&cfg.FlagDatabaseURI, "d", "postgres://postgres:pos111@localhost:5432/postgres?sslmode=disable", "Database URI")
	// Строка с адресом подключения к системе расчёта начислений должна получаться из переменной окружения ACCRUAL_SYSTEM_ADDRESS или флага командной строки -r
	flag.StringVar(&cfg.FlagASAddr, "r", "", "Accrual system address")
	// парсим переданные серверу аргументы в зарегистрированные переменные
	flag.Parse()

	fmt.Fprintln(os.Stdout, " flags 1")
	fmt.Fprintln(os.Stdout, cfg.FlagRunAddr)
	fmt.Fprintln(os.Stdout, cfg.FlagDatabaseURI)
	fmt.Fprintln(os.Stdout, cfg.FlagASAddr)

	// для случаев, когда в переменной окружения присутствует непустое значение,
	// используем его, даже если значение было передано через аргумент командной строки
	if cfg.EnvRunAddr != "" {
		cfg.FlagRunAddr = cfg.EnvRunAddr
	}

	if cfg.EnvDatabaseURI != "" {
		cfg.FlagDatabaseURI = cfg.EnvDatabaseURI
	}
	if cfg.EnvASAddr != "" {
		cfg.FlagASAddr = cfg.EnvASAddr
	}

	fmt.Fprintln(os.Stdout, " flags 2")
	fmt.Fprintln(os.Stdout, cfg.FlagRunAddr)
	fmt.Fprintln(os.Stdout, cfg.FlagDatabaseURI)
	fmt.Fprintln(os.Stdout, cfg.FlagASAddr)

	return *cfg
}
