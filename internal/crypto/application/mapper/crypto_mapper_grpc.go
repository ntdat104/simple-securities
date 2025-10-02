package mapper

import (
	crypto "simple-securities/gen/crypto/v1"
	"simple-securities/internal/crypto/application/dto"
)

func ToKline(klineDto *dto.KlineDto) *crypto.Kline {
	if klineDto == nil {
		return nil
	}
	return &crypto.Kline{
		OpenTime:                 klineDto.OpenTime,
		Open:                     klineDto.Open,
		High:                     klineDto.High,
		Low:                      klineDto.Low,
		Close:                    klineDto.Close,
		Volume:                   klineDto.Volume,
		CloseTime:                klineDto.CloseTime,
		QuoteAssetVolume:         klineDto.QuoteAssetVolume,
		NumberOfTrades:           klineDto.NumberOfTrades,
		TakerBuyBaseAssetVolume:  klineDto.TakerBuyBaseAssetVolume,
		TakerBuyQuoteAssetVolume: klineDto.TakerBuyQuoteAssetVolume,
	}
}

func ToKlines(klineDtos []*dto.KlineDto) []*crypto.Kline {
	if klineDtos == nil {
		return nil
	}
	klines := make([]*crypto.Kline, 0, len(klineDtos))
	for _, klineDto := range klineDtos {
		klines = append(klines, ToKline(klineDto))
	}
	return klines
}
