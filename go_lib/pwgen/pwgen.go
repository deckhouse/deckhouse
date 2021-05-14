package pwgen

import "crypto/rand"

var num = "0123456789"
var alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
var symbols = "[]{}<>()=-_!@#$%^&*.,"
var alphaNum = num + alpha
var alphaNumSymbols = alphaNum + symbols

func generateString(length int, chars string) string {
	var bytes = make([]byte, length)
	var op = byte(len(chars))

	_, _ = rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = chars[b%op]
	}
	return string(bytes)
}

// Num generates a random string of the given length out of numeric characters
func Num(length int) string {
	return generateString(length, num)
}

// Alpha generates a random string of the given length out of alphabetic characters
func Alpha(length int) string {
	return generateString(length, alpha)
}

// Symbols generates a random string of the given length out of symbols
func Symbols(length int) string {
	return generateString(length, symbols)
}

// AlphaNum generates a random string of the given length out of alphanumeric characters
func AlphaNum(length int) string {
	return generateString(length, alphaNum)
}

// AlphaNumSymbols generates a random string of the given length out of alphanumeric characters and
// symbols
func AlphaNumSymbols(length int) string {
	return generateString(length, alphaNumSymbols)
}
