package httpclient

// zeroRetryRandom selects the lower jitter bound so exponential delays are zero.
type zeroRetryRandom struct{}

func (zeroRetryRandom) Int64n(int64) int64 { return 0 }
