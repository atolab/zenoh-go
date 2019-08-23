package zenoh

// VleEncode returns the value as a VLE encoded in a []byte
func VleEncode(value int) []byte {
	var buf []byte
	for value > 0x7f {
		c := (byte)((value & 0x7f) | 0x80)
		buf = append(buf, c)
		value = value >> 7
	}
	buf = append(buf, (byte)(value))

	return buf
}

// VleDecode returns the int value decoded from a VLE encoded in a []byte
func VleDecode(buf []byte) (int, []byte) {
	var value, c int
	var i, j uint
	for ok := true; ok; ok = (c > 0x7f) {
		c = (int)(buf[j] & 0xff)
		value = value | ((c & 0x7f) << i)
		i += 7
		j++
	}
	return value, buf[j:]
}
