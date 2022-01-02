package main_test

import (
	"bytes"
	"testing"
)

func BenchmarkBytesAppendByteByByte(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var bSl []byte
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
	}
}

func BenchmarkBufferWriteByteByByte(b *testing.B) {
	var buf bytes.Buffer
	var scratch [64]byte
	for i := 0; i < b.N; i++ {
		bSl := scratch[:0]
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		bSl = append(bSl, 'n')
		buf.Write(bSl)
	}
}
