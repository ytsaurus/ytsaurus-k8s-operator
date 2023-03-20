package ytconfig

import (
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"strings"
)

func generateCellId(cellTag int16) string {
	cellId, err := uuid.NewRandomFromReader(strings.NewReader("ytsaurus-kubernetes-operator"))
	if err != nil {
		panic(err)
	}
	uuidBytes, err := cellId.MarshalBinary()
	if err != nil {
		panic(err)
	}

	uuidBytes[4] = byte(cellTag >> 8)
	uuidBytes[5] = byte(cellTag & 0xff)

	masterCellType := 601
	uuidBytes[6] = byte(masterCellType >> 8)
	uuidBytes[7] = byte(masterCellType & 0xff)

	getGuidPart := func(data []byte) string {
		format := strings.Repeat("%02x", len(data))
		args := make([]any, 0, len(data))
		for _, value := range data {
			args = append(args, value)
		}

		part := fmt.Sprintf(format, args...)
		return strings.TrimLeft(part, "0")
	}
	return fmt.Sprintf("%s-%s-%s-%s", getGuidPart(uuidBytes[12:]), getGuidPart(uuidBytes[8:12]), getGuidPart(uuidBytes[4:8]), getGuidPart(uuidBytes[:4]))
}

func RandString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
