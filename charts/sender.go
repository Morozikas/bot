// Пакет charts — генерация торговых графиков.
// sender.go — публичный интерфейс. Конфигурация читается из .env.chart
package charts

import signalengine "bybit-elite-signal/signal_engine"

var globalCfg ChartConfig

func init() {
	// Загружаем конфигурацию из .env.chart при старте
	globalCfg = LoadConfig()
}

// GenerateForSignal создаёт PNG-график для сигнала.
// Конфигурация читается из .env.chart
func GenerateForSignal(sig *signalengine.Signal, tps []float64, mode, deadline string) ([]byte, error) {
	data := FromSignal(sig, tps, mode, deadline, &globalCfg)
	return Generate(data, globalCfg)
}

// GenerateFromRaw создаёт PNG из произвольных данных.
func GenerateFromRaw(data *SignalChartData) ([]byte, error) {
	return Generate(data, globalCfg)
}

// ReloadConfig перечитывает .env.chart без перезапуска.
// Полезно при ручном изменении настроек на лету.
func ReloadConfig() {
	globalCfg = LoadConfig()
}
