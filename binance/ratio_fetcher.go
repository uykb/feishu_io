package binance

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"binance-monitor/models"
)

const (
	ratioAPIURL = "https://fapi.binance.com/futures/data/globalLongShortAccountRatio"
)

type RatioFetcher struct {
	symbols     []string
	ratioDataCh chan models.LongShortRatioData
	interval    time.Duration
	client      *http.Client
}

func NewRatioFetcher(symbols []string, ratioDataCh chan models.LongShortRatioData, interval time.Duration) *RatioFetcher {
	return &RatioFetcher{
		symbols:     symbols,
		ratioDataCh: ratioDataCh,
		interval:    interval,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (rf *RatioFetcher) Start() {
	ticker := time.NewTicker(rf.interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		for _, symbol := range rf.symbols {
			go rf.fetchRatio(symbol)
		}
	}
}

type ratioResponse struct {
	Symbol         string `json:"symbol"`
	LongShortRatio string `json:"longShortRatio"`
	Timestamp      int64  `json:"timestamp"`
}

func (rf *RatioFetcher) fetchRatio(symbol string) {
	url := fmt.Sprintf("%s?symbol=%s&period=5m", ratioAPIURL, symbol)
	resp, err := rf.client.Get(url)
	if err != nil {
		log.Printf("获取多空比失败 %s: %v", symbol, err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取多空比响应失败 %s: %v", symbol, err)
		return
	}

	var data []ratioResponse
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("解析多空比JSON失败 %s: %v", symbol, err)
		return
	}

	if len(data) > 0 {
		ratio, err := parseFloat(data[0].LongShortRatio)
		if err != nil {
			log.Printf("转换多空比失败 %s: %v", symbol, err)
			return
		}

		rf.ratioDataCh <- models.LongShortRatioData{
			Symbol:        data[0].Symbol,
			LongShortRatio: ratio,
			Timestamp:     time.Unix(0, data[0].Timestamp*int64(time.Millisecond)),
		}
	}
}
