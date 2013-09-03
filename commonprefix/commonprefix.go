package commonprefix

// CommonPrefix returns the common prefix from slice of strings
func CommonPrefix(p []string) string {
	if len(p) == 0 {
		return ""
	}
	prefixlen := len(p[0])
	for _, v := range p {
		if len(v) == 0 {
			return ""
		}
		for i := 0; i < prefixlen; i++ {
			if v[i] != p[0][i] {
				prefixlen = i
				break
			}
		}
		if prefixlen == 0 {
			break
		}
	}
	return p[0][:prefixlen]
}
