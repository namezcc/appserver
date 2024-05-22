package util

import (
	"crypto/md5"
	"encoding/hex"
	"hash/crc32"
	"strconv"
	"strings"
)

func SplitIntArray(s string, sep string) []int {
	vec := strings.Split(s, sep)
	res := make([]int, 0)
	for _, v := range vec {
		i, _ := strconv.Atoi(v)
		res = append(res, i)
	}
	return res
}

func StringToInt(v string) int {
	i, _ := strconv.Atoi(v)
	return i
}

func StringToInt64(v string) int64 {
	i, _ := strconv.ParseInt(v, 10, 64)
	return i
}

func StringHash(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}

func StringCharLen(s string) int {
	return len([]rune(s))
}

func StringMd5(s string) string {
	mdns := md5.Sum([]byte(s))
	return hex.EncodeToString(mdns[:])
}
