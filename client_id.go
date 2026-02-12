package main

import (
	"crypto/rand"
	"math/big"
)

func generateClientID() (string, error) {
	const digits = 3
	const maxDigit = 10

	var result [digits]byte
	for i := 0; i < digits; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(maxDigit))
		if err != nil {
			return "", err
		}
		result[i] = byte('0' + n.Int64())
	}

	return string(result[:]), nil
}

