package main

import (
	"crypto/rand"
	"math/big"
)

func generateClientID() (string, error) {
	const digits = 9
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

func formatClientID(id string) string {
	if len(id) != 9 {
		return id
	}
	return id[0:3] + " " + id[3:6] + " " + id[6:9]
}

