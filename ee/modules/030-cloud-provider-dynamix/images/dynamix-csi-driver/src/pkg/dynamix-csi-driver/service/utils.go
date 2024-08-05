package service

func convertBytesToGigabytes(b uint64) uint64 {
	return b / 1024 / 1024 / 1024
}

func convertGigabytesToBytes(g uint64) uint64 {
	return g * 1024 * 1024 * 1024
}
