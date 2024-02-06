package clib

//#include "./crc32.h"
import "C"

func Crc32(s string) uint32 {
	hashval := uint32(C.crc32_crypto(C.CString(s), C.int(len(s))))
	return hashval
}
