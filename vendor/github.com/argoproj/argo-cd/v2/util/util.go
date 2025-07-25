package util

import (
	"crypto/rand"
	"encoding/base64"

	"k8s.io/apimachinery/pkg/runtime"
)

// MakeSignature generates a cryptographically-secure pseudo-random token, based on a given number of random bytes, for signing purposes.
func MakeSignature(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		b = nil
	}
	// base64 encode it so signing key can be typed into validation utilities
	b = []byte(base64.StdEncoding.EncodeToString(b))
	return b, err
}

// SliceCopy generates a deep copy of a slice containing any type that implements the runtime.Object interface.
func SliceCopy[T runtime.Object](items []T) []T {
	itemsCopy := make([]T, len(items))
	for i, item := range items {
		itemsCopy[i] = item.DeepCopyObject().(T)
	}
	return itemsCopy
}
