package logger

import (
	"log"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type (
	// берём структуру для хранения сведений об ответе
	responseData struct {
		status int
		size   int
	}

	// добавляем реализацию http.ResponseWriter
	loggingResponseWriter struct {
		http.ResponseWriter // встраиваем оригинальный http.ResponseWriter
		responseData        *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	// записываем ответ, используя оригинальный http.ResponseWriter
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size // захватываем размер
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	// записываем код статуса, используя оригинальный http.ResponseWriter
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode // захватываем код статуса
}

// WithLogging добавляет дополнительный код для регистрации сведений о запросе
// и возвращает новый http.Handler.
func WithLogging(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {

		// создаём предустановленный регистратор zap
		logger, err := zap.NewDevelopment()
		if err != nil {
			// вызываем панику, если ошибка
			log.Fatal(err)
		}
		defer logger.Sync() //nolint

		// делаем регистратор SugaredLogger
		sugar := *logger.Sugar()

		start := time.Now()

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
			responseData:   responseData,
		}
		h.ServeHTTP(&lw, r) // внедряем реализацию http.ResponseWriter

		duration := time.Since(start)

		if r.Method == http.MethodGet {
			sugar.Infoln(
				"uri", r.RequestURI,
				"method", r.Method,
				"duration", duration,
				"status", responseData.status, // получаем перехваченный код статуса ответа
				"size", responseData.size, // получаем перехваченный размер ответа
			)
		} else {
			sugar.Infoln(
				"uri", r.RequestURI,
				"method", r.Method,
				"status", responseData.status,
				"duration", duration,
			)
		}
	}
	// возвращаем функционально расширенный хендлер
	return http.HandlerFunc(logFn)
}

func ServerRunningInfo(RunAddr string) {
	// добавляем предустановленный логер NewDevelopment
	logger, err := zap.NewDevelopment()
	if err != nil {
		// вызываем панику, если ошибка
		log.Fatal(err)
	}
	// это нужно добавить, если логер буферизован
	// в данном случае не буферизован, но привычка хорошая
	defer logger.Sync() //nolint

	// делаем логер SugaredLogger
	sugar := logger.Sugar()

	// выводим сообщение уровня Info, но со строкой URL, это тоже SugaredLogger
	sugar.Infof("Running server on %s", RunAddr)
}

func Warnf(s string) {
	// добавляем предустановленный логер NewDevelopment
	logger, err := zap.NewDevelopment()
	if err != nil {
		// вызываем панику, если ошибка
		log.Fatal(err)
	}
	// это нужно добавить, если логер буферизован
	// в данном случае не буферизован, но привычка хорошая
	defer logger.Sync() //nolint

	// делаем логер SugaredLogger
	sugar := logger.Sugar()

	// выводим сообщение
	sugar.Warnf("%s", s)
}
