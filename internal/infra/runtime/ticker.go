package runtime

import "time"

type Ticker struct{ ticker *time.Ticker }

func NewTicker(interval time.Duration) *Ticker { return &Ticker{ticker: time.NewTicker(interval)} }
func (t *Ticker) C() <-chan time.Time          { return t.ticker.C }
func (t *Ticker) Stop()                        { t.ticker.Stop() }
